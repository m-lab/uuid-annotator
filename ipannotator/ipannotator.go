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
)

// ReloadingAnnotator is just a regular annotator with a Reload method.
type ReloadingAnnotator interface {
	annotator.Annotator
	Reload()
}

type knownAnnotators struct {
	geo, v4asn, v6asn api.Annotator
}

// ipannotator is the central struct for this module.
type ipannotator struct {
	mut          sync.RWMutex
	bucket, file string
	localAddrs   []net.Addr
	annotators   knownAnnotators
}

// direction gives us an enum to keep track of which end of the connection is
// the server, because we are informed of connections without regard to which
// end is the local server.
type direction int

const (
	s2c direction = iota
	c2s
	unknown
)

// Annotate puts into geolocation data and ASN data into the passed-in annotations map.
func (ipa *ipannotator) Annotate(ID *inetdiag.SockID, annotations *annotator.Annotations) error {
	ipa.mut.RLock()
	defer ipa.mut.RUnlock()

	dir := unknown
	for _, local := range ipa.localAddrs {
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
	errs = append(errs, ipa.annotators.geo.Annotate(ID.SrcIP, src))
	errs = append(errs, ipa.annotators.geo.Annotate(ID.DstIP, dst))

	// Return the first error (if any).
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// Reload is intended to be regularly called in a loop. It should check whether
// the data in GCS is newer than the local data, and, if it is, then download
// and load that new data into memory and then replace it in the annotator.
func (ipa *ipannotator) Reload() {
	// TODO: check cached status of the current dataset instead of unconditionally reloading.
	newAnnotators, err := ipa.load()
	if err != nil {
		log.Println("Could not reload dataset:", err)
		return
	}
	// Don't acquire the lock until after we have done all network IO
	ipa.mut.Lock()
	defer ipa.mut.Unlock()
	ipa.annotators = newAnnotators
}

// load unconditionally loads the annotator datasets from GCS.
func (ipa *ipannotator) load() (knownAnnotators, error) {
	ka := knownAnnotators{}
	var err error
	ka.geo, err = geolite2v2.LoadG2Dataset(ipa.file, ipa.bucket)
	// TODO: also load ASN datasets
	return ka, err
}

// New makes a new Annotator that uses IP addresses to generate geolocation and
// ASNumber metadata for that IP based on the current copy of MaxMind data
// stored in GCS.
func New(bucket, file string, localAddrs []net.Addr) ReloadingAnnotator {
	ipa := &ipannotator{
		bucket:     bucket,
		file:       file,
		localAddrs: localAddrs,
	}
	var err error
	ipa.annotators, err = ipa.load()
	rtx.Must(err, "Could not load annotation db")
	return ipa
}
