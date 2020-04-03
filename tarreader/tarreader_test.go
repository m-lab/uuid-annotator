package tarreader

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/m-lab/go/rtx"
)

func mustRead(name string) []byte {
	b, err := ioutil.ReadFile(name)
	rtx.Must(err, "Failed to read %q", name)
	return b
}

func TestFromTarGZ(t *testing.T) {
	tests := []struct {
		name     string
		tgz      []byte
		filename string
		want     []byte
		wantErr  bool
	}{
		{
			name:     "success",
			tgz:      mustRead("../testdata/empty.tar.gz"),
			filename: "found.txt",
			want:     []byte{},
		},
		{
			name:     "file-not-found",
			tgz:      mustRead("../testdata/empty.tar.gz"),
			filename: "not-a-file",
			wantErr:  true,
		},
		{
			name:     "error",
			tgz:      []byte{},
			filename: "anything",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromTarGZ(tt.tgz, tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromTarGZ() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromTarGZ() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestFromGZ(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		want    []byte
		wantErr bool
	}{
		{
			name: "success",
			file: "../testdata/RouteViewIPv4.tiny.gz",
		},
		{
			name:    "error-corrupt-gz",
			file:    "../testdata/corrupt.gz",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			b, err := ioutil.ReadFile(tt.file)
			rtx.Must(err, "Failed to read test file: %q", tt.file)

			_, err = FromGZ(b)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromGZ() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
