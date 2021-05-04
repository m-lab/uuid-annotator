package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"

	"github.com/m-lab/tcp-info/inetdiag"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/tcp-info/eventsocket"
	"github.com/m-lab/uuid-annotator/annotator"
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

	// Create the handler to test and connect it to the server.
	hCtx, hCancel := context.WithCancel(context.Background())
	defer hCancel()
	h := New(dir, 1, nil)
	go eventsocket.MustRun(hCtx, dir+"/tcpevents.sock", h)

	// Give the client some time to connect before we send events down the pipe.
	time.Sleep(10 * time.Millisecond)

	// Now send two events. The second should be dropped, because we are not
	// processing events and have specified a buffer size of 1.
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
	srv.FlowCreated(
		tstamp,
		"THISISAUUID2",
		inetdiag.SockID{
			SrcIP: "127.0.0.1",
			SPort: 123,
			DstIP: "10.0.0.1",
			DPort: 456,
		},
	)

	// Make sure the event is fully processed before we start the processing goroutine.
	time.Sleep(time.Millisecond)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		h.ProcessIncomingRequests(hCtx)
		wg.Done()
	}()

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

	// Verify that calling close does not cause a crash.
	h.Close(hCtx, time.Now(), "")

	// Cancel the handler's context and then wait to verify that the
	// cancellation of the context causes ProcessIncomingRequests to terminate.
	hCancel()
	wg.Wait()
}

type badannotator struct{}

func (badannotator) Annotate(ID *inetdiag.SockID, annotations *annotator.Annotations) error {
	return errors.New("an error for testing")
}

func setFs(newfs afero.Fs) (cleanup func()) {
	oldfs := fs
	cleanup = func() {
		fs = oldfs
		fsutil = &afero.Afero{Fs: oldfs}
	}
	fs = newfs
	fsutil = &afero.Afero{Fs: newfs}
	return cleanup
}

type errFS struct {
	callback func()
}

var errForTesting = errors.New("Error for testing")

func (e *errFS) Create(name string) (afero.File, error)       { e.callback(); return nil, errForTesting }
func (e *errFS) Mkdir(name string, perm os.FileMode) error    { e.callback(); return errForTesting }
func (e *errFS) MkdirAll(path string, perm os.FileMode) error { e.callback(); return errForTesting }
func (e *errFS) Open(name string) (afero.File, error)         { e.callback(); return nil, errForTesting }
func (e *errFS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	e.callback()
	return nil, errForTesting
}
func (e *errFS) Remove(name string) error                  { e.callback(); return errForTesting }
func (e *errFS) RemoveAll(path string) error               { e.callback(); return errForTesting }
func (e *errFS) Rename(oldname, newname string) error      { e.callback(); return errForTesting }
func (e *errFS) Stat(name string) (os.FileInfo, error)     { e.callback(); return nil, errForTesting }
func (e *errFS) Name() string                              { return "" }
func (e *errFS) Chmod(name string, mode os.FileMode) error { e.callback(); return errForTesting }
func (e *errFS) Chown(name string, uid, gid int) error     { e.callback(); return errForTesting }
func (e *errFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	e.callback()
	return errForTesting
}

func TestErrorCases(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer setFs(&errFS{cancel})()
	defer cancel()

	// A handler with a bad directory and a bad annotator.
	h := New("/../thisisimpossible/", 1, []annotator.Annotator{badannotator{}})
	h.Open(ctx, time.Now(), "UUID_IS_THIS", &inetdiag.SockID{})
	h.ProcessIncomingRequests(ctx)
	// No crash, successful termination and full coverage == success
}
