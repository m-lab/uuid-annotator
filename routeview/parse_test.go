package routeview

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/annotation-service/asn"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

var rv *Index
var an api.Annotator

func init() {
	var err error
	// Load file in setup for Benchmark.
	b, err := ioutil.ReadFile("../testdata/RouteViewIPv4.pfx2as.gz")
	rtx.Must(err, "Failed to read routeview data")
	b2, err := rawfile.FromGZ(b)
	rtx.Must(err, "Failed to decompress routeview")
	rv = ParseRouteView(b2)

	// Only used for Benchmark.
	an, err = asn.LoadASNDatasetFromReader(bytes.NewBuffer(b2))
	rtx.Must(err, "Failed to load api.Annotator")
}

func TestParseRouteView(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		wantCount int
	}{
		{
			name:      "success",
			filename:  "../testdata/RouteViewIPv4.pfx2as.gz",
			wantCount: 545957,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gz, err := ioutil.ReadFile(tt.filename)
			rtx.Must(err, "Failed to read routeview data")
			b, err := rawfile.FromGZ(gz)
			rtx.Must(err, "Failed to decompress routeview")

			rv := ParseRouteView(b)
			if len(rv.n) != tt.wantCount {
				t.Errorf("Parse() = %v, want %v", len(rv.n), tt.wantCount)
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

func BenchmarkSearch(b *testing.B) {
	found := 0
	missing := 0
	src := "1.0.192.1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := rv.Search(src)
		if err != nil {
			missing++
		} else {
			found++
		}
		_ = ParseSystems(*r.Systems)
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
