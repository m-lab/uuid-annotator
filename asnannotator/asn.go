package asnannotator

import (
	"context"
	"log"
	"net"
	"sync"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/rtx"

	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/ipinfo"
	"github.com/m-lab/uuid-annotator/routeview"
	"github.com/m-lab/uuid-annotator/tarreader"
)

// ASNAnnotator is just a regular annotator with a Reload method and an AnnotateIP method.
type ASNAnnotator interface {
	annotator.Annotator
	Reload(context.Context)
	AnnotateIP(src string) *annotator.Network
}

// asnAnnotator is the central struct for this module.
type asnAnnotator struct {
	m          sync.RWMutex
	localIPs   []net.IP
	as4        content.Provider
	as6        content.Provider
	asnamedata content.Provider
	asn4       routeview.Index
	asn6       routeview.Index
	asnames    ipinfo.ASNames
}

// New makes a new Annotator that uses IP addresses to lookup ASN metadata for
// that IP based on the current copy of RouteViews data stored in the given providers.
func New(ctx context.Context, as4 content.Provider, as6 content.Provider, asnamedata content.Provider, localIPs []net.IP) ASNAnnotator {
	a := &asnAnnotator{
		as4:        as4,
		as6:        as6,
		asnamedata: asnamedata,
		localIPs:   localIPs,
	}
	var err error
	a.asn4, err = load(ctx, as4, nil)
	rtx.Must(err, "Could not load Routeviews IPv4 ASN db")
	a.asn6, err = load(ctx, as6, nil)
	rtx.Must(err, "Could not load Routeviews IPv6 ASN db")
	a.asnames, err = loadNames(ctx, asnamedata, nil)
	rtx.Must(err, "Could not load IPinfo.io AS name db")
	return a
}

// Annotate puts ASN data into the given annotations.
func (a *asnAnnotator) Annotate(ID *inetdiag.SockID, annotations *annotator.Annotations) error {
	a.m.RLock()
	defer a.m.RUnlock()

	dir, err := annotator.FindDirection(ID, a.localIPs)
	if err != nil {
		return err
	}

	// TODO: annotate the server IP with siteinfo data.
	switch dir {
	case annotator.DstIsServer:
		annotations.Client.Network = a.annotateIPHoldingLock(ID.SrcIP)
	case annotator.SrcIsServer:
		annotations.Client.Network = a.annotateIPHoldingLock(ID.DstIP)
	}
	return nil
}

func (a *asnAnnotator) AnnotateIP(src string) *annotator.Network {
	a.m.RLock()
	defer a.m.RUnlock()
	return a.annotateIPHoldingLock(src)
}

func (a *asnAnnotator) annotateIPHoldingLock(src string) *annotator.Network {
	ann := &annotator.Network{}
	// Check IPv4 first.
	ipnet, err := a.asn4.Search(src)
	// NOTE: ignore errors on the first attempt.
	if err == nil {
		ann.Systems = routeview.ParseSystems(ipnet.Systems)
		ann.ASNumber = ann.FirstASN()
		ann.CIDR = ipnet.String()
		if a.asnames != nil {
			ann.ASName = a.asnames[ann.ASNumber]
		}
		// The annotation succeeded with IPv4.
		return ann
	}

	ipnet, err = a.asn6.Search(src)
	if err != nil {
		// In this case, the search has failed twice.
		ann.Missing = true
		return ann
	}

	ann.Systems = routeview.ParseSystems(ipnet.Systems)
	ann.ASNumber = ann.FirstASN()
	if a.asnames != nil {
		ann.ASName = a.asnames[ann.ASNumber]
	}
	ann.CIDR = ipnet.String()
	// The annotation succeeded with IPv6.
	return ann
}

// Reload is intended to be regularly called in a loop. It should check whether
// the data in GCS is newer than the local data, and, if it is, then download
// and load that new data into memory and then replace it in the annotator.
func (a *asnAnnotator) Reload(ctx context.Context) {
	new4, err := load(ctx, a.as4, a.asn4)
	if err != nil {
		log.Println("Could not reload v4 routeviews:", err)
		return
	}
	new6, err := load(ctx, a.as6, a.asn6)
	if err != nil {
		log.Println("Could not reload v6 routeviews:", err)
		return
	}
	newnames, err := loadNames(ctx, a.asnamedata, a.asnames)
	if err != nil {
		log.Println("Could not reload asnames from ipinfo:", err)
		return
	}
	// Don't acquire the lock until after the data is in RAM.
	a.m.Lock()
	defer a.m.Unlock()
	a.asn4 = new4
	a.asn6 = new6
	a.asnames = newnames
}

func load(ctx context.Context, src content.Provider, oldvalue routeview.Index) (routeview.Index, error) {
	gz, err := src.Get(ctx)
	if err == content.ErrNoChange {
		return oldvalue, nil
	}
	if err != nil {
		return nil, err
	}
	return loadGZ(gz)
}

func loadGZ(gz []byte) (routeview.Index, error) {
	data, err := tarreader.FromGZ(gz)
	if err != nil {
		return nil, err
	}
	return routeview.ParseRouteView(data), nil
}

func loadNames(ctx context.Context, src content.Provider, oldvalue ipinfo.ASNames) (ipinfo.ASNames, error) {
	data, err := src.Get(ctx)
	if err == content.ErrNoChange {
		return oldvalue, nil
	}
	if err != nil {
		return nil, err
	}
	return ipinfo.Parse(data)
}

// fakeASNAnnotator is just a real asnAnnotator that has a fixed dataset and
// can't be reloaded.
type fakeASNAnnotator struct {
	asnAnnotator
}

func (*fakeASNAnnotator) Reload(ctx context.Context) {}

// NewFake returns an annotator that know about just one v4 IP (1.2.3.4) and one
// v6 IP (1111:2222:3333:4444:5555:6666:7777:8888). This is useful for testing
// other components when you don't want to carry around canonical datafiles, or
// for building up a local IP annotation service with known outputs for testing.
//
// TODO(http://github.com/m-lab/uuid-annotator/issues/38): Consider moving this
// fake to its own subpackage.
func NewFake() ASNAnnotator {
	f := &fakeASNAnnotator{}

	// Set up v4 data for 1.2.3.4.
	asn4Entry := routeview.IPNet{}
	_, v4net, err := net.ParseCIDR("1.2.3.4/32")
	rtx.Must(err, "Could not parse fixed string")
	asn4Entry.IPNet = *v4net
	asn4Entry.Systems = "5"
	f.asn4 = routeview.Index{asn4Entry}

	// Set up v6 data for 1111:2222:3333:4444:5555:6666:7777:8888.
	asn6Entry := routeview.IPNet{}
	_, v6net, err := net.ParseCIDR("1111:2222:3333:4444:5555:6666:7777:8888/128")
	rtx.Must(err, "Could not parse fixed string")
	asn6Entry.IPNet = *v6net
	asn6Entry.Systems = "9"
	f.asn6 = routeview.Index{asn6Entry}

	// Set up AS name entries for AS5 and AS9
	f.asnames = ipinfo.ASNames{
		5: "Test Number Five",
		9: "Test Number Nine",
	}
	return f
}
