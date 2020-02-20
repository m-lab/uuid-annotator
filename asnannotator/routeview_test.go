package asnannotator

import (
	"context"
	"errors"
	"log"
	"net"
	"net/url"
	"sync"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

var local4Rawfile rawfile.Provider
var local6Rawfile rawfile.Provider
var corruptFile rawfile.Provider
var localIPs []net.IP

func init() {
	var err error
	u4, err := url.Parse("file:../testdata/RouteViewIPv4.pfx2as.gz")
	rtx.Must(err, "Could not parse URL")
	local4Rawfile, err = rawfile.FromURL(context.Background(), u4)
	rtx.Must(err, "Could not create rawfile.Provider")

	u6, err := url.Parse("file:../testdata/RouteViewIPv6.pfx2as.gz")
	rtx.Must(err, "Could not parse URL")
	local6Rawfile, err = rawfile.FromURL(context.Background(), u6)
	rtx.Must(err, "Could not create rawfile.Provider")

	cor, err := url.Parse("file:../testdata/corrupt.gz")
	rtx.Must(err, "Could not parse URL")
	corruptFile, err = rawfile.FromURL(context.Background(), cor)
	rtx.Must(err, "Could not create rawfile.Provider")

	localIPs = []net.IP{
		net.ParseIP("1.0.0.1"),
		net.ParseIP("2001:200::1"),
	}

	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func Test_asnAnnotator_Annotate(t *testing.T) {

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
				SrcIP: "1.0.0.1",
				DPort: 2,
				DstIP: "9.0.0.9",
			},
			want: &annotator.Annotations{
				// Local IP is identified as the Server with valid ASN value.
				Server: api.Annotations{
					Network: &api.ASData{
						Systems: []api.System{
							{
								ASNs: []uint32{13335},
							},
						},
					},
				},
				// Client will not be populated, b/c 9.0.0.9 is not present in data.
			},
		},
		{
			name: "success-multiple-asns",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "223.252.176.1",
				DPort: 2,
				DstIP: "1.0.0.1",
			},
			want: &annotator.Annotations{
				// Local IP is identified as the Server with valid ASN value.
				Client: api.Annotations{
					Network: &api.ASData{
						Systems: []api.System{
							{ASNs: []uint32{133929}},
							{ASNs: []uint32{133107}},
						},
					},
				},
				Server: api.Annotations{
					Network: &api.ASData{
						Systems: []api.System{
							{ASNs: []uint32{13335}},
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
				SrcIP: "1.0.0.1",
				DPort: 2,
				DstIP: "this-is-not-an-ip",
			},
			want: &annotator.Annotations{
				Server: api.Annotations{Network: &api.ASData{Systems: []api.System{{ASNs: []uint32{13335}}}}},
			},
			wantErr: true,
		},
		{
			name: "success-ipv6",
			ID: &inetdiag.SockID{
				SPort: 1,
				SrcIP: "2001:200::1",
				DPort: 2,
				DstIP: "this-is-not-an-ip",
			},
			want: &annotator.Annotations{
				Server: api.Annotations{Network: &api.ASData{Systems: []api.System{{ASNs: []uint32{2500}}}}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			a := New(ctx, local4Rawfile, local6Rawfile, localIPs)

			ann := &annotator.Annotations{}
			if err := a.Annotate(tt.ID, ann); (err != nil) != tt.wantErr {
				t.Errorf("asnAnnotator.Annotate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := deep.Equal(ann, tt.want); diff != nil {
				t.Errorf("Annotate() failed; got , want %#v", diff) // ann, tt.want)
			}
		})
	}
}

type badProvider struct {
	err error
}

func (b badProvider) Get(_ context.Context) ([]byte, error) {
	return nil, b.err
}

func Test_asnAnnotator_Reload(t *testing.T) {
	type fields struct {
		m        sync.RWMutex
		localIPs []net.IP
		as4      rawfile.Provider
		as6      rawfile.Provider
		asn4     api.Annotator
		asn6     api.Annotator
	}
	tests := []struct {
		name string
		as4  rawfile.Provider
		as6  rawfile.Provider
	}{
		{
			name: "success",
			as4:  local4Rawfile,
			as6:  local6Rawfile,
		},
		{
			name: "v4-bad-provider",
			as4:  badProvider{errors.New("fake v4 error")},
			as6:  local6Rawfile,
		},
		{
			name: "v4-no-change",
			as4:  badProvider{rawfile.ErrNoChange},
			as6:  local6Rawfile,
		},
		{
			name: "bad-v6-provider",
			as4:  local4Rawfile,
			as6:  badProvider{errors.New("fake v6 error")},
		},
		{
			name: "v6-no-change",
			as4:  local4Rawfile,
			as6:  badProvider{rawfile.ErrNoChange},
		},
		{
			name: "corrupt-gz",
			as4:  corruptFile,
			as6:  local6Rawfile,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// NOTE: we don't use New() to allow injecting bad providers.
			a := &asnAnnotator{
				localIPs: localIPs,
				as4:      tt.as4,
				as6:      tt.as6,
			}
			a.Reload(ctx)
		})
	}
}
