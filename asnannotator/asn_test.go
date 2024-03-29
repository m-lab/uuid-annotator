package asnannotator

import (
	"context"
	"errors"
	"log"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/content"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
)

var local4Rawfile content.Provider
var local6Rawfile content.Provider
var localASNamesfile content.Provider
var corruptFile content.Provider
var localIPs []net.IP

func setUp() {
	var err error
	u4, err := url.Parse("file:../testdata/RouteViewIPv4.pfx2as.gz")
	rtx.Must(err, "Could not parse URL")
	local4Rawfile, err = content.FromURL(context.Background(), u4)
	rtx.Must(err, "Could not create content.Provider")

	u6, err := url.Parse("file:../testdata/RouteViewIPv6.pfx2as.gz")
	rtx.Must(err, "Could not parse URL")
	local6Rawfile, err = content.FromURL(context.Background(), u6)
	rtx.Must(err, "Could not create content.Provider")

	asn, err := url.Parse("file:../data/asnames.ipinfo.csv")
	rtx.Must(err, "Could not parse URL")
	localASNamesfile, err = content.FromURL(context.Background(), asn)
	rtx.Must(err, "Could not create content.Provider")

	cor, err := url.Parse("file:../testdata/corrupt.gz")
	rtx.Must(err, "Could not parse URL")
	corruptFile, err = content.FromURL(context.Background(), cor)
	rtx.Must(err, "Could not create content.Provider")

	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func Test_asnAnnotator_Annotate(t *testing.T) {
	setUp()
	localV4 := "9.0.0.9"
	localV6 := "2002::1"
	localIPs = []net.IP{
		net.ParseIP(localV4),
		net.ParseIP(localV6),
	}
	tests := []struct {
		name    string
		ID      *inetdiag.SockID
		want    *annotator.Annotations
		wantErr bool
	}{
		{
			name: "success",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: localV4,
				DPort: 2,
				DstIP: "1.0.0.1",
			},
			want: &annotator.Annotations{
				// Identify dst as the client.
				Client: annotator.ClientAnnotations{
					Network: &annotator.Network{
						CIDR:     "1.0.0.0/24",
						ASNumber: 13335,
						ASName:   "Cloudflare, Inc.",
						Systems: []annotator.System{
							{ASNs: []uint32{13335}},
						},
					},
				},
			},
		},
		{
			name: "success-multiple-asns",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "223.252.176.1",
				DPort: 2,
				DstIP: localV4,
			},
			want: &annotator.Annotations{
				// Identify src as the client.
				Client: annotator.ClientAnnotations{
					Network: &annotator.Network{
						CIDR:     "223.252.176.0/24",
						ASNumber: 133929,
						ASName:   "TWOWIN CO., LIMITED",
						Systems: []annotator.System{
							{ASNs: []uint32{133929}},
							{ASNs: []uint32{133107}},
						},
					},
				},
			},
		},
		{
			name: "error-unknown-direction",
			ID: &inetdiag.SockID{
				// Neither IP is a localIP.
				SPort: 1,
				SrcIP: "223.252.176.1",
				DPort: 2,
				DstIP: "9.9.9.9",
			},
			want:    &annotator.Annotations{},
			wantErr: true,
		},
		{
			name: "error-bad-ip",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: localV4,
				DPort: 2,
				DstIP: "this-is-not-an-ip",
			},
			want: &annotator.Annotations{
				Client: annotator.ClientAnnotations{
					Network: &annotator.Network{
						Missing: true,
					},
				},
			},
		},
		{
			name: "success-ipv6",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "2001:200::1",
				DPort: 2,
				DstIP: localV6,
			},
			want: &annotator.Annotations{
				Client: annotator.ClientAnnotations{
					Network: &annotator.Network{
						CIDR:     "2001:200::/32",
						ASNumber: 2500,
						ASName:   "WIDE Project",
						Systems: []annotator.System{
							{ASNs: []uint32{2500}},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setUp()
			ctx := context.Background()
			a := New(ctx, local4Rawfile, local6Rawfile, localASNamesfile, localIPs)
			ann := &annotator.Annotations{}
			if err := a.Annotate(tt.ID, ann); (err != nil) != tt.wantErr {
				t.Errorf("asnAnnotator.Annotate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := deep.Equal(ann, tt.want); diff != nil {
				t.Errorf("Annotate() failed; %s", strings.Join(diff, "\n")) // ann, tt.want)
			}
		})
	}
}

func Test_asnAnnotator_AnnotateIP(t *testing.T) {
	setUp()
	localV4 := "9.0.0.9"
	localV6 := "2002::1"
	localIPs = []net.IP{
		net.ParseIP(localV4),
		net.ParseIP(localV6),
	}
	ctx := context.Background()
	a := New(ctx, local4Rawfile, local6Rawfile, localASNamesfile, localIPs)
	got := a.AnnotateIP("2001:200::1")
	want := annotator.Network{
		CIDR:     "2001:200::/32",
		ASNumber: 2500,
		ASName:   "WIDE Project",
		Systems: []annotator.System{
			{ASNs: []uint32{2500}},
		},
	}
	if diff := deep.Equal(*got, want); diff != nil {
		t.Error("got!=want", diff)
	}
}

type badProvider struct {
	err error
}

func (b badProvider) Get(_ context.Context) ([]byte, error) {
	return nil, b.err
}

func Test_asnAnnotator_Reload(t *testing.T) {
	setUp()
	tests := []struct {
		name       string
		as4        content.Provider
		as6        content.Provider
		asnamedata content.Provider
	}{
		{
			name:       "success",
			as4:        local4Rawfile,
			as6:        local6Rawfile,
			asnamedata: localASNamesfile,
		},
		{
			name:       "v4-bad-provider",
			as4:        badProvider{errors.New("fake v4 error")},
			as6:        local6Rawfile,
			asnamedata: localASNamesfile,
		},
		{
			name:       "v4-no-change",
			as4:        badProvider{content.ErrNoChange},
			as6:        local6Rawfile,
			asnamedata: localASNamesfile,
		},
		{
			name:       "bad-v6-provider",
			as4:        local4Rawfile,
			as6:        badProvider{errors.New("fake v6 error")},
			asnamedata: localASNamesfile,
		},
		{
			name:       "v6-no-change",
			as4:        local4Rawfile,
			as6:        badProvider{content.ErrNoChange},
			asnamedata: localASNamesfile,
		},
		{
			name:       "bad-names-provider",
			as4:        local4Rawfile,
			as6:        local6Rawfile,
			asnamedata: badProvider{errors.New("fake v6 error")},
		},
		{
			name:       "names-no-change",
			as4:        local4Rawfile,
			as6:        local6Rawfile,
			asnamedata: badProvider{content.ErrNoChange},
		},
		{
			name:       "names-not-a-csv",
			as4:        local4Rawfile,
			as6:        local6Rawfile,
			asnamedata: corruptFile,
		},
		{
			name:       "corrupt-gz",
			as4:        corruptFile,
			as6:        local6Rawfile,
			asnamedata: localASNamesfile,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// NOTE: we don't use New() to allow injecting bad providers.
			a := &asnAnnotator{
				localIPs:   localIPs,
				as4:        tt.as4,
				as6:        tt.as6,
				asnamedata: tt.asnamedata,
			}
			a.Reload(ctx)
		})
	}
}

