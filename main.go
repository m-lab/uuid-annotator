package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/host"
	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/go/warnonerror"
	"github.com/m-lab/tcp-info/eventsocket"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/asnannotator"
	"github.com/m-lab/uuid-annotator/geoannotator"
	"github.com/m-lab/uuid-annotator/handler"
	"github.com/m-lab/uuid-annotator/ipservice"
	"github.com/m-lab/uuid-annotator/siteannotator"
)

var (
	datadir         = flag.String("datadir", ".", "The directory to put the data in")
	hostname        = flag.String("hostname", "", "The server hostname, used to lookup server siteinfo annotations")
	hostnameFile    = flagx.FileBytes{}
	maxmindurl      = flagx.URL{}
	routeviewv4     = flagx.URL{}
	routeviewv6     = flagx.URL{}
	asnameurl       = flagx.URL{}
	siteinfo        = flagx.URL{}
	eventbuffersize = flag.Int("eventbuffersize", 1000, "How many events should we buffer before dropping them?")

	// Reloading relatively frequently should be fine as long as (a) download
	// failure is non-fatal for reloads and (b) cache-checking actually works so
	// that we don't re-download the data until it is new. The first condition is
	// enforced in the geoannotator package, and the second in content.
	reloadMin  = flag.Duration("reloadmin", time.Hour, "Minimum time to wait between reloads of backing data")
	reloadTime = flag.Duration("reloadtime", 5*time.Hour, "Expected time to wait between reloads of backing data")
	reloadMax  = flag.Duration("reloadmax", 24*time.Hour, "Maximum time to wait between reloads of backing data")

	// Context, cancellation, and a channel all in support of testing.
	mainCtx, mainCancel = context.WithCancel(context.Background())
	mainRunning         = make(chan struct{}, 1)
)

func init() {
	flag.Var(&hostnameFile, "hostname-file", "The file containing the server hostname.")
	flag.Var(&maxmindurl, "maxmind.url", "The URL for the file containing MaxMind IP metadata.  Accepted URL schemes currently are: gs://bucket/file and file:./relativepath/file")
	flag.Var(&routeviewv4, "routeview-v4.url", "The URL for the RouteViewIPv4 file containing ASN metadata. gs:// and file:// schemes accepted.")
	flag.Var(&routeviewv6, "routeview-v6.url", "The URL for the RouteViewIPv6 file containing ASN metadata. gs:// and file:// schemes accepted.")
	flag.Var(&asnameurl, "asname.url", "The URL for the ASName CSV file containing a mapping of AS numbers to AS names provided by IPInfo.io")
	flag.Var(&siteinfo, "siteinfo.url", "The URL for the Siteinfo JSON file containing server location and ASN metadata. gs:// and file:// schemes accepted.")
	log.SetFlags(log.LstdFlags | log.LUTC | log.Llongfile)
}

