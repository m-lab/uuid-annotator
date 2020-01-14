package zipfile

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/url"

	"cloud.google.com/go/storage"
	"github.com/googleapis/google-cloud-go-testing/storage/stiface"
	"github.com/m-lab/uuid-annotator/metrics"
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
	Get(ctx context.Context) (*zip.Reader, error)
}

// gcsProvider gets zip files from Google Cloud Storage.
type gcsProvider struct {
	bucket, filename string
	client           stiface.Client
	md5              []byte
	cachedReader     *zip.Reader
}

func (g *gcsProvider) Get(ctx context.Context) (*zip.Reader, error) {
	o := g.client.Bucket(g.bucket).Object(g.filename)
	oa, err := o.Attrs(ctx)
	if err != nil {
		return nil, err
	}
	if g.cachedReader == nil || g.md5 == nil || !bytes.Equal(g.md5, oa.MD5) {
		// Reload data only if the object changed or the data was never loaded in the first place.
		r, err := o.NewReader(ctx)
		if err != nil {
			return nil, err
		}
		var data []byte
		data, err = ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		g.cachedReader = zr
		if g.md5 != nil {
			metrics.GCSFilesLoaded.WithLabelValues(string(g.md5)).Set(0)
		}
		g.md5 = oa.MD5
		metrics.GCSFilesLoaded.WithLabelValues(string(g.md5)).Set(1)
	}
	return g.cachedReader, nil
}

// fileProvider gets zipfiles from the local disk.
type fileProvider struct {
	filename string
}

func (f *fileProvider) Get(ctx context.Context) (*zip.Reader, error) {
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
func FromURL(ctx context.Context, u *url.URL) (Provider, error) {
	switch u.Scheme {
	case "gs":
		client, err := storage.NewClient(ctx)
		return &gcsProvider{
			client:   stiface.AdaptClient(client),
			bucket:   u.Host,
			filename: u.Path,
		}, err
	case "file":
		return &fileProvider{
			filename: u.Opaque,
		}, nil
	default:
		return nil, ErrUnsupportedURLScheme
	}
}
