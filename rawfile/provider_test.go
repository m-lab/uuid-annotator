package rawfile

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/googleapis/google-cloud-go-testing/storage/stiface"
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

func TestFromGSURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *gcsProvider
		wantErr bool
	}{
		// Some of these endpoints do not exist, but since we never call .Get(),
		// the provider can still be created successfully.
		{
			name: "Good file",
			url:  "gs://downloader-mlab-sandbox/Maxmind/current/GeoLite2-City-CSV.zip",
			want: &gcsProvider{
				bucket:   "downloader-mlab-sandbox",
				filename: "Maxmind/current/GeoLite2-City-CSV.zip",
			},
		},
		{
			name: "GCS nonexistent file",
			url:  "gs://mlab-nonexistent-bucket/nonexistent-object.zip",
			want: &gcsProvider{
				bucket:   "mlab-nonexistent-bucket",
				filename: "nonexistent-object.zip",
			},
		},
		{
			name:    "GCS no file",
			url:     "gs://mlab-nonexistent-bucket/",
			wantErr: true,
		},
		{
			name:    "GCS no file no slash",
			url:     "gs://mlab-nonexistent-bucket",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			rtx.Must(err, "Could not parse URL")
			p, err := FromURL(context.Background(), u)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromURL() error=%v (which should be or wrap ErrUnsupportedURLScheme=%v), wantErr=%v", err, ErrUnsupportedURLScheme, tt.wantErr)
				return
			}
			if err != nil {
				// Don't verify the output in the error case. The API makes no promises on error cases.
				return
			}
			gcsp := p.(*gcsProvider)
			if gcsp.bucket != tt.want.bucket || gcsp.filename != tt.want.filename {
				t.Errorf(
					"Bucket and filename should be (%q,%q), but are (%q,%q)",
					tt.want.bucket, tt.want.filename, gcsp.bucket, gcsp.filename)
			}
		})
	}
}

type stifaceReaderThatsJustAnIOReader struct {
	stiface.Reader
	r io.Reader
}

func (s *stifaceReaderThatsJustAnIOReader) Read(p []byte) (int, error) {
	return s.r.Read(p)
}

type readerWhereReadFails struct {
	stiface.Reader
}

func (*readerWhereReadFails) Read(p []byte) (int, error) {
	return 0, errors.New("This reader fails for test purposes")
}

type fakeObjectHandle struct {
	stiface.ObjectHandle
	attrErr   error
	attrs     *storage.ObjectAttrs
	readerErr error
	reader    stiface.Reader
}

func (foh *fakeObjectHandle) Attrs(ctx context.Context) (*storage.ObjectAttrs, error) {
	return foh.attrs, foh.attrErr
}

func (foh *fakeObjectHandle) NewReader(ctx context.Context) (stiface.Reader, error) {
	return foh.reader, foh.readerErr
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

func readerForZipfileOnDisk() io.Reader {
	r, err := os.Open("../testdata/GeoLite2City.zip")
	rtx.Must(err, "Could not open test data")
	return r
}

func Test_gcsProvider_Get(t *testing.T) {
	zipReaderForCaching := []byte{}

	//readerForNonZipfileOnDisk, err := os.Open("provider_test.go")
	//rtx.Must(err, "Could not open this test file")

	type fields struct {
		bucket       string
		filename     string
		client       stiface.Client
		md5          []byte
		cachedReader []byte
	}
	tests := []struct {
		name       string
		fields     fields
		wantNonNil bool
		wantErr    bool
	}{
		{
			name: "Can't get Attrs",
			fields: fields{
				client: &fakeClient{
					bh: &fakeBucketHandle{
						oh: &fakeObjectHandle{
							attrErr: errors.New("Error for testing"),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Test caching (hashes should match and reader error should not be returned)",
			fields: fields{
				client: &fakeClient{
					bh: &fakeBucketHandle{
						oh: &fakeObjectHandle{
							attrs: &storage.ObjectAttrs{
								MD5: []byte("a hash"),
							},
							readerErr: errors.New("This should not happen"),
						},
					},
				},
				cachedReader: zipReaderForCaching,
				md5:          []byte("a hash"),
			},
			wantNonNil: true,
		},
		{
			name: "NewReader error is handled",
			fields: fields{
				client: &fakeClient{
					bh: &fakeBucketHandle{
						oh: &fakeObjectHandle{
							attrs: &storage.ObjectAttrs{
								MD5: []byte("a hash"),
							},
							readerErr: errors.New("Can't make reader"),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ReadAll error is handled",
			fields: fields{
				client: &fakeClient{
					bh: &fakeBucketHandle{
						oh: &fakeObjectHandle{
							attrs: &storage.ObjectAttrs{
								MD5: []byte("a hash"),
							},
							reader: &readerWhereReadFails{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Read successfully from fake GCS",
			fields: fields{
				client: &fakeClient{
					bh: &fakeBucketHandle{
						oh: &fakeObjectHandle{
							attrs: &storage.ObjectAttrs{
								MD5: []byte("a hash"),
							},
							reader: &stifaceReaderThatsJustAnIOReader{
								r: readerForZipfileOnDisk(),
							},
						},
					},
				},
			},
			wantNonNil: true,
		},
		{
			name: "Read successfully from fake GCS with cached data",
			fields: fields{
				client: &fakeClient{
					bh: &fakeBucketHandle{
						oh: &fakeObjectHandle{
							attrs: &storage.ObjectAttrs{
								MD5: []byte("a hash"),
							},
							reader: &stifaceReaderThatsJustAnIOReader{
								r: readerForZipfileOnDisk(),
							},
						},
					},
				},
				cachedReader: zipReaderForCaching,
				md5:          []byte("a different hash"),
			},
			wantNonNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &gcsProvider{
				bucket:       tt.fields.bucket,
				filename:     tt.fields.filename,
				client:       tt.fields.client,
				md5:          tt.fields.md5,
				cachedReader: tt.fields.cachedReader,
			}
			got, err := g.Get(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("gcsProvider.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantNonNil != (got != nil) {
				t.Errorf("gcsProvider.Get() = %v, wantNonNil=%v", got, tt.wantNonNil)
			}
		})
	}
}
