package siteannotator

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
	log.SetFlags(0)
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

	minimalServerAnn := func(site string) annotator.ServerAnnotations {
		return annotator.ServerAnnotations{
			Site:    site,
			Machine: "mlab1",
			Geo: &annotator.Geolocation{
				City: "New York",
			},
			Network: &annotator.Network{
				ASName: "TATA COMMUNICATIONS (AMERICA) INC",
			},
		}
	}
	defaultServerAnn := annotator.ServerAnnotations{
		Machine: "mlab1",
		Site:    "lga03",
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
	}

	tests := []struct {
		name     string
		localIPs []net.IP
		provider rawfile.Provider
		hostname string
		ID       *inetdiag.SockID
		want     annotator.Annotations
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
			want: annotator.Annotations{
				Server: defaultServerAnn,
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
			want: annotator.Annotations{
				Server: defaultServerAnn,
			},
		},
		{
			name:     "success-empty-ipv4-with-ipv6-connection",
			localIPs: []net.IP{net.ParseIP("2001:5a0:4300::2")},
			provider: localRawfile,
			hostname: "mlab1.six02.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "2001:5a0:4300::2",
				DPort: 2,
				DstIP: "2600::1",
			},
			want: annotator.Annotations{
				Server: minimalServerAnn("six02"),
			},
		},
		{
			name:     "success-empty-ipv4-with-ipv4-connection",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: localRawfile,
			hostname: "mlab1.six02.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "64.86.148.137",
				DPort: 2,
				DstIP: "1.0.0.1",
			},
			want: annotator.Annotations{},
		},
		{
			name:     "success-empty-ipv6-with-ipv4-connection",
			localIPs: []net.IP{net.ParseIP("64.86.148.130")},
			provider: localRawfile,
			hostname: "mlab1.six01.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "64.86.148.130",
				DPort: 2,
				DstIP: "1.0.0.1",
			},
			want: annotator.Annotations{
				Server: minimalServerAnn("six01"),
			},
		},
		{
			name:     "success-empty-ipv6-with-ipv6-connection",
			localIPs: []net.IP{net.ParseIP("2001:5a0:4300::2")},
			provider: localRawfile,
			hostname: "mlab1.six01.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "2001:5a0:4300::2",
				DPort: 2,
				DstIP: "2600::1",
			},
			want: annotator.Annotations{},
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
			wantErr: true,
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
			if diff := deep.Equal(ann, tt.want); diff != nil {
				t.Errorf("Annotate() failed; %s", strings.Join(diff, "\n"))
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
		provider rawfile.Provider
		hostname string
		ID       *inetdiag.SockID
		want     *annotator.ServerAnnotations
		wantErr  bool
	}{
		{
			name:     "success",
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
			name:     "success-project-flat-name",
			provider: localRawfile,
			hostname: "mlab1-lga03.mlab-oti.measurement-lab.org",
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
			name:     "success-no-six",
			provider: localRawfile,
			hostname: "mlab1.six01.measurement-lab.org",
			want: &annotator.ServerAnnotations{
				Site:    "six01",
				Machine: "mlab1",
				Geo: &annotator.Geolocation{
					City: "New York",
				},
				Network: &annotator.Network{
					ASName: "TATA COMMUNICATIONS (AMERICA) INC",
				},
			},
		},
		{
			name:     "error-bad-ipv4",
			provider: localRawfile,
			hostname: "mlab1.bad04.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-bad-ipv6",
			provider: localRawfile,
			hostname: "mlab1.bad06.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-loading-provider",
			provider: &badProvider{fmt.Errorf("Fake load error")},
			hostname: "mlab1.lga03.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-corrupt-json",
			provider: corruptFile,
			hostname: "mlab1.lga03.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-bad-hostname",
			provider: localRawfile,
			hostname: "this-is-not-a-hostname",
			wantErr:  true,
		},
		{
			name:     "error-bad-name-separator",
			provider: localRawfile,
			hostname: "mlab1=lga03.mlab-oti.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-hostname-not-in-annotations",
			provider: localRawfile,
			hostname: "mlab1.abc01.measurement-lab.org",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &siteAnnotator{
				siteinfoSource: tt.provider,
				hostname:       tt.hostname,
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
