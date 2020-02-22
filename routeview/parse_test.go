package routeview

import (
	"io/ioutil"
	"net"
	"reflect"
	"testing"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/uuid-annotator/rawfile"
)

func init() {
	b, err := ioutil.ReadFile("../testdata/RouteViewIPv4.pfx2as.gz")
	rtx.Must(err, "Failed to read routeview data")

	b2, err := rawfile.FromGZ(b)
	rtx.Must(err, "Failed to decompress routeview")
	TrySearch(Parse(b2))
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
			if got := Parse(tt.args.file); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
