package ipannotator

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

// ipannotator is the central struct for this module.
type ipannotator struct {
	mut               sync.RWMutex
	localIPs          []net.IP
	backingDataSource rawfile.Provider
	maxmind           *geoip2.Reader
}

func (ipa *ipannotator) Annotate(ID *inetdiag.SockID, annotations *annotator.Annotations) error {
	ipa.mut.RLock()
	defer ipa.mut.RUnlock()

	src, dst, err := annotator.Direction(ID, ipa.localIPs, annotations)
	if err != nil {
		return err
	}

	var errs []error
	errs = append(errs, ipa.annotate(ID.SrcIP, src))
	errs = append(errs, ipa.annotate(ID.DstIP, dst))

	// Return the first error (if any).
	for _, e := range errs {
		if e != nil {
			return fmt.Errorf("Could not annotate ip (%s): %w", e.Error(), annotator.ErrNoAnnotation)
		}
	}
	return nil
}

var emptyResult = geoip2.City{}

func (ipa *ipannotator) annotate(src string, ann *api.Annotations) error {
	ip := net.ParseIP(src)
	if ip == nil {
		return fmt.Errorf("failed to parse IP %q", src)
	}
	record, err := ipa.maxmind.City(ip)
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
		ContinentCode: record.Continent.Code,
		CountryCode:   record.Country.IsoCode,
		// CountryCode3: not present in record struct.
		CountryName: record.Country.Names["en"],
		MetroCode:   int64(record.Location.MetroCode),
		City:        record.City.Names["en"],
		// AreaCode: not present in record struct.
		PostalCode:       record.Postal.Code,
		Latitude:         record.Location.Latitude,
		Longitude:        record.Location.Longitude,
		AccuracyRadiusKm: int64(record.Location.AccuracyRadius),
	}
	// TODO: collect all subdivision data.
	if len(record.Subdivisions) > 0 {
		ann.Geo.Region = record.Subdivisions[0].IsoCode
	}
	return nil
}

func isEmpty(r *geoip2.City) bool {
	return r.City.GeoNameID == 0 && r.Country.GeoNameID == 0 && r.Continent.GeoNameID == 0
}

// Reload is intended to be regularly called in a loop. It should check whether
// the data in GCS is newer than the local data, and, if it is, then download
// and load that new data into memory and then replace it in the annotator.
func (ipa *ipannotator) Reload(ctx context.Context) {
	newMM, err := ipa.load(ctx)
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
func (ipa *ipannotator) load(ctx context.Context) (*geoip2.Reader, error) {
	tgz, err := ipa.backingDataSource.Get(ctx)
	if err == rawfile.ErrNoChange {
		return ipa.maxmind, nil
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
	ipa := &ipannotator{
		backingDataSource: geo,
		localIPs:          localIPs,
	}
	var err error
	ipa.maxmind, err = ipa.load(ctx)
	rtx.Must(err, "Could not load annotation db")
	return ipa
}
