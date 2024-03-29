package siteannotator

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/content"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
)

type badProvider struct {
	err error
}

func (b badProvider) Get(_ context.Context) ([]byte, error) {
	return nil, b.err
}

var (
	localRawfile content.Provider
	corruptFile  content.Provider
)

func setUp() {
	u, err := url.Parse("file:../testdata/annotations.json")
	rtx.Must(err, "Could not parse URL")
	localRawfile, err = content.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create content.Provider")

	u, err = url.Parse("file:../testdata/corrupt-annotations.json")
	rtx.Must(err, "Could not parse URL")
	corruptFile, err = content.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create content.Provider")
}

func TestNew(t *testing.T) {
	setUp()
	minimalServerAnn := func(site, cidr string) annotator.ServerAnnotations {
		return annotator.ServerAnnotations{
			Site:    site,
			Machine: "mlab1",
			Geo: &annotator.Geolocation{
				City: "New York",
			},
			Network: &annotator.Network{
				CIDR:   cidr,
				ASName: "TATA COMMUNICATIONS (AMERICA) INC",
			},
		}
	}
	defaultServerAnnV4 := annotator.ServerAnnotations{
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
			CIDR:     "64.86.148.128/26",
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
		provider *content.Provider
		hostname string
		ID       *inetdiag.SockID
		want     annotator.Annotations
		wantErr  bool
	}{
		{
			name:     "success-src",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: &localRawfile,
			hostname: "mlab1-lga03.mlab-sandbox.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "1.0.0.1",
				DPort: 2,
				DstIP: "64.86.148.137",
			},
			want: annotator.Annotations{
				Server: defaultServerAnnV4,
			},
		},
		{
			name:     "success-dest",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: &localRawfile,
			hostname: "mlab1-lga03.mlab-sandbox.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "64.86.148.137",
				DPort: 2,
				DstIP: "1.0.0.1",
			},
			want: annotator.Annotations{
				Server: defaultServerAnnV4,
			},
		},
		{
			name:     "success-no-ipv4-config-with-ipv6-connection",
			localIPs: []net.IP{net.ParseIP("2001:5a0:4300::2")},
			provider: &localRawfile,
			hostname: "mlab1-six02.mlab-sandbox.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "2001:5a0:4300::2",
				DPort: 2,
				DstIP: "2600::1",
			},
			want: annotator.Annotations{
				Server: minimalServerAnn("six02", "2001:5a0:4300::/64"),
			},
		},
		{
			name:     "success-no-ipv4-config-with-ipv4-connection",
			localIPs: []net.IP{net.ParseIP("64.86.148.137")},
			provider: &localRawfile,
			hostname: "mlab1-six02.mlab-sandbox.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "64.86.148.137",
				DPort: 2,
				DstIP: "1.0.0.1",
			},
			want: annotator.Annotations{},
		},
		{
			name:     "success-no-ipv6-config-with-ipv4-connection",
			localIPs: []net.IP{net.ParseIP("64.86.148.130")},
			provider: &localRawfile,
			hostname: "mlab1-six01.mlab-sandbox.measurement-lab.org",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "64.86.148.130",
				DPort: 2,
				DstIP: "1.0.0.1",
			},
			want: annotator.Annotations{
				Server: minimalServerAnn("six01", "64.86.148.128/26"),
			},
		},
		{
			name:     "success-no-ipv6-config-with-ipv6-connection",
			localIPs: []net.IP{net.ParseIP("2001:5a0:4300::2")},
			provider: &localRawfile,
			hostname: "mlab1-six01.mlab-sandbox.measurement-lab.org",
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
			provider: &localRawfile,
			hostname: "mlab1-lga03.mlab-sandbox.measurement-lab.org",
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
			setUp()
			ctx := context.Background()
			g, _ := New(ctx, tt.hostname, *tt.provider, tt.localIPs)
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
	var bad content.Provider
	var testLocalIPs []net.IP = []net.IP{net.ParseIP("10.0.0.1")}
	tests := []struct {
		name         string
		provider     *content.Provider
		hostname     string
		want         *annotator.ServerAnnotations
		wantLocalIPs []net.IP
		wantErr      bool
	}{
		{
			name:     "success",
			provider: &localRawfile,
			hostname: "mlab1-lga03.mlab-sandbox.measurement-lab.org",
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
			wantLocalIPs: testLocalIPs,
		},
		{
			name:     "success-project-flat-name",
			provider: &localRawfile,
			hostname: "mlab1-lga03.mlab-sandbox.measurement-lab.org",
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
			wantLocalIPs: testLocalIPs,
		},
		{
			name:     "success-no-six",
			provider: &localRawfile,
			hostname: "mlab1-six01.mlab-sandbox.measurement-lab.org",
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
			wantLocalIPs: testLocalIPs,
		},
		{
			name:     "success-append-localips",
			provider: &localRawfile,
			hostname: "mlab1-six06.mlab-sandbox.measurement-lab.org",
			want: &annotator.ServerAnnotations{
				Site:    "six06",
				Machine: "mlab1",
				Geo: &annotator.Geolocation{
					City: "New York",
				},
				Network: &annotator.Network{
					ASName: "TATA COMMUNICATIONS (AMERICA) INC",
				},
			},
			wantLocalIPs: append(testLocalIPs, net.ParseIP("64.86.148.129").To4(), net.ParseIP("2001:5a0:4300::")),
		},
		{
			name:     "error-bad-ipv4",
			provider: &localRawfile,
			hostname: "mlab1-bad04.mlab-sandbox.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-bad-ipv6",
			provider: &localRawfile,
			hostname: "mlab1-bad06.mlab-sandbox.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-loading-provider",
			provider: &bad,
			hostname: "mlab1-lga03.mlab-sandbox.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-corrupt-json",
			provider: &corruptFile,
			hostname: "mlab1-lga03.mlab-sandbox.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-bad-hostname",
			provider: &localRawfile,
			hostname: "this-is-not-a-hostname",
			wantErr:  true,
		},
		{
			name:     "error-bad-name-separator",
			provider: &localRawfile,
			hostname: "mlab1=lga03.mlab-sandbox.measurement-lab.org",
			wantErr:  true,
		},
		{
			name:     "error-hostname-not-in-annotations",
			provider: &localRawfile,
			hostname: "mlab1-abc01.mlab-sandbox.measurement-lab.org",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setUp()
			bad = &badProvider{fmt.Errorf("Fake load error")}
			g := &siteAnnotator{
				siteinfoSource: *tt.provider,
				hostname:       tt.hostname,
			}
			ctx := context.Background()
			an, localIPs, err := g.load(ctx, testLocalIPs)
			if !reflect.DeepEqual(localIPs, tt.wantLocalIPs) {
				t.Errorf("srvannotator.load() want localIPs %v, got %v", tt.wantLocalIPs, localIPs)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("srvannotator.Annotate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := deep.Equal(an, tt.want); diff != nil {
				t.Errorf("Annotate() failed; %s", strings.Join(diff, "\n")) // ann, tt.want)
			}
		})
	}
}
