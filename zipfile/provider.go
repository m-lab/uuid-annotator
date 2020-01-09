package zipfile

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/ioutil"
	"net/url"
)

// Errors that might be returned outside the package.
var (
	ErrUnsupportedURLScheme = errors.New("Unsupported URL scheme")
)

// Provider is the interface implemented by everything that can return a
// zip.Reader.
type Provider interface {
	// Get returns a zip.Reader pointer based on the latest copy of the data the
	// provider refers to. It may be called multiple times, and caching is left
	// up to the individual Provider implementation.
	Get() (*zip.Reader, error)
}

// gcsProvider gets zip files from Google Cloud Storage.
type gcsProvider struct {
	bucket, filename string
}

func (g *gcsProvider) Get() (*zip.Reader, error) {
	return nil, errors.New("unimplemented")
}

// fileProvider gets zipfiles from the local disk.
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

// FromURL returns a new zipfile.Provider based on the passed-in URL. Supported
// URL schemes are currently: gs://bucket/filename and file:localpath . Whether
// the path contained in the URL is valid isn't known until the Get() method of
// the returned Provider is called. Unsupported URL schemes cause this to return
// ErrUnsupportedURLScheme.
//
// Users interested in having the daemon download the data directly from MaxMind
// should implement an https case in the below handler. M-Lab doesn't need that
// case because we cache MaxMind's data to reduce load on their servers and to
// eliminate a runtime dependency on a third party service.
func FromURL(u *url.URL) (Provider, error) {
	switch u.Scheme {
	case "gs":
		return &gcsProvider{
			bucket:   u.Host,
			filename: u.Path,
		}, nil
	case "file":
		return &fileProvider{
			filename: u.Opaque,
		}, nil
	default:
		return nil, ErrUnsupportedURLScheme
	}
}
