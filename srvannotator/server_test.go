package srvannotator

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

type badProvider struct {
	err error
}

func (b badProvider) Get(_ context.Context) ([]byte, error) {
	return nil, b.err
}

func TestNew(t *testing.T) {
	u, err := url.Parse("file:../testdata/annotations.json")
	rtx.Must(err, "Could not parse URL")
	localRawfile, err := rawfile.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create rawfile.Provider")

	tests := []struct {
		name     string
		localIPs []net.IP
		provider rawfile.Provider
		hostname string
		ID       *inetdiag.SockID
		wantErr  bool
	}{
		{
			name:     "success-src",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: localRawfile,
			hostname: "mlab1.lga03.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "1.0.0.1",
				DPort: 2,
				DstIP: "64.86.148.137",
			},
		},
		{
			name:     "success-dest",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: localRawfile,
			hostname: "mlab1.lga03.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "64.86.148.137",
				DPort: 2,
				DstIP: "1.0.0.1",
			},
		},
		{
			name:     "error-neither-ips-are-server",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: localRawfile,
			hostname: "mlab1.lga03.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "2.0.0.2",
				DPort: 2,
				DstIP: "1.0.0.1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			g := New(ctx, tt.hostname, tt.provider, tt.localIPs)
			ann := annotator.Annotations{}
			if err := g.Annotate(tt.ID, &ann); (err != nil) != tt.wantErr {
				t.Errorf("srvannotator.Annotate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func Test_srvannotator_load(t *testing.T) {
	u, err := url.Parse("file:../testdata/annotations.json")
	rtx.Must(err, "Could not parse URL")
	localRawfile, err := rawfile.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create rawfile.Provider")

	u, err = url.Parse("file:../testdata/corrupt-annotations.json")
	rtx.Must(err, "Could not parse URL")
	corruptFile, err := rawfile.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create rawfile.Provider")

	tests := []struct {
		name     string
		localIPs []net.IP
		provider rawfile.Provider
		hostname string
		ID       *inetdiag.SockID
		want     *annotator.ServerAnnotations
		wantErr  bool
	}{
		{
			name:     "success",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: localRawfile,
			hostname: "mlab1.lga03.measurement-lab.org",
			want: &annotator.ServerAnnotations{
				Site:    "lga03",
				Machine: "mlab1",
				Geo: &annotator.Geolocation{
					ContinentCode: "NA",
					CountryCode:   "US",
					City:          "New York",
					Latitude:      40.7667,
					Longitude:     -73.8667,
				},
				Network: &annotator.Network{
					ASNumber: 6453,
					ASName:   "TATA COMMUNICATIONS (AMERICA) INC",
					Systems: []annotator.System{
						{ASNs: []uint32{6453}},
					},
				},
			},
		},
		{
			name:     "error-loading-provider",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: &badProvider{fmt.Errorf("Fake load error")},
			hostname: "mlab1.lga03.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-corrupt-json",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: corruptFile,
			hostname: "mlab1.lga03.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-bad-hostname",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: localRawfile,
			hostname: "this-is-not-a-hostname",
			wantErr:  true,
		},
		{
			name:     "error-hostname-not-in-annotations",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: localRawfile,
			hostname: "mlab1.none0.measurement-lab.org",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &srvannotator{
				siteinfoSource: tt.provider,
				hostname:       tt.hostname,
				localIPs:       tt.localIPs,
			}
			ctx := context.Background()
			an, err := g.load(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("srvannotator.Annotate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := deep.Equal(an, tt.want); diff != nil {
				t.Errorf("Annotate() failed; %s", strings.Join(diff, "\n")) // ann, tt.want)
			}
		})
	}
}
