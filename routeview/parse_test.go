package routeview

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"reflect"
	"testing"

	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/annotation-service/asn"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/tarreader"
)

var ns Index
var an api.Annotator

func init() {
	var err error
	// Load file in setup for Benchmark.
	b, err := ioutil.ReadFile("../testdata/RouteViewIPv4.pfx2as.gz")
	rtx.Must(err, "Failed to read routeview data")
	b2, err := tarreader.FromGZ(b)
	rtx.Must(err, "Failed to decompress routeview")
	ns = ParseRouteView(b2)

	// Only used for Benchmark.
	an, err = asn.LoadASNDatasetFromReader(bytes.NewBuffer(b2))
	rtx.Must(err, "Failed to load api.Annotator")

	log.SetFlags(0)
}

func TestParseRouteView(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		wantCount int
	}{
		{
			name:      "success-ipv4",
			filename:  "../testdata/RouteViewIPv4.pfx2as.gz",
			wantCount: 545957,
		},
		{
			name:      "success-ipv6",
			filename:  "../testdata/RouteViewIPv6.pfx2as.gz",
			wantCount: 54317,
		},
		{
			name:      "corrupt-ipv4",
			filename:  "../testdata/RouteViewIPv4.corrupt.gz",
			wantCount: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gz, err := ioutil.ReadFile(tt.filename)
			rtx.Must(err, "Failed to read routeview data")
			b, err := tarreader.FromGZ(gz)
			rtx.Must(err, "Failed to decompress routeview")

			ns := ParseRouteView(b)
			if len(ns) != tt.wantCount {
				t.Errorf("Parse() = %v, want %v", len(ns), tt.wantCount)
			}
		})
	}
}

func TestParseSystems(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want []annotator.System
	}{
		{
			name: "success",
			s:    "12345",
			want: []annotator.System{
				{ASNs: []uint32{12345}},
			},
		},
		{
			name: "success-multiple-as",
			s:    "12345,54321",
			want: []annotator.System{
				{ASNs: []uint32{12345, 54321}},
			},
		},
		{
			name: "success-multi-origin-as",
			s:    "12345_54321",
			want: []annotator.System{
				{ASNs: []uint32{12345}},
				{ASNs: []uint32{54321}},
			},
		},
		{
			name: "success-ignore-bad-format",
			s:    "this-is-not-an-asn",
			want: []annotator.System{
				{ASNs: []uint32{}},
			},
		},
		{
			name: "success-skip-bad-format",
			s:    "1234,this-is-not-an-asn",
			want: []annotator.System{
				{ASNs: []uint32{1234}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseSystems(tt.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSystems() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIndex_Search(t *testing.T) {

	tests := []struct {
		name     string
		filename string
		src      string
		want     IPNet
		wantErr  bool
	}{
		{
			name:     "success",
			filename: "../testdata/RouteViewIPv4.pfx2as.gz",
			src:      "1.0.192.1",
			want: IPNet{
				IPNet:   net.IPNet{IP: net.ParseIP("1.0.128.0").To4(), Mask: net.CIDRMask(17, 32)},
				Systems: "23969",
			},
		},
		{
			name:     "success-ipv6",
			filename: "../testdata/RouteViewIPv6.pfx2as.gz",
			src:      "2001:200::1",
			want: IPNet{

				IPNet:   net.IPNet{IP: net.ParseIP("2001:200::"), Mask: net.CIDRMask(32, 128)},
				Systems: "2500",
			},
		},
		{
			name:     "error-not-found-ipv6",
			filename: "../testdata/RouteViewIPv4.pfx2as.gz",
			src:      "2001:ff00::1", // IPv6 address will not be found in IPv4 views.
			wantErr:  true,
		},
		{
			name:     "error-not-found-ipv4",
			filename: "../testdata/RouteViewIPv4.pfx2as.gz",
			src:      "9.0.0.9", // not present in IPv4 route view.
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gz, err := ioutil.ReadFile(tt.filename)
			rtx.Must(err, "Failed to read routeview data")
			b, err := tarreader.FromGZ(gz)
			rtx.Must(err, "Failed to decompress routeview")
			ns := ParseRouteView(b)

			got, err := ns.Search(tt.src)
			if (err != nil) != tt.wantErr {
				t.Errorf("Index.Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got.IPNet, tt.want.IPNet) {
				t.Errorf("Index.Search() = %v, want %v", got.IPNet, tt.want.IPNet)
			}
			if got.Systems != tt.want.Systems {
				t.Errorf("Index.Search() returned wrong Systems = %q, want %q", got.Systems, tt.want.Systems)
			}
		})
	}
}

func BenchmarkSearch(b *testing.B) {
	found := 0
	missing := 0
	src := "1.0.192.1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := ns.Search(src)
		if err != nil {
			missing++
		} else {
			found++
		}
		_ = ParseSystems(r.Systems)
	}
	fmt.Println("f:", found, "m:", missing)
}

func BenchmarkAnnotate(b *testing.B) {
	found := 0
	missing := 0
	src := "1.0.192.1"
	ann := api.Annotations{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := an.Annotate(src, &ann)
		if err != nil {
			missing++
		} else {
			found++
		}
		ann.Network = nil
	}
	fmt.Println("f:", found, "m:", missing)
}
