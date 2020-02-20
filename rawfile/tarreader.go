package rawfile

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"strings"
)

// ErrFileNotFound is returned when the given name is not found in the archive.
var ErrFileNotFound = errors.New("file not found")

// FromGZ decompresses the given data.
func FromGZ(gz []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(gr)
}

// ReadFromTar reads the named file from the compressed, tar archive in tgz.
func ReadFromTar(tgz []byte, name string) ([]byte, error) {
	tr, err := newTarReader(bytes.NewReader(tgz))
	if err != nil {
		return nil, err
	}
	defer tr.Close()
	d, err := tr.readFile(name)
	return d, err
}

// Helper functions for reading a target file from a .tar.gz archive.
type tarReader struct {
	*tar.Reader
	io.Closer
}

func newTarReader(rdr io.Reader) (*tarReader, error) {
	gr, err := gzip.NewReader(rdr)
	if err != nil {
		return nil, err
	}
	t := &tarReader{
		Reader: tar.NewReader(gr),
		Closer: gr,
	}
	return t, nil
}

// NOTE: readFile is not guaranteed to work on more than one file.
func (tr *tarReader) readFile(name string) ([]byte, error) {
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, ErrFileNotFound
		}
		if strings.HasSuffix(h.Name, name) {
			return ioutil.ReadAll(tr)
		}
	}
}
