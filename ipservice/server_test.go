package ipservice

import (
	"bytes"
	"errors"
	"log"
	"testing"

	"github.com/m-lab/go/rtx"
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