func findLocalIPs(localAddrs []net.Addr) []net.IP {
	localIPs := []net.IP{}
	for _, addr := range localAddrs {
		// By default, addr.String() includes the netblock suffix. By casting to
		// the underlying net.IPNet we can extract just the IP.
		if a, ok := addr.(*net.IPNet); ok {
			localIPs = append(localIPs, a.IP)
		}
	}
	return localIPs
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Could not get args from environment variables")

	// If the -hostname flag was not set, try reading the hostname from the
	// -hostname-file.
	if hostnameFile.String() != "" && *hostname == "" {
		*hostname = hostnameFile.String()
	}

	// Create the datatype directory immediately, since pusher will crash
	// without it.
	rtx.Must(os.MkdirAll(*datadir, 0755), "Could not create datatype dir %s", datadir)

	// Parse the node's name into its constituent parts. This ensures that the
	// value of the -hostname flag is actually valid. Additionally, virtual
	// nodes which are part of a managed instance group may have a random
	// suffix, which uuid-annotator cannot use, so we explicitly only include
	// the parts of the node name that uuid-annotator actually cares about. The
	// resultant variable mlabHostname should match a machine name in siteinfo's
	// annotations.json:
	//
	// https://siteinfo.mlab-oti.measurementlab.net/v2/sites/annotations.json
	h, err := host.Parse(*hostname)
	rtx.Must(err, "Failed to parse the provided hostname")
	mlabHostname := h.String()

	defer mainCancel()
	// A waitgroup that waits for every component goroutine to complete before main exits.
	wg := sync.WaitGroup{}

	// Serve prometheus metrics.
	srv := prometheusx.MustServeMetrics()
	wg.Add(1)
	go func() {
		<-mainCtx.Done()
		warnonerror.Close(srv, "Could not close the metrics server cleanly")
		wg.Done()
	}()

	// Set up IP annotation, first by loading the initial config.
	localAddrs, err := net.InterfaceAddrs()
	rtx.Must(err, "Could not read local addresses")
	localIPs := findLocalIPs(localAddrs)

	// Load the siteinfo annotations for "site" specific metadata. Additionally,
	// if this is a virtual site, New() will append the public IP of the
	// managed instance group's load balancer to localIPs. If uuid-annotator
	// does not know about the public IP of the load balancer, then it will fail
	// to annotate anything because it doesn't recognize its own public address
	// in either the Src or Dest of incoming tcp-info events.
	js, err := content.FromURL(mainCtx, siteinfo.URL)
	rtx.Must(err, "Could not load siteinfo URL")
	site, localIPs := siteannotator.New(mainCtx, mlabHostname, js, localIPs)

	p, err := content.FromURL(mainCtx, maxmindurl.URL)
	rtx.Must(err, "Could not get maxmind data from url")
	geo := geoannotator.New(mainCtx, p, localIPs)

	p4, err := content.FromURL(mainCtx, routeviewv4.URL)
	rtx.Must(err, "Could not load routeview v4 URL")
	p6, err := content.FromURL(mainCtx, routeviewv6.URL)
	rtx.Must(err, "Could not load routeview v6 URL")
	asnames, err := content.FromURL(mainCtx, asnameurl.URL)
	rtx.Must(err, "Could not load AS names URL")
	asn := asnannotator.New(mainCtx, p4, p6, asnames, localIPs)

	// Reload the IP annotation config on a randomized schedule.
	wg.Add(1)
	go func() {
		reloadConfig := memoryless.Config{
			Min:      *reloadMin,
			Max:      *reloadMax,
			Expected: *reloadTime,
		}
		tick, err := memoryless.NewTicker(mainCtx, reloadConfig)
		rtx.Must(err, "Could not create ticker for reloading")
		for range tick.C {
			geo.Reload(mainCtx)
			asn.Reload(mainCtx)
		}
		wg.Done()
	}()

	if *eventsocket.Filename != "" {

		// Generate .json files for every UUID discovered.
		h := handler.New(*datadir, *eventbuffersize, []annotator.Annotator{geo, asn, site})
		wg.Add(1)
		go func() {
			h.ProcessIncomingRequests(mainCtx)
			wg.Done()
		}()

		// Listen to the event socket to find out about new UUIDs and then process them.
		wg.Add(1)
		go func() {
			eventsocket.MustRun(mainCtx, *eventsocket.Filename, h)
			wg.Done()
		}()
	}

	// Set up the local service to serve IP annotations as a local service on a
	// local unix-domain socket.
	if *ipservice.SocketFilename != "" {
		ipsrv, err := ipservice.NewServer(*ipservice.SocketFilename, asn, geo)
		rtx.Must(err, "Could not start up the local IP annotation service")
		wg.Add(2)
		go func() {
			rtx.Must(ipsrv.Serve(), "Could not serve the local IP annotation service")
			wg.Done()
		}()
		go func() {
			<-mainCtx.Done()
			ipsrv.Close()
			wg.Done()
		}()
	}

	mainRunning <- struct{}{}
	wg.Wait()
}