func Test_loadGZ_errors(t *testing.T) {
	_, err := loadGZ([]byte{})
	if err == nil {
		t.Error("Should have had an error, not nil")
	}
}

func TestNewFake(t *testing.T) {
	f := NewFake()
	f.Reload(context.Background()) // no crash == success
	n1 := f.AnnotateIP("1.2.3.4")
	if n1.ASName != "Test Number Five" {
		t.Error("Bad return value from AnnotateIP for 1.2.3.4:", n1)
	}
	n2 := f.AnnotateIP("1111:2222:3333:4444:5555:6666:7777:8888")
	if n2.ASName != "Test Number Nine" {
		t.Error("Bad return value from AnnotateIP for 1111:2222:3333:4444:5555:6666:7777:8888:", n2)
	}
	n3 := f.AnnotateIP("1.0.0.0")
	if !n3.Missing {
		t.Error("Should have had a missing return value, not", n3)
	}
}

func Test_IPv4Annotator_AnnotateIP(t *testing.T) {
	setUp()
	tests := []struct {
		name string
		addr string
		want annotator.Network
	}{
		{
			name: "success-ipv4",
			addr: "1.0.0.1",
			want: annotator.Network{
				CIDR:     "1.0.0.0/24",
				ASNumber: 13335,
				Systems: []annotator.System{
					{ASNs: []uint32{13335}},
				},
			},
		},
		{
			name: "success-ipv6",
			addr: "2001:200::1",
			want: annotator.Network{
				Missing: true,
			},
		},
	}
	ctx := context.Background()
	a := NewIPv4(ctx, local4Rawfile)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.AnnotateIP(tt.addr)
			if diff := deep.Equal(*got, tt.want); diff != nil {
				t.Error("AnnotateIP() wrong value; got!=want", diff)
			}
		})
	}
}
