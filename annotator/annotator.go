// Package annotator provides structs and interfaces used throughout the
// program.
package annotator

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/m-lab/tcp-info/inetdiag"
)

var (
	// ErrNoAnnotation is for when we could not perform an annotation for some
	// reason. It is intended to convey that the annotation system is still
	// functioning fine, but one or more of the annotations you asked for could
	// not be performed.
	ErrNoAnnotation = errors.New("Could not annotate IP address")
)

// The Geolocation struct contains all the information needed for the
// geolocation MaxMind Geo1 and Geo2 data used to annotate BigQuery rows.
//
// This is in common because it is used by the etl repository.
type Geolocation struct {
	ContinentCode string `json:",omitempty"` // Gives a shorthand for the continent
	CountryCode   string `json:",omitempty"` // Gives a shorthand for the country
	CountryCode3  string `json:",omitempty"` // Geo1: Gives a shorthand for the country
	CountryName   string `json:",omitempty"` // Name of the country
	Region        string `json:",omitempty"` // Geo1: Region or State within the country

	// Subdivision fields are provided by MaxMind Geo2 format and used by uuid-annotator.
	Subdivision1ISOCode string `json:",omitempty"`
	Subdivision1Name    string `json:",omitempty"`
	Subdivision2ISOCode string `json:",omitempty"`
	Subdivision2Name    string `json:",omitempty"`

	MetroCode        int64   `json:",omitempty"` // Metro code within the country
	City             string  `json:",omitempty"` // City within the region
	AreaCode         int64   `json:",omitempty"` // Geo1: Area code, similar to metro code
	PostalCode       string  `json:",omitempty"` // Postal code, again similar to metro
	Latitude         float64 `json:",omitempty"` // Latitude
	Longitude        float64 `json:",omitempty"` // Longitude
	AccuracyRadiusKm int64   `json:",omitempty"` // Geo2: Accuracy Radius (since 2018)

	Missing bool `json:",omitempty"` // True when the Geolocation data is missing from MaxMind.
}

// We currently use CAIDA RouteView data to populate ASN annotations.
// See documentation at:
// http://data.caida.org/datasets/routing/routeviews-prefix2as/README.txt

// A System is the base element. It may contain a single ASN or multiple ASNs
// comprising an AS set.
type System struct {
	// ASNs contains a single ASN, or AS set. There must always be at least one
	// ASN. If there are more than one ASN, they will be listed in the same order
	// as RouteView.
	ASNs []uint32
}

// Network contains the Autonomous System information associated with the IP prefix.
// Roughly 99% of mappings consist of a single System with a single ASN.
type Network struct {
	CIDR     string `json:",omitempty"` // The IP prefix found in the RouteView data.
	ASNumber uint32 `json:",omitempty"` // First AS number.
	ASName   string `json:",omitempty"` // Place holder for AS name.
	Missing  bool   `json:",omitempty"` // True when the ASN data is missing from RouteView.

	// Systems may contain data for Multi-Origin ASNs. Typically, RouteView
	// records a single ASN per netblock.
	Systems []System `json:",omitempty"`
}

func (n *Network) FirstASN() uint32 {
	if n.Systems == nil || len(n.Systems) == 0 {
		return 0
	}
	s0 := n.Systems[0]
	if len(s0.ASNs) == 0 {
		return 0
	}
	return s0.ASNs[0]
}

// ClientAnnotations are client-specific fields for annotation metadata with
// pointers to geo location and ASN data.
type ClientAnnotations struct {
	Geo     *Geolocation `json:",omitempty"` // Holds the Client geolocation data
	Network *Network     `json:",omitempty"` // Holds the Autonomous System data.
}

// ServerAnnotations are server-specific fields populated by the uuid-annotator.
type ServerAnnotations struct {
	Site    string       `json:",omitempty"` // M-Lab site, i.e. lga01, yyz02, etc.
	Machine string       `json:",omitempty"` // Specific M-Lab machine at a site, i.e. "mlab1", "mlab2", etc.
	Geo     *Geolocation `json:",omitempty"` // Holds the Server geolocation data.
	Network *Network     `json:",omitempty"` // Holds the Autonomous System data.
}

// Annotations contains the standard columns we would like to add as annotations for every UUID.
type Annotations struct {
	UUID      string
	Timestamp time.Time
	Server    ServerAnnotations `json:",omitempty" bigquery:"server"` // Use Standard Top-Level Column names.
	Client    ClientAnnotations `json:",omitempty" bigquery:"client"` // Use Standard Top-Level Column names.
}

// Annotator is the interface that all systems that want to add metadata should implement.
type Annotator interface {
	Annotate(ID *inetdiag.SockID, annotations *Annotations) error
}

// Direction gives us an enum to keep track of which end of the connection is
// the server, because we are informed of connections without regard to which
// end is the local server.
type Direction int

// Specific directions.
const (
	Unknown Direction = iota
	SrcIsServer
	DstIsServer
)

// FindDirection determines whether the IPs in the given ID map to the server or client annotations.
// FindDirection returns the corresponding "src" and "dst" annotation fields from the given annotator.Annotations.
func FindDirection(ID *inetdiag.SockID, localIPs []net.IP) (Direction, error) {
	for _, local := range localIPs {
		if ID.SrcIP == local.String() {
			return SrcIsServer, nil
		}
		if ID.DstIP == local.String() {
			return DstIsServer, nil
		}
	}
	return Unknown, fmt.Errorf("Can't annotate connection: Unknown direction for %+v", ID)
}
