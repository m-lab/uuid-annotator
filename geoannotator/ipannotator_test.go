package geoannotator

import (
	"context"
	"errors"
	"log"
	"math"
	"net"
	"net/url"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/pretty"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/oschwald/geoip2-golang"

	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

var localRawfile rawfile.Provider
var localWrongType rawfile.Provider
var localEmpty rawfile.Provider

// Networks taken from https://github.com/maxmind/MaxMind-DB/blob/master/source-data/GeoIP2-City-Test.json
var localIP = "175.16.199.3"
var remoteIP = "2.125.160.216" // includes multiple subdivision annotations.

func init() {
	var err error
	u, err := url.Parse("file:../testdata/fake.tar.gz")
	rtx.Must(err, "Could not parse URL")
	localRawfile, err = rawfile.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create rawfile.Provider")

	u, err = url.Parse("file:../testdata/wrongtype.tar.gz")
	rtx.Must(err, "Could not parse URL")
	localWrongType, err = rawfile.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create rawfile.Provider")

	u, err = url.Parse("file:../testdata/empty.tar.gz")
	rtx.Must(err, "Could not parse URL")
	localEmpty, err = rawfile.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create rawfile.Provider")

	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func TestIPAnnotationS2C(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP(localIP),
	}
	g := New(context.Background(), localRawfile, localaddrs)

	// Try to annotate a S2C connection.
	conn := &inetdiag.SockID{
		SrcIP:  localIP, // A local IP
		SPort:  1,
		DstIP:  remoteIP, // One of the IPs in the test data
		DPort:  2,
		Cookie: 3,
	}
	ann := &annotator.Annotations{}
	rtx.Must(g.Annotate(conn, ann), "Could not annotate connection")

	// Latitudes gotten out of the testdata by hand.
	if math.Abs(ann.Client.Geo.Longitude - -1.25) > .01 {
		t.Error("Bad Server latitude:", ann.Client.Geo.Longitude, "!~=", -1.25)
	}
	if math.Abs(ann.Client.Geo.Latitude-51.75) > .01 {
		t.Error("Bad Client latitude:", ann.Client.Geo.Latitude, "!~=", 51.75)
	}

	ann2 := &annotator.Annotations{}
	g.AnnotateIP(net.ParseIP(remoteIP), &ann2.Client.Geo)

	if diff := deep.Equal(ann, ann2); diff != nil {
		log.Println("Annotate and AnnotateIP should do the same thing, but they differ:", diff)
	}

	// Test nil IP
	ann3 := &annotator.Annotations{}
	err := g.AnnotateIP(nil, &ann3.Client.Geo)
	if err == nil {
		t.Error("Should have had a non-nil error from a nil IP")
	}
}

func TestIPAnnotationC2S(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP(localIP),
	}
	g := New(context.Background(), localRawfile, localaddrs)

	// Try to annotate a C2S connection.
	conn := &inetdiag.SockID{
		SrcIP:  remoteIP, // One of the IPs in the test data
		SPort:  1,
		DstIP:  localIP, // A local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	rtx.Must(g.Annotate(conn, ann), "Could not annotate connection")

	// Client and Server should be the same, no matter the order of dst and src.
	// Latitudes gotten out of the testdata by hand.
	if math.Abs(ann.Client.Geo.Longitude - -1.25) > .01 {
		t.Error("Bad Server latitude:", ann.Client.Geo.Longitude, "!~=", -1.25)
	}
	if math.Abs(ann.Client.Geo.Latitude-51.75) > .01 {
		t.Error("Bad Client latitude:", ann.Client.Geo.Latitude, "!~=", 51.75)
	}
}

func TestIPAnnotationBadIP(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP("1.0.0.1"),
	}
	g := New(context.Background(), localRawfile, localaddrs)

	conn := &inetdiag.SockID{
		SrcIP:  "this-is-not-an-IP",
		SPort:  1,
		DstIP:  "1.0.0.1", // A local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := g.Annotate(conn, ann)

	if err == nil {
		t.Errorf("Annotate succeeded with a bad IP: %q", conn.SrcIP)
	}
}

func TestIPAnnotationBadDst(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP("1.0.0.1"),
	}
	g := New(context.Background(), localRawfile, localaddrs)

	conn := &inetdiag.SockID{
		SrcIP:  "1.0.0.1",
		SPort:  1,
		DstIP:  "this is not an IP address",
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := g.Annotate(conn, ann)

	if err == nil {
		t.Errorf("Annotate succeeded with a bad IP: %q", conn.DstIP)
	}
}

func TestIPAnnotationUnknownDirection(t *testing.T) {
	localaddrs := []net.IP{net.ParseIP("1.0.0.1")}
	g := New(context.Background(), localRawfile, localaddrs)

	// Try to annotate a connection with no local IP.
	conn := &inetdiag.SockID{
		SrcIP:  "1.0.2.5", // One of the IPs in the test data
		SPort:  1,
		DstIP:  "1.0.1.1", // Another IP in our test data
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := g.Annotate(conn, ann)
	if err == nil {
		t.Error("Should have had an error due to unknown direction")
	}
}

func TestIPAnnotationUnknownIP(t *testing.T) {
	localaddrs := []net.IP{net.ParseIP("1.0.0.1")}
	g := New(context.Background(), localRawfile, localaddrs)

	// Try to annotate a connection with no local IP.
	conn := &inetdiag.SockID{
		SrcIP:  "127.0.0.1", // A remote IP not in our test data
		SPort:  1,
		DstIP:  "1.0.0.1", // Local IP
		DPort:  2,
		Cookie: 4,
	}

	ann := &annotator.Annotations{}
	err := g.Annotate(conn, ann)
	if err != nil || ann.Client.Geo == nil || !ann.Client.Geo.Missing {
		pretty.Print(ann)
		t.Error("Should have had a client annotation with everything set to Missing, but got", err)
	}
}

type badProvider struct {
	err error
}

func (b badProvider) Get(_ context.Context) ([]byte, error) {
	return nil, b.err
}

func TestIPAnnotationLoadNoChange(t *testing.T) {
	ctx := context.Background()
	fakeReader := geoip2.Reader{}
	g := geoannotator{
		backingDataSource: badProvider{rawfile.ErrNoChange},
		localIPs:          []net.IP{net.ParseIP(localIP)},
		maxmind:           &fakeReader, // NOTE: fake pointer just to verify return value below.
	}

	mm, err := g.load(ctx)
	if err != nil {
		t.Errorf("geoannotator.load() returned error; got %q, want nil", err)
	}
	if mm != g.maxmind {
		t.Errorf("geoannotator.load() did not return expected ptr; got %v, want %v", mm, g.maxmind)
	}
}
func TestIPAnnotationLoadErrors(t *testing.T) {
	ctx := context.Background()
	g := geoannotator{
		backingDataSource: badProvider{errors.New("Error for testing")},
		localIPs:          []net.IP{net.ParseIP(localIP)},
	}
	_, err := g.load(ctx)
	if err == nil {
		t.Error("Should have had a non-nil error due to missing file")
	}

	// load errors should not cause Reload to crash.
	g.Reload(ctx) // No crash == success.

	// Now change the backing source, and the next Reload should load the actual data.
	g.backingDataSource = localRawfile
	g.Reload(ctx)

	// Annotations should now succeed...
	conn := &inetdiag.SockID{
		SrcIP:  localIP, // A local IP
		SPort:  1,
		DstIP:  remoteIP, // One of the IPs in the test data
		DPort:  2,
		Cookie: 3,
	}
	ann := &annotator.Annotations{}
	rtx.Must(g.Annotate(conn, ann), "Could not annotate connection")
}

func TestIPAnnotationWrongDbType(t *testing.T) {
	localIPs := []net.IP{net.ParseIP(localIP)}
	// localWrongType should load successfully, but fail to annotate.
	g := New(context.Background(), localWrongType, localIPs)

	// Annotations should now succeed...
	conn := &inetdiag.SockID{
		SrcIP:  localIP, // A local IP
		SPort:  1,
		DstIP:  remoteIP, // One of the IPs in the test data
		DPort:  2,
		Cookie: 3,
	}
	ann := &annotator.Annotations{}
	err := g.Annotate(conn, ann)

	if err == nil {
		t.Errorf("Annotate() with wrong db type did not return an error: %q", err)
	}
}

func TestIPAnnotationMissingCityDB(t *testing.T) {
	ctx := context.Background()
	g := geoannotator{
		backingDataSource: localEmpty,
		localIPs:          []net.IP{net.ParseIP(localIP)},
	}

	mm, err := g.load(ctx)
	if err != rawfile.ErrFileNotFound {
		t.Errorf("geoannotator.load() returned wrong error; got %q, want %q", err, rawfile.ErrFileNotFound)
	}
	if mm != nil {
		t.Errorf("geoannotator.load() return non-nil ptr; got %v, want nil", mm)
	}
}
