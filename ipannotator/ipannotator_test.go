package ipannotator

import (
	"log"
	"math"
	"net"
	"testing"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"

	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/zipfile"
)

func TestIPAnnotation(t *testing.T) {
	localaddrs := []net.IP{
		net.ParseIP("1.0.0.1"),
	}
	fp := zipfile.FromFile("testdata/GeoLite2City.zip")
	ipa := New(fp, localaddrs)

	// Try to annotate a test with the local address as the SrcIP.
	conn := &inetdiag.SockID{
		SrcIP:  "1.0.0.1", // A local IP
		SPort:  1,
		DstIP:  "1.0.2.2", // One of the IPs in the test data
		DPort:  2,
		Cookie: 3,
	}
	// Annotations is initially empty.
	ann := &annotator.Annotations{}
	rtx.Must(ipa.Annotate(conn, ann), "Could not annotate connection")

	// Latitudes gotten out of the testdata by hand.
	if math.Abs(ann.Server.Geo.Latitude - -37.7) > .01 {
		log.Println("Bad Server latitude:", ann.Server.Geo.Latitude, "!~=", -37.7)
	}
	if math.Abs(ann.Client.Geo.Latitude-26.0614) > .01 {
		log.Println("Bad Client latitude:", ann.Client.Geo.Latitude, "!~=", 26.0614)
	}

	// Try to annotate a test with the local address as the DstIP.
	conn = &inetdiag.SockID{
		SrcIP:  "1.0.2.5", // One of the IPs in the test data
		SPort:  1,
		DstIP:  "1.0.0.1", // A local IP
		DPort:  2,
		Cookie: 4,
	}

	// Annotations is initially empty.
	ann = &annotator.Annotations{}
	rtx.Must(ipa.Annotate(conn, ann), "Could not annotate connection")

	// Client and server should be the same, no matter the order of dst and src.
	// Latitudes gotten out of the testdata by hand.
	if math.Abs(ann.Server.Geo.Latitude - -37.7) > .01 {
		log.Println("Bad Server latitude:", ann.Server.Geo.Latitude, "!~=", -37.7)
	}
	if math.Abs(ann.Client.Geo.Latitude-26.0614) > .01 {
		log.Println("Bad Client latitude:", ann.Client.Geo.Latitude, "!~=", 26.0614)
	}
}
