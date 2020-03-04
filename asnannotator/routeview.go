package asnannotator

import (
	"context"
	"log"
	"net"
	"sync"

	"github.com/m-lab/go/rtx"

	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
	"github.com/m-lab/uuid-annotator/routeview"
)

// ASNAnnotator is just a regular annotator with a Reload method and an AnnotateIP method.
type ASNAnnotator interface {
	annotator.Annotator
	Reload(context.Context)
	AnnotateIP(src string) *annotator.Network
}

// asnAnnotator is the central struct for this module.
type asnAnnotator struct {
	m        sync.RWMutex
	localIPs []net.IP
	as4      rawfile.Provider
	as6      rawfile.Provider
	asn4     routeview.Index
	asn6     routeview.Index
}

// New makes a new Annotator that uses IP addresses to lookup ASN metadata for
// that IP based on the current copy of RouteViews data stored in the given providers.
func New(ctx context.Context, as4 rawfile.Provider, as6 rawfile.Provider, localIPs []net.IP) ASNAnnotator {
	a := &asnAnnotator{
		as4:      as4,
		as6:      as6,
		localIPs: localIPs,
	}
	var err error
	a.asn4, err = a.load4(ctx)
	rtx.Must(err, "Could not load IPv4 ASN db")
	a.asn6, err = a.load6(ctx)
	rtx.Must(err, "Could not load IPv6 ASN db")
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
	ann.CIDR = ipnet.String()
	// The annotation succeeded with IPv6.
	return ann
}

// Reload is intended to be regularly called in a loop. It should check whether
// the data in GCS is newer than the local data, and, if it is, then download
// and load that new data into memory and then replace it in the annotator.
func (a *asnAnnotator) Reload(ctx context.Context) {
	new4, err := a.load4(ctx)
	if err != nil {
		log.Println("Could not reload v4 routeviews:", err)
		return
	}
	new6, err := a.load6(ctx)
	if err != nil {
		log.Println("Could not reload v6 routeviews:", err)
		return
	}
	// Don't acquire the lock until after the data is in RAM.
	a.m.Lock()
	defer a.m.Unlock()
	a.asn4 = new4
	a.asn6 = new6
}

func (a *asnAnnotator) load4(ctx context.Context) (routeview.Index, error) {
	gz, err := a.as4.Get(ctx)
	if err == rawfile.ErrNoChange {
		return a.asn4, nil
	}
	if err != nil {
		return nil, err
	}
	return a.load(ctx, gz)
}

func (a *asnAnnotator) load6(ctx context.Context) (routeview.Index, error) {
	gz, err := a.as6.Get(ctx)
	if err == rawfile.ErrNoChange {
		return a.asn6, nil
	}
	if err != nil {
		return nil, err
	}
	return a.load(ctx, gz)
}

func (a *asnAnnotator) load(ctx context.Context, gz []byte) (routeview.Index, error) {
	data, err := rawfile.FromGZ(gz)
	if err != nil {
		return nil, err
	}
	return routeview.ParseRouteView(data), nil
}
