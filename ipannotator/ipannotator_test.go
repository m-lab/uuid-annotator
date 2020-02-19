package ipannotator

import (
	"context"
	"errors"
	"log"
	"math"
	"net"
	"net/url"
	"testing"

	"github.com/m-lab/go/pretty"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"

	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

var localRawfile rawfile.Provider

// Networks taken from https://github.com/maxmind/MaxMind-DB/blob/master/source-data/GeoIP2-City-Test.json
var localIP = "175.16.199.3"
var remoteIP = "202.196.224.5"

func init() {
	var err error
	// u, err := url.Parse("file:../testdata/GeoLite2-City.tar.gz")
	u, err := url.Parse("file:../testdata/fake.tar.gz")
	rtx.Must(err, "Could not parse URL")
	localRawfile, err = rawfile.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create rawfile.Provider")
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func TestIPAnnotationS2C(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP(localIP),
	}
	ipa := New(context.Background(), localRawfile, localaddrs)

	// Try to annotate a S2C connection.
	conn := &inetdiag.SockID{
		SrcIP:  localIP, // A local IP
		SPort:  1,
		DstIP:  remoteIP, // One of the IPs in the test data
		DPort:  2,
		Cookie: 3,
	}
	ann := &annotator.Annotations{}
	rtx.Must(ipa.Annotate(conn, ann), "Could not annotate connection")

	// Latitudes gotten out of the testdata by hand.
	if math.Abs(ann.Server.Geo.Latitude-43.88) > .01 {
		t.Error("Bad Server latitude:", ann.Server.Geo.Latitude, "!~=", 43.88)
	}
	if math.Abs(ann.Client.Geo.Latitude-13) > .01 {
		t.Error("Bad Client latitude:", ann.Client.Geo.Latitude, "!~=", 13)
	}
}

func TestIPAnnotationC2S(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP(localIP),
	}
	ipa := New(context.Background(), localRawfile, localaddrs)

	// Try to annotate a C2S connection.
	conn := &inetdiag.SockID{
		SrcIP:  remoteIP, // One of the IPs in the test data
		SPort:  1,
		DstIP:  localIP, // A local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	rtx.Must(ipa.Annotate(conn, ann), "Could not annotate connection")

	// Client and Server should be the same, no matter the order of dst and src.
	// Latitudes gotten out of the testdata by hand.
	if math.Abs(ann.Server.Geo.Latitude-43.88) > .01 {
		t.Error("Bad Server latitude:", ann.Server.Geo.Latitude, "!~=", -43.88)
	}
	if math.Abs(ann.Client.Geo.Latitude-13) > .01 {
		t.Error("Bad Client latitude:", ann.Client.Geo.Latitude, "!~=", 13)
	}
}

func TestIPAnnotationBadIP(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP("1.0.0.1"),
	}
	ipa := New(context.Background(), localRawfile, localaddrs)

	conn := &inetdiag.SockID{
		SrcIP:  "this-is-not-an-IP",
		SPort:  1,
		DstIP:  "1.0.0.1", // A local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := ipa.Annotate(conn, ann)

	if err == nil {
		t.Errorf("Annotate succeeded with a bad IP: %q", conn.SrcIP)
	}
}

func TestIPAnnotationBadDst(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP("1.0.0.1"),
	}
	ipa := New(context.Background(), localRawfile, localaddrs)

	conn := &inetdiag.SockID{
		SrcIP:  "1.0.0.1",
		SPort:  1,
		DstIP:  "0.0.0.0", // A local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := ipa.Annotate(conn, ann)

	if err == nil {
		t.Errorf("Annotate succeeded with a bad IP: %q", conn.SrcIP)
	}
}

func TestIPAnnotationUknownDirection(t *testing.T) {
	localaddrs := []net.IP{net.ParseIP("1.0.0.1")}
	ipa := New(context.Background(), localRawfile, localaddrs)

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
	ipa := New(context.Background(), localRawfile, localaddrs)

	// Try to annotate a connection with no local IP.
	conn := &inetdiag.SockID{
		SrcIP:  "127.0.0.1", // A remote IP not in our test data
		SPort:  1,
		DstIP:  "1.0.0.1", // Local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := ipa.Annotate(conn, ann)
	if !errors.Is(err, annotator.ErrNoAnnotation) {
		pretty.Print(ann)
		t.Error("Should have had an ErrNoAnnotation error due to IP missing from our dataset, but got", err)
	}
}

type badProvider struct{}

func (badProvider) Get(_ context.Context) ([]byte, error) {
	return nil, errors.New("Error for testing")
}

func TestIPAnnotationLoadErrors(t *testing.T) {
	ctx := context.Background()
	ipa := ipannotator{
		backingDataSource: badProvider{},
		localIPs:          []net.IP{net.ParseIP(localIP)},
	}
	_, err := ipa.load(ctx)
	if err == nil {
		t.Error("Should have had a non-nil error due to missing file")
	}

	// load errors should not cause Reload to crash.
	ipa.Reload(ctx) // No crash == success.

	// Now change the backing source, and the next Reload should load the actual data.
	ipa.backingDataSource = localRawfile
	ipa.Reload(ctx)

	// Annotations should now succeed...
	conn := &inetdiag.SockID{
		SrcIP:  localIP, // A local IP
		SPort:  1,
		DstIP:  remoteIP, // One of the IPs in the test data
		DPort:  2,
		Cookie: 3,
	}
	ann := &annotator.Annotations{}
	rtx.Must(ipa.Annotate(conn, ann), "Could not annotate connection")
}
