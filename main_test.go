package main

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/m-lab/tcp-info/eventsocket"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/uuid-annotator/zipfile"
)

func TestMainSmokeTest(t *testing.T) {
	mainCtx, mainCancel = context.WithCancel(context.Background())
	zipfileFromGCS = func(_, _ string) zipfile.Provider {
		return zipfile.FromFile("testdata/GeoLite2City.zip")
	}
	defer func() {
		zipfileFromGCS = zipfile.FromGCS
	}()

	dir, err := ioutil.TempDir("", "TestMain")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)

	socketfilename := dir + "/eventsocket.sock"
	*eventsocket.Filename = socketfilename

	// Now start up a fake eventsocket.
	srv := eventsocket.New(*eventsocket.Filename)
	srv.Listen()
	go srv.Serve(mainCtx)

	// Cancel main after a tenth of a second.
	go func() {
		time.Sleep(100 * time.Millisecond)
		mainCancel()
	}()

	// Run main. Full coverage and no crash == success!
	main()
}
