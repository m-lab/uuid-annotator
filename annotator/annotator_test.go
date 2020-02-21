package annotator

import (
	"encoding/json"
	"net"
	"reflect"
	"testing"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/inetdiag"
)

func TestJSONSerialization(t *testing.T) {
	_, err := json.Marshal(Annotations{})
	rtx.Must(err, "Could not serialize annotations to JSON")
}

func TestDirection(t *testing.T) {
	tests := []struct {
		name        string
		ID          *inetdiag.SockID
		localIPs    []net.IP
		ann         *Annotations
		serverIsSrc bool
		wantErr     bool
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
			ann:         &Annotations{},
			serverIsSrc: true,
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
			ann:         &Annotations{},
			serverIsSrc: false,
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
			ann:     &Annotations{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst, err := Direction(tt.ID, tt.localIPs, tt.ann)
			if (err != nil) != tt.wantErr {
				t.Errorf("Direction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.serverIsSrc {
				if !reflect.DeepEqual(src, &tt.ann.Server) {
					t.Errorf("Direction() src = %v, want %v", src, tt.ann.Server)
				}
			} else {
				if !reflect.DeepEqual(dst, &tt.ann.Server) {
					t.Errorf("Direction() dst = %v, want %v", dst, tt.ann.Client)
				}
			}
		})
	}
}
