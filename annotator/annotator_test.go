package annotator

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"
)

func TestJSONSerialization(t *testing.T) {
	_, err := json.Marshal(Annotations{})
	rtx.Must(err, "Could not serialize annotations to JSON")
}

func TestFindDirection(t *testing.T) {
	tests := []struct {
		name     string
		ID       *inetdiag.SockID
		localIPs []net.IP
		want     Direction
		wantErr  bool
	}{
		{
			name: "success-src-is-server",
			ID: &inetdiag.SockID{
				SrcIP: "1.0.0.1",
				DstIP: "9.0.0.9",
			},
			localIPs: []net.IP{
				net.ParseIP("1.0.0.1"),
			},
			want: SrcIsServer,
		},
		{
			name: "success-dst-is-server",
			ID: &inetdiag.SockID{
				SrcIP: "9.0.0.9",
				DstIP: "1.0.0.1",
			},
			localIPs: []net.IP{
				net.ParseIP("1.0.0.1"),
			},
			want: DstIsServer,
		},
		{
			name: "error-unknown-direction",
			ID: &inetdiag.SockID{
				SrcIP: "9.0.0.9",
				DstIP: "8.0.0.8",
			},
			localIPs: []net.IP{
				net.ParseIP("1.0.0.1"), // neither src, nor dst IP.
			},
			want:    Unknown,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := FindDirection(tt.ID, tt.localIPs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Direction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != dir {
				t.Errorf("Direction() wrong; got = %d, want %d", dir, tt.want)
			}
		})
	}
}

func TestNetwork_FirstASN(t *testing.T) {
	tests := []struct {
		name    string
		Systems []System
		want    uint32
	}{
		{
			name: "success",
			Systems: []System{
				{ASNs: []uint32{111}},
			},
			want: 111,
		},
		{
			name:    "success-systems-empty",
			Systems: []System{},
			want:    0,
		},
		{
			name: "success-asns-empty",
			Systems: []System{
				{ASNs: []uint32{}},
			},
			want: 0,
		},
		{
			name:    "success-nil",
			Systems: nil,
			want:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Network{
				Systems: tt.Systems,
			}
			if got := n.FirstASN(); got != tt.want {
				t.Errorf("Network.FirstASN() = %v, want %v", got, tt.want)
			}
		})
	}
}
