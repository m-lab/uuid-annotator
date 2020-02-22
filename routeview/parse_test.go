package routeview

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"testing"

	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/annotation-service/asn"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/uuid-annotator/rawfile"
)

var rv IPNetSlice
var an api.Annotator

func init() {
	var err error
	b, err := ioutil.ReadFile("../testdata/RouteViewIPv4.pfx2as.gz")
	rtx.Must(err, "Failed to read routeview data")

	b2, err := rawfile.FromGZ(b)
	rtx.Must(err, "Failed to decompress routeview")
	rv = ParseRouteView(b2)

	an, err = asn.LoadASNDatasetFromReader(bytes.NewBuffer(b2))
	rtx.Must(err, "Failed to load api.Annotator")
}

func TestParse(t *testing.T) {
	type args struct {
		file []byte
	}
	tests := []struct {
		name string
		args args
		want []net.IPNet
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseRouteView(tt.args.file); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
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
		_, err := Search(rv, src)
		if err != nil {
			missing++
		} else {
			found++
		}
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
			panic(err)
			missing++
		} else {
			found++
		}
		ann.Network = nil
	}
	fmt.Println("f:", found, "m:", missing)
}

// fmt.Println(n, src)

func TrySearch(n IPNetSlice) {

	tests := []struct {
		src  string
		want net.IPNet
	}{
		{src: "1.0.0.1"},
		{src: "1.0.174.1"},
		{src: "1.0.192.1"},
		{src: "9.9.0.1"},
	}
	for _, t := range tests {
		r, err := Search(n, t.src)
		fmt.Println("found:", r, err)
	}

}
