package rawfile

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"net/url"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/googleapis/google-cloud-go-testing/storage/stiface"
	"github.com/m-lab/uuid-annotator/metrics"
)

// Errors that might be returned outside the package.
var (
	ErrUnsupportedURLScheme = errors.New("Unsupported URL scheme")
)

// Provider is the interface implemented by everything that can return raw files.
type Provider interface {
	// Get returns the raw file []byte read from the latest copy of the provider
	// URL. It may be called multiple times. Caching is left up to the individual
	// Provider implementation.
	Get(ctx context.Context) ([]byte, error)
}

// gcsProvider gets zip files from Google Cloud Storage.
type gcsProvider struct {
	bucket, filename string
	client           stiface.Client
	md5              []byte
	cachedReader     []byte
}

func (g *gcsProvider) Get(ctx context.Context) ([]byte, error) {
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
		g.cachedReader = data
		if g.md5 != nil {
			metrics.GCSFilesLoaded.WithLabelValues(hex.EncodeToString(g.md5)).Set(0)
		}
		g.md5 = oa.MD5
		metrics.GCSFilesLoaded.WithLabelValues(hex.EncodeToString(g.md5)).Set(1)
	}
	return g.cachedReader, nil
}

// fileProvider gets files from the local disk.
type fileProvider struct {
	filename string
}

func (f *fileProvider) Get(ctx context.Context) ([]byte, error) {
	b, err := ioutil.ReadFile(f.filename)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// FromURL returns a new rawfile.Provider based on the passed-in URL. Supported
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
		filename := strings.TrimPrefix(u.Path, "/")
		if len(filename) == 0 {
			return nil, errors.New("Bad GS url, no filename detected")
		}
		return &gcsProvider{
			client:   stiface.AdaptClient(client),
			bucket:   u.Host,
			filename: filename,
		}, err
	case "file":
		return &fileProvider{
			filename: u.Opaque,
		}, nil
	default:
		return nil, ErrUnsupportedURLScheme
	}
}
