package zipfile

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/google-cloud-go-testing/storage/stiface"
	"github.com/m-lab/go/rtx"
)

func TestFileFromURLThenGet(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantGetErr bool
	}{
		{
			name: "Good file",
			url:  "file:../testdata/GeoLite2City.zip",
		},
		{
			name:       "Nonexistent file",
			url:        "file://this/file/does/not/exist",
			wantGetErr: true,
		},
		{
			name:       "File that exists but is not a zipfile",
			url:        "file:provider_test.go",
			wantGetErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			rtx.Must(err, "Could not parse URL")
			provider, err := FromURL(context.Background(), u)
			rtx.Must(err, "Could not create provider")
			_, err = provider.Get(context.Background())
			if (err != nil) != tt.wantGetErr {
				t.Errorf("Get() error = %v, wantGetErr %v", err, tt.wantGetErr)
			}
		})
	}
}

func TestFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Some of these endpoints do not exist, but since we never call .Get(),
		// the provider can still be created successfully.
		{
			name: "Good file",
			url:  "file:../testdata/GeoLite2City.zip",
		},
		{
			name: "Nonexistent file",
			url:  "file://this/file/does/not/exist",
		},
		{
			name: "GCS nonexistent file",
			url:  "gs://mlab-nonexistent-bucket/nonexistent-object.zip",
		},
		{
			name:    "Unsupported URL scheme",
			url:     "gopher://gopher.floodgap.com/1/world",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			rtx.Must(err, "Could not parse URL")
			_, err = FromURL(context.Background(), u)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromURL() error=%v (which should be or wrap ErrUnsupportedURLScheme=%v), wantErr=%v", err, ErrUnsupportedURLScheme, tt.wantErr)
				return
			}
			if err != nil {
				// The only errors returned from FromURL should derive from ErrUnsupportedURLScheme
				if !errors.Is(err, ErrUnsupportedURLScheme) {
					t.Errorf("Returned error %v should either be or wrap ErrUnsupportedURLScheme(%v)", err, ErrUnsupportedURLScheme)
				}
				return
			}
		})
	}
}

type fakeObjectHandle struct {
	stiface.ObjectHandle
	err   error
	attrs *storage.ObjectAttrs
}

func (foh *fakeObjectHandle) Attrs(ctx context.Context) (*storage.ObjectAttrs, error) {
	return foh.attrs, foh.err
}

type fakeBucketHandle struct {
	stiface.BucketHandle
	oh stiface.ObjectHandle
}

func (fbh *fakeBucketHandle) Object(string) stiface.ObjectHandle {
	return fbh.oh
}

type fakeClient struct {
	stiface.Client
	bh stiface.BucketHandle
}

func (fc *fakeClient) Bucket(name string) stiface.BucketHandle { return fc.bh }

func TestGCSGet(t *testing.T) {
	client := &fakeClient{
		bh: &fakeBucketHandle{
			oh: nil,
		},
	}

	gcp := gcsProvider{
		bucket:   "testb",
		filename: "testf.zip",
		client:   client,
	}

	_, err := gcp.Get(context.Background())
	rtx.Must(err, "Could not get")
}
