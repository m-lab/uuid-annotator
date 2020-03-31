package main

import (
	"context"
	"flag"
	"log"
	"net"
	"sync"
	"time"

	"github.com/m-lab/go/flagx"
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
	"github.com/m-lab/uuid-annotator/rawfile"
	"github.com/m-lab/uuid-annotator/siteannotator"
)

var (
	datadir         = flag.String("datadir", ".", "The directory to put the data in")
	hostname        = flag.String("hostname", "", "The server hostname, used to lookup server siteinfo annotations")
	maxmindurl      = flagx.URL{}
	routeviewv4     = flagx.URL{}
	routeviewv6     = flagx.URL{}
	asnameurl       = flagx.URL{}
	siteinfo        = flagx.URL{}
	eventbuffersize = flag.Int("eventbuffersize", 1000, "How many events should we buffer before dropping them?")

	// Reloading relatively frequently should be fine as long as (a) download
	// failure is non-fatal for reloads and (b) cache-checking actually works so
	// that we don't re-download the data until it is new. The first condition is
	// enforced in the geoannotator package, and the second in rawfile.
	reloadMin  = flag.Duration("reloadmin", time.Hour, "Minimum time to wait between reloads of backing data")
	reloadTime = flag.Duration("reloadtime", 5*time.Hour, "Expected time to wait between reloads of backing data")
	reloadMax  = flag.Duration("reloadmax", 24*time.Hour, "Maximum time to wait between reloads of backing data")

	// Context, cancellation, and a channel all in support of testing.
	mainCtx, mainCancel = context.WithCancel(context.Background())
	mainRunning         = make(chan struct{}, 1)
)

func init() {
	flag.Var(&maxmindurl, "maxmind.url", "The URL for the file containing MaxMind IP metadata.  Accepted URL schemes currently are: gs://bucket/file and file:./relativepath/file")
	flag.Var(&routeviewv4, "routeview-v4.url", "The URL for the RouteViewIPv4 file containing ASN metadata. gs:// and file:// schemes accepted.")
	flag.Var(&routeviewv6, "routeview-v6.url", "The URL for the RouteViewIPv6 file containing ASN metadata. gs:// and file:// schemes accepted.")
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
	p, err := rawfile.FromURL(mainCtx, maxmindurl.URL)
	rtx.Must(err, "Could not get maxmind data from url")
	geo := geoannotator.New(mainCtx, p, localIPs)

	p4, err := rawfile.FromURL(mainCtx, routeviewv4.URL)
	rtx.Must(err, "Could not load routeview v4 URL")
	p6, err := rawfile.FromURL(mainCtx, routeviewv6.URL)
	rtx.Must(err, "Could not load routeview v6 URL")
	asnames, err := rawfile.FromURL(mainCtx, asnameurl.URL)
	rtx.Must(err, "Could not load AS names URL")
	asn := asnannotator.New(mainCtx, p4, p6, asnames, localIPs)

	js, err := rawfile.FromURL(mainCtx, siteinfo.URL)
	rtx.Must(err, "Could not load siteinfo URL")
	site := siteannotator.New(mainCtx, *hostname, js, localIPs)

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
