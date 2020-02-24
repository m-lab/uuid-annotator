package asnannotator

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/m-lab/go/rtx"

	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/annotation-service/asn"
	"github.com/m-lab/annotation-service/iputils"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

// ReloadingAnnotator is just a regular annotator with a Reload method.
type ReloadingAnnotator interface {
	annotator.Annotator
	Reload(context.Context)
}

// asnAnnotator is the central struct for this module.
type asnAnnotator struct {
	m        sync.RWMutex
	localIPs []net.IP
	as4      rawfile.Provider
	as6      rawfile.Provider
	asn4     api.Annotator
	asn6     api.Annotator
}

// New makes a new Annotator that uses IP addresses to lookup ASN metadata for
// that IP based on the current copy of RouteViews data stored in the given providers.
func New(ctx context.Context, as4 rawfile.Provider, as6 rawfile.Provider, localIPs []net.IP) ReloadingAnnotator {
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

	_, err := annotator.FindDirection(ID, a.localIPs)
	if err != nil {
		return err
	}

	// TODO: remove after enabling asn annotations again.
	var src, dst *api.Annotations

	var errs []error
	errs = append(errs, a.annotate(ID.SrcIP, src))
	errs = append(errs, a.annotate(ID.DstIP, dst))

	// Return the first error (if any).
	for _, e := range errs {
		if e != nil {
			return fmt.Errorf("Could not annotate ip (%s): %w", e.Error(), annotator.ErrNoAnnotation)
		}
	}
	return nil
}

func (a *asnAnnotator) annotate(src string, ann *api.Annotations) error {
	err := a.asn4.Annotate(src, ann)
	// Check if IPv4 succeeded.
	if err == nil && ann.Network != nil && len(ann.Network.Systems) > 0 {
		// The annotation succeeded with IPv4.
		return nil
	}
	// Otherwise reset to try again.
	ann.Network = nil

	err = a.asn6.Annotate(src, ann)
	if err != nil && err != iputils.ErrNodeNotFound {
		// Ignore not found errors.
		return err
	}
	return nil
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

func (a *asnAnnotator) load4(ctx context.Context) (api.Annotator, error) {
	gz, err := a.as4.Get(ctx)
	if err == rawfile.ErrNoChange {
		return a.asn4, nil
	}
	if err != nil {
		return nil, err
	}
	return a.load(ctx, gz)
}

func (a *asnAnnotator) load6(ctx context.Context) (api.Annotator, error) {
	gz, err := a.as6.Get(ctx)
	if err == rawfile.ErrNoChange {
		return a.asn6, nil
	}
	if err != nil {
		return nil, err
	}
	return a.load(ctx, gz)
}

func (a *asnAnnotator) load(ctx context.Context, gz []byte) (api.Annotator, error) {
	data, err := rawfile.FromGZ(gz)
	if err != nil {
		return nil, err
	}
	return asn.LoadASNDatasetFromReader(bytes.NewBuffer(data))
}
