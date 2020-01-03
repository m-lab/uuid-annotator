package handler_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"github.com/m-lab/tcp-info/inetdiag"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/eventsocket"
	"github.com/m-lab/uuid-annotator/handler"
)

func TestHandlerWithNoAnnotatorsE2E(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestHandlerWithAnnotations")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)

	// Start up a eventsocket server
	srv := eventsocket.New(dir + "/tcpevents.sock")
	rtx.Must(srv.Listen(), "Could not listen")
	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()
	go srv.Serve(srvCtx)

	// Give the server some time to start before we try to connect.
	time.Sleep(100 * time.Millisecond)

	// Create the handler to test
	hCtx, hCancel := context.WithCancel(context.Background())
	defer hCancel()
	h := handler.New(dir, 10, nil)
	go h.ProcessIncomingRequests(hCtx)
	go eventsocket.MustRun(hCtx, dir+"/tcpevents.sock", h)

	// Give the client some time to connect before we send events down the pipe.
	time.Sleep(100 * time.Millisecond)

	// Now send an event
	tstamp := time.Date(2009, 3, 18, 1, 2, 3, 0, time.UTC)
	srv.FlowCreated(
		tstamp,
		"THISISAUUID",
		inetdiag.SockID{
			SrcIP: "127.0.0.1",
			SPort: 123,
			DstIP: "10.0.0.1",
			DPort: 456,
		},
	)

	// Verify that the relevant file was created and is a JSON file in good standing.
	_, err = os.Stat(dir + "/2009/03/18/THISISAUUID.json")
	for err != nil {
		log.Println("Waiting for the file...")
		time.Sleep(time.Millisecond)
		_, err = os.Stat(dir + "/2009/03/18/THISISAUUID.json")
	}

	// File was created! Now let's check its contents...
	contents, err := ioutil.ReadFile(dir + "/2009/03/18/THISISAUUID.json")
	rtx.Must(err, "Could not read file")
	data := make(map[string]interface{})
	rtx.Must(json.Unmarshal(contents, &data), "Could not unmarshal")
	if data["UUID"].(string) != "THISISAUUID" {
		t.Error("Bad uuid:", data)
	}
	filetime, err := time.Parse(time.RFC3339, data["Timestamp"].(string))
	rtx.Must(err, "Could not parse time")
	if filetime != tstamp {
		t.Error("Unequal times:", filetime, tstamp)
	}
}
