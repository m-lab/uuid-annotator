package main

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/m-lab/tcp-info/eventsocket"

	"github.com/m-lab/go/rtx"
)

func TestMainSmokeTest(t *testing.T) {
	// Set up the local HD.
	dir, err := ioutil.TempDir("", "TestMain")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)

	// Set up global variables.
	mainCtx, mainCancel = context.WithCancel(context.Background())
	*eventsocket.Filename = dir + "/eventsocket.sock"
	*maxmindurl = "file:./testdata/GeoLite2City.zip"

	// Now start up a fake eventsocket.
	srv := eventsocket.New(*eventsocket.Filename)
	srv.Listen()
	go srv.Serve(mainCtx)

	// Cancel main after a tenth of a second.
	go func() {
		time.Sleep(100 * time.Millisecond)
		mainCancel()
	}()

	// Run main. Full coverage, no crash, and return == success!
	main()
}
