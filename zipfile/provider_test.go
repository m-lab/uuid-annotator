package zipfile

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/m-lab/go/rtx"
)

func TestFromURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantErr    bool
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
			provider, err := FromURL(context.Background(), u)
			// The only errors returned from FromURL should derive from ErrUnsupportedURLScheme
			if (err != nil) != tt.wantErr {
				t.Errorf("FromURL() error = %v (which should be or wrap ErrUnsupportedURLScheme=%v), wantErr %v", err, ErrUnsupportedURLScheme, tt.wantErr)
				return
			}
			// Only test .Get() if we were able to make the provider successfully
			if err != nil {
				if !errors.Is(err, ErrUnsupportedURLScheme) {
					t.Errorf("Returned error %v should either be or wrap ErrUnsupportedURLScheme(%v)", err, ErrUnsupportedURLScheme)
				}
				return
			}
			_, err = provider.Get()
			if (err != nil) != tt.wantGetErr {
				t.Errorf("Get() error = %v, wantGetErr %v", err, tt.wantGetErr)
			}
		})
	}
}
