package main

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/eventsocket"
)

func TestMainSmokeTest(t *testing.T) {
	// Set up the local HD.
	dir, err := ioutil.TempDir("", "TestMain")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)

	// Set up global variables.
	mainCtx, mainCancel = context.WithCancel(context.Background())
	*eventsocket.Filename = dir + "/eventsocket.sock"
	*maxmindurl = "file:./testdata/GeoLite2-City.tar.gz"

	// Now start up a fake eventsocket.
	srv := eventsocket.New(*eventsocket.Filename)
	srv.Listen()
	go srv.Serve(mainCtx)

	// Cancel main after a tenth of a second.
	go func() {
		time.Sleep(1000 * time.Millisecond)
		mainCancel()
	}()

	// Run main. Full coverage, no crash, and return == success!
	main()
}

func Test_findLocalIPs(t *testing.T) {
	tests := []struct {
		name  string
		local []net.Addr
		want  []net.IP
	}{
		{
			name: "success",
			local: []net.Addr{
				&net.IPNet{
					IP:   net.ParseIP("127.0.0.1"),
					Mask: net.CIDRMask(24, 32),
				},
				&net.IPNet{
					IP:   net.ParseIP("2001:1900:2100:2d::75"),
					Mask: net.CIDRMask(64, 128),
				},
			},
			want: []net.IP{
				net.ParseIP("127.0.0.1"),
				net.ParseIP("2001:1900:2100:2d::75"),
			},
		},
		{
			name: "skip-all-return-empty",
			local: []net.Addr{
				&net.UnixAddr{
					Name: "fake-unix",
					Net:  "unix",
				},
			},
			want: []net.IP{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findLocalIPs(tt.local); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findLocalIPs() = %v, want %v", got, tt.want)
			}
		})
	}
}
