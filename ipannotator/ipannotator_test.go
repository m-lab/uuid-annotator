package ipannotator

import (
	"archive/zip"
	"errors"
	"math"
	"net"
	"net/url"
	"testing"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"

	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/zipfile"
)

var localZipfile zipfile.Provider

func init() {
	var err error
	u, err := url.Parse("file:../testdata/GeoLite2City.zip")
	rtx.Must(err, "Could not parse URL")
	localZipfile, err = zipfile.FromURL(u)
	rtx.Must(err, "Could not create zipfile.Provider")
}

func TestIPAnnotationS2C(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP("1.0.0.1"),
	}
	ipa := New(localZipfile, localaddrs)

	// Try to annotate a S2C connection.
	conn := &inetdiag.SockID{
		SrcIP:  "1.0.0.1", // A local IP
		SPort:  1,
		DstIP:  "1.0.2.2", // One of the IPs in the test data
		DPort:  2,
		Cookie: 3,
	}
	ann := &annotator.Annotations{}
	rtx.Must(ipa.Annotate(conn, ann), "Could not annotate connection")

	// Latitudes gotten out of the testdata by hand.
	if math.Abs(ann.Server.Geo.Latitude - -37.7) > .01 {
		t.Error("Bad Server latitude:", ann.Server.Geo.Latitude, "!~=", -37.7)
	}
	if math.Abs(ann.Client.Geo.Latitude-26.0614) > .01 {
		t.Error("Bad Client latitude:", ann.Client.Geo.Latitude, "!~=", 26.0614)
	}
}

func TestIPAnnotationC2S(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP("1.0.0.1"),
	}
	ipa := New(localZipfile, localaddrs)

	// Try to annotate a C2S connection.
	conn := &inetdiag.SockID{
		SrcIP:  "1.0.2.5", // One of the IPs in the test data
		SPort:  1,
		DstIP:  "1.0.0.1", // A local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	rtx.Must(ipa.Annotate(conn, ann), "Could not annotate connection")

	// Client and Server should be the same, no matter the order of dst and src.
	// Latitudes gotten out of the testdata by hand.
	if math.Abs(ann.Server.Geo.Latitude - -37.7) > .01 {
		t.Error("Bad Server latitude:", ann.Server.Geo.Latitude, "!~=", -37.7)
	}
	if math.Abs(ann.Client.Geo.Latitude-26.0614) > .01 {
		t.Error("Bad Client latitude:", ann.Client.Geo.Latitude, "!~=", 26.0614)
	}
}

func TestIPAnnotationUknownDirection(t *testing.T) {
	localaddrs := []net.IP{net.ParseIP("1.0.0.1")}
	ipa := New(localZipfile, localaddrs)

	// Try to annotate a connection with no local IP.
	conn := &inetdiag.SockID{
		SrcIP:  "1.0.2.5", // One of the IPs in the test data
		SPort:  1,
		DstIP:  "1.0.1.1", // Another IP in our test data
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := ipa.Annotate(conn, ann)
	if err == nil {
		t.Error("Should have had an error due to unknown direction")
	}
}

func TestIPAnnotationUknownIP(t *testing.T) {
	localaddrs := []net.IP{net.ParseIP("1.0.0.1")}
	ipa := New(localZipfile, localaddrs)

	// Try to annotate a connection with no local IP.
	conn := &inetdiag.SockID{
		SrcIP:  "9.9.9.9", // A remote IP not in our test data
		SPort:  1,
		DstIP:  "1.0.0.1", // Local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := ipa.Annotate(conn, ann)
	if !errors.Is(err, annotator.ErrNoAnnotation) {
		t.Error("Should have had an ErrNoAnnotation error due to IP missing from our dataset, but got", err)
	}
}

type badProvider struct{}

func (badProvider) Get() (*zip.Reader, error) {
	return nil, errors.New("Error for testing")
}

func TestIPAnnotationLoadErrors(t *testing.T) {
	ipa := ipannotator{
		backingDataSource: badProvider{},
		localIPs:          []net.IP{net.ParseIP("1.0.0.1")},
	}
	_, err := ipa.load()
	if err == nil {
		t.Error("Should have had a non-nil error due to missing file")
	}

	// load errors should not cause Reload to crash.
	ipa.Reload() // No crash == success.

	// Now change the backing source, and the next Reload should load the actual data.
	ipa.backingDataSource = localZipfile
	ipa.Reload()

	// Annotations should now succeed...
	conn := &inetdiag.SockID{
		SrcIP:  "1.0.0.1", // A local IP
		SPort:  1,
		DstIP:  "1.0.2.2", // One of the IPs in the test data
		DPort:  2,
		Cookie: 3,
	}
	ann := &annotator.Annotations{}
	rtx.Must(ipa.Annotate(conn, ann), "Could not annotate connection")
}
