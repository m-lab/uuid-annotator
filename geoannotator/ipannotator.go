package geoannotator

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/m-lab/go/rtx"
	"github.com/oschwald/geoip2-golang"

	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

// ReloadingAnnotator is just a regular annotator with a Reload method.
type ReloadingAnnotator interface {
	annotator.Annotator
	Reload(context.Context)
}

// geoannotator is the central struct for this module.
type geoannotator struct {
	mut               sync.RWMutex
	localIPs          []net.IP
	backingDataSource rawfile.Provider
	maxmind           *geoip2.Reader
}

// Annotate puts into geolocation data and ASN data into the passed-in annotations map.
func (g *geoannotator) Annotate(ID *inetdiag.SockID, annotations *annotator.Annotations) error {
	g.mut.RLock()
	defer g.mut.RUnlock()

	src, dst, err := annotator.Direction(ID, g.localIPs, annotations)
	if err != nil {
		return err
	}

	var errs []error
	errs = append(errs, g.annotate(ID.SrcIP, src))
	errs = append(errs, g.annotate(ID.DstIP, dst))

	// Return the first error (if any).
	for _, e := range errs {
		if e != nil {
			return fmt.Errorf("Could not annotate ip (%s): %w", e.Error(), annotator.ErrNoAnnotation)
		}
	}
	return nil
}

var emptyResult = geoip2.City{}

func (g *geoannotator) annotate(src string, ann *api.Annotations) error {
	ip := net.ParseIP(src)
	if ip == nil {
		return fmt.Errorf("failed to parse IP %q", src)
	}
	record, err := g.maxmind.City(ip)
	if err != nil {
		return err
	}

	// Check for empty results because "not found" is not an error. Instead the
	// geoip2 package returns an empty result. May be fixed in a future version:
	// https://github.com/oschwald/geoip2-golang/issues/32
	if isEmpty(record) {
		return fmt.Errorf("not found %q", src)
	}

	ann.Geo = &api.GeolocationIP{
		ContinentCode:    record.Continent.Code,
		CountryCode:      record.Country.IsoCode,
		CountryName:      record.Country.Names["en"],
		MetroCode:        int64(record.Location.MetroCode),
		City:             record.City.Names["en"],
		PostalCode:       record.Postal.Code,
		Latitude:         record.Location.Latitude,
		Longitude:        record.Location.Longitude,
		AccuracyRadiusKm: int64(record.Location.AccuracyRadius),
	}

	// Collect subdivision information, if found.
	if len(record.Subdivisions) > 0 {
		ann.Geo.Subdivision1ISOCode = record.Subdivisions[0].IsoCode
		ann.Geo.Subdivision1Name = record.Subdivisions[0].Names["en"]
		if len(record.Subdivisions) > 1 {
			ann.Geo.Subdivision2ISOCode = record.Subdivisions[1].IsoCode
			ann.Geo.Subdivision2Name = record.Subdivisions[1].Names["en"]
		}
	}
	return nil
}

func isEmpty(r *geoip2.City) bool {
	return r.City.GeoNameID == 0 && r.Country.GeoNameID == 0 && r.Continent.GeoNameID == 0
}

// Reload is intended to be regularly called in a loop. It should check whether
// the data in GCS is newer than the local data, and, if it is, then download
// and load that new data into memory and then replace it in the annotator.
func (g *geoannotator) Reload(ctx context.Context) {
	newMM, err := g.load(ctx)
	if err != nil {
		log.Println("Could not reload dataset:", err)
		return
	}
	// Don't acquire the lock until after the data is in RAM.
	g.mut.Lock()
	defer g.mut.Unlock()
	g.maxmind = newMM
}

// load unconditionally loads datasets and returns them.
func (g *geoannotator) load(ctx context.Context) (*geoip2.Reader, error) {
	tgz, err := g.backingDataSource.Get(ctx)
	if err == rawfile.ErrNoChange {
		return g.maxmind, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := rawfile.FromTarGZ(tgz, "GeoLite2-City.mmdb")
	if err != nil {
		return nil, err
	}
	return geoip2.FromBytes(data)
}

// New makes a new Annotator that uses IP addresses to generate geolocation and
// ASNumber metadata for that IP based on the current copy of MaxMind data
// stored in GCS.
func New(ctx context.Context, geo rawfile.Provider, localIPs []net.IP) ReloadingAnnotator {
	g := &geoannotator{
		backingDataSource: geo,
		localIPs:          localIPs,
	}
	var err error
	g.maxmind, err = g.load(ctx)
	rtx.Must(err, "Could not load annotation db")
	return g
}
