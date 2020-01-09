package zipfile

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/ioutil"
)

// Provider is the interface implemented by everything that can return a
// zip.Reader.
type Provider interface {
	// Get returns a zip.Reader pointer based on the latest copy of the data the
	// provider refers to. It may be called multiple times, and caching is left
	// up to the individual Provider implementation.
	Get() (*zip.Reader, error)
}

type gcsProvider struct {
	bucket, filename string
}

func (g *gcsProvider) Get() (*zip.Reader, error) {
	return nil, errors.New("unimplemented")
}

// FromGCS returns a zipfile.Provider based on a file in Google Cloud Storage.
func FromGCS(bucket, filename string) Provider {
	return &gcsProvider{
		bucket:   bucket,
		filename: filename,
	}
}

type fileProvider struct {
	filename string
}

func (f *fileProvider) Get() (*zip.Reader, error) {
	b, err := ioutil.ReadFile(f.filename)
	if err != nil {
		return nil, err
	}
	return zip.NewReader(bytes.NewReader(b), int64(len(b)))
}

// FromFile returns a zipfile.Provider based on a local filename.
func FromFile(filename string) Provider {
	return &fileProvider{
		filename: filename,
	}
}
