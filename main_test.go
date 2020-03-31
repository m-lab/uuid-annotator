package main

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"testing"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/eventsocket"
	"github.com/m-lab/uuid-annotator/ipservice"
)

func TestMainSmokeTest(t *testing.T) {
	// Set up the local HD.
	dir, err := ioutil.TempDir("", "TestMain")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)

	// Set up global variables.
	mainCtx, mainCancel = context.WithCancel(context.Background())
	mainRunning = make(chan struct{}, 1)
	*eventsocket.Filename = dir + "/eventsocket.sock"
	*ipservice.SocketFilename = dir + "/ipannotator.sock"
	rtx.Must(maxmindurl.Set("file:./testdata/fake.tar.gz"), "Failed to set maxmind url for testing")
	rtx.Must(routeviewv4.Set("file:./testdata/RouteViewIPv4.tiny.gz"), "Failed to set routeview v4 url for testing")
	rtx.Must(routeviewv6.Set("file:./testdata/RouteViewIPv6.tiny.gz"), "Failed to set routeview v6 url for testing")
	rtx.Must(asnameurl.Set("file:./data/asnames.ipinfo.csv"), "Failed to set ipinfo ASName url for testing")
	rtx.Must(siteinfo.Set("file:./testdata/annotations.json"), "Failed to set siteinfo annotations url for testing")
	*hostname = "mlab1.lga03.measurement-lab.org"

	// Now start up a fake eventsocket.
	srv := eventsocket.New(*eventsocket.Filename)
	rtx.Must(srv.Listen(), "Could not listen")
	go srv.Serve(mainCtx)

	// Cancel main after main is running
	go func() {
		<-mainRunning
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
