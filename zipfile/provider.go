package zipfile

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/ioutil"
)

type Provider interface {
	Get() (*zip.Reader, error)
}

type gcsProvider struct {
	bucket, filename string
}

func (g *gcsProvider) Get() (*zip.Reader, error) {
	return nil, errors.New("unimplemented")
}

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

func FromFile(filename string) Provider {
	return &fileProvider{
		filename: filename,
	}
}
