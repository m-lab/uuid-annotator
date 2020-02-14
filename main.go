package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/go/warnonerror"

	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/ipannotator"
	"github.com/m-lab/uuid-annotator/zipfile"

	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/eventsocket"
	"github.com/m-lab/uuid-annotator/handler"
)

var (
	datadir         = flag.String("datadir", ".", "The directory to put the data in")
	maxmindurl      = flag.String("url", "", "The URL for the zipfile containing MaxMind IP metadata.  Accepted URL schemes currently are: gs://bucket/file and file:./relativepath/file")
	eventbuffersize = flag.Int("eventbuffersize", 1000, "How many events should we buffer before dropping them?")

	// Reloading relatively frequently should be fine as long as (a) download
	// failure is non-fatal for reloads and (b) cache-checking actually works so
	// that we don't actually re-download it if the data is new. The first
	// condition is enforced in the ipannotator package, and the second in
	// zipfile.
	reloadMin  = flag.Duration("reloadmin", time.Hour, "Minimum time to wait between reloads of backing data")
	reloadTime = flag.Duration("reloadtime", 5*time.Hour, "Expected time to wait between reloads of backing data")
	reloadMax  = flag.Duration("reloadmax", 24*time.Hour, "Maximum time to wait between reloads of backing data")

	// Context and cancellation in support of testing.
	mainCtx, mainCancel = context.WithCancel(context.Background())
)

func init() {
	log.SetFlags(log.LstdFlags | log.LUTC | log.Lshortfile)
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
	u, err := url.Parse(*maxmindurl)
	rtx.Must(err, "Could not parse URL")
	p, err := zipfile.FromURL(mainCtx, u)
	rtx.Must(err, "Could not get maxmind data from url")
	ipa := ipannotator.New(mainCtx, p, localIPs)

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
			ipa.Reload(mainCtx)
		}
		wg.Done()
	}()

	// Generate .json files for every UUID discovered.
	h := handler.New(*datadir, *eventbuffersize, []annotator.Annotator{ipa})
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

	wg.Wait()
}
