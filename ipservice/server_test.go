package ipservice

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/go/warnonerror"
	"github.com/m-lab/uuid-annotator/asnannotator"
	"github.com/m-lab/uuid-annotator/geoannotator"
)

func Test_logOnError(t *testing.T) {
	type args struct {
		err  error
		args []interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no log on nil err",
			args: args{
				err:  nil,
				args: []interface{}{"should not appear"},
			},
		},
		{
			name: "log on non-nil err",
			args: args{
				err:  errors.New("for testing"),
				args: []interface{}{"should", "appear"},
			},
			want: "should appear\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer log.SetOutput(log.Writer())
			defer log.SetFlags(log.Flags())
			b := &bytes.Buffer{}
			log.SetOutput(b)
			log.SetFlags(0)
			logOnError(tt.args.err, tt.args.args...)
			if len(tt.want) > 0 {
				line, err := b.ReadString('\n')
				rtx.Must(err, "Could not split buffer")
				if line != tt.want {
					t.Errorf("%q != %q", line, tt.want)
				}
			} else {
				by := b.Bytes()
				if len(by) != 0 {
					t.Error("No output was supposed to have occured, but we got", by)
				}
			}
		})
	}
}

func Test_logOnNil(t *testing.T) {
	object := struct{}{}
	type args struct {
		ptr  interface{}
		args []interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no log on non-nil",
			args: args{
				ptr:  nil,
				args: []interface{}{"should", "appear"},
			},
			want: "should appear\n",
		},
		{
			name: "log on non-nil err",
			args: args{
				ptr:  &object,
				args: []interface{}{"should not", "appear"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer log.SetOutput(log.Writer())
			defer log.SetFlags(log.Flags())
			b := &bytes.Buffer{}
			log.SetOutput(b)
			log.SetFlags(0)
			logOnNil(tt.args.ptr, tt.args.args...)
			if len(tt.want) > 0 {
				line, err := b.ReadString('\n')
				rtx.Must(err, "Could not split buffer")
				if line != tt.want {
					t.Errorf("%q != %q", line, tt.want)
				}
			} else {
				by := b.Bytes()
				if len(by) != 0 {
					t.Error("No output was supposed to have occured, but we got", by)
				}
			}
		})
	}
}

func ExampleServer_forTesting() {
	dir, err := ioutil.TempDir("", "ExampleFakeServerForTesting")
	rtx.Must(err, "could not create tempdir")
	defer os.RemoveAll(dir)

	*SocketFilename = dir + "/ipservice.sock"
	srv, err := NewServer(*SocketFilename, asnannotator.NewFake(), geoannotator.NewFake())
	rtx.Must(err, "Could not create server")
	defer warnonerror.Close(srv, "Could not stop the server")

	go srv.Serve()
	_, err = os.Stat(*SocketFilename)
	for err != nil {
		time.Sleep(time.Millisecond)
		_, err = os.Stat(*SocketFilename)
	}

	// Now the server exists, and clients can connect to it via:
	// c := NewClient(*SocketFilename)
	// and then you can call c.Annotate() and use the returned values.
}
