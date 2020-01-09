package ipannotator

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/m-lab/go/rtx"

	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/annotation-service/geolite2v2"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/zipfile"
)

// ReloadingAnnotator is just a regular annotator with a Reload method.
type ReloadingAnnotator interface {
	annotator.Annotator
	Reload()
}

// ipannotator is the central struct for this module.
type ipannotator struct {
	mut               sync.RWMutex
	localIPs          []net.IP
	backingDataSource zipfile.Provider
	maxmind           *geolite2v2.GeoDataset
}

// direction gives us an enum to keep track of which end of the connection is
// the server, because we are informed of connections without regard to which
// end is the local server.
type direction int

const (
	unknown direction = iota
	s2c
	c2s
)

// Annotate puts into geolocation data and ASN data into the passed-in annotations map.
func (ipa *ipannotator) Annotate(ID *inetdiag.SockID, annotations *annotator.Annotations) error {
	ipa.mut.RLock()
	defer ipa.mut.RUnlock()

	dir := unknown
	for _, local := range ipa.localIPs {
		if ID.SrcIP == local.String() {
			dir = s2c
		}
		if ID.DstIP == local.String() {
			dir = c2s
		}
	}

	var src, dst *api.Annotations
	switch dir {
	case s2c:
		src = &annotations.Server
		dst = &annotations.Client
	case c2s:
		src = &annotations.Client
		dst = &annotations.Server
	case unknown:
		return fmt.Errorf("Can't annotate connection: Unknown direction for %+v", ID)
	}

	var errs []error
	errs = append(errs, ipa.maxmind.Annotate(ID.SrcIP, src))
	errs = append(errs, ipa.maxmind.Annotate(ID.DstIP, dst))

	// Return the first error (if any).
	for _, e := range errs {
		if e != nil {
			return fmt.Errorf("Could not annotate ip (%s): %w", e.Error(), annotator.ErrNoAnnotation)
		}
	}
	return nil
}

// Reload is intended to be regularly called in a loop. It should check whether
// the data in GCS is newer than the local data, and, if it is, then download
// and load that new data into memory and then replace it in the annotator.
func (ipa *ipannotator) Reload() {
	newMM, err := ipa.load()
	if err != nil {
		log.Println("Could not reload dataset:", err)
		return
	}
	// Don't acquire the lock until after the data is in RAM.
	ipa.mut.Lock()
	defer ipa.mut.Unlock()
	ipa.maxmind = newMM
}

// load unconditionally loads datasets and returns them.
func (ipa *ipannotator) load() (*geolite2v2.GeoDataset, error) {
	z, err := ipa.backingDataSource.Get()
	if err != nil {
		return nil, err
	}
	return geolite2v2.DatasetFromZip(z)
}

// New makes a new Annotator that uses IP addresses to generate geolocation and
// ASNumber metadata for that IP based on the current copy of MaxMind data
// stored in GCS.
func New(geo zipfile.Provider, localIPs []net.IP) ReloadingAnnotator {
	ipa := &ipannotator{
		backingDataSource: geo,
		localIPs:          localIPs,
	}
	var err error
	ipa.maxmind, err = ipa.load()
	rtx.Must(err, "Could not load annotation db")
	return ipa
}
