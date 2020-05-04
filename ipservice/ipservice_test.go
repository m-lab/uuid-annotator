package ipservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/asnannotator"
	"github.com/m-lab/uuid-annotator/geoannotator"
)

var (
	asn asnannotator.ASNAnnotator
	geo geoannotator.GeoAnnotator
)

func init() {
	ctx := context.Background()

	// Set up ASN annotator.
	u4, err := url.Parse("file:../testdata/RouteViewIPv4.pfx2as.gz")
	rtx.Must(err, "Could not parse URL")
	local4Rawfile, err := content.FromURL(context.Background(), u4)
	rtx.Must(err, "Could not create content.Provider")
	u6, err := url.Parse("file:../testdata/RouteViewIPv6.pfx2as.gz")
	rtx.Must(err, "Could not parse URL")
	local6Rawfile, err := content.FromURL(context.Background(), u6)
	rtx.Must(err, "Could not create content.Provider")
	as, err := url.Parse("file:../data/asnames.ipinfo.csv")
	rtx.Must(err, "Could not parse URL")
	localASNamesfile, err := content.FromURL(context.Background(), as)
	rtx.Must(err, "Could not create content.Provider")

	localIPs := []net.IP{
		net.ParseIP("9.0.0.9"),
		net.ParseIP("2002::1"),
	}
	asn = asnannotator.New(ctx, local4Rawfile, local6Rawfile, localASNamesfile, localIPs)

	// Set up geo annotator.
	u, err := url.Parse("file:../testdata/fake.tar.gz")
	rtx.Must(err, "Could not parse URL")
	localRawfile, err := content.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create content.Provider")
	geo = geoannotator.New(ctx, localRawfile, localIPs)
}

func TestServerAndClientE2E(t *testing.T) {
	d, err := ioutil.TempDir("", "TestServerAndClientE2E")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(d)

	sock := d + "/annotator.sock"
	srv, err := NewServer(sock, asn, geo)
	rtx.Must(err, "Could not create server")

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		rtx.Must(srv.Serve(), "Could not serve the annotator")
		wg.Done()
	}()

	c := NewClient(sock)
	ctx := context.Background()

	tests := []struct {
		name    string
		ips     []string
		want    map[string]*annotator.ClientAnnotations
		wantErr bool
	}{
		{
			name:    "Nil ips",
			ips:     nil,
			wantErr: true,
		},
		{
			name:    "Bad ips",
			ips:     []string{"this is not an ip address"},
			wantErr: true,
		},
		{
			name: "Localhost-v4",
			ips:  []string{"127.0.0.1"},
			want: map[string]*annotator.ClientAnnotations{
				"127.0.0.1": {
					Network: &annotator.Network{
						Missing: true,
					},
					Geo: &annotator.Geolocation{
						Missing: true,
					},
				},
			},
		},
		{
			name: "Localhost-v6",
			ips:  []string{"::1"},
			want: map[string]*annotator.ClientAnnotations{
				"::1": {
					Network: &annotator.Network{
						Missing: true,
					},
					Geo: &annotator.Geolocation{
						Missing: true,
					},
				},
			},
		},
		{
			name: "IP that has everything",
			ips:  []string{"2.125.160.216"},
			want: map[string]*annotator.ClientAnnotations{
				"2.125.160.216": {
					Network: &annotator.Network{
						CIDR:     "2.120.0.0/13",
						ASNumber: 5607,
						ASName:   "Sky UK Limited",
						Systems: []annotator.System{
							{ASNs: []uint32{5607}},
						},
					},
					Geo: &annotator.Geolocation{
						ContinentCode:       "EU",
						CountryCode:         "GB",
						CountryName:         "United Kingdom",
						Subdivision1ISOCode: "ENG",
						Subdivision1Name:    "England",
						Subdivision2ISOCode: "WBK",
						Subdivision2Name:    "West Berkshire",
						City:                "Boxford",
						PostalCode:          "OX1",
						Latitude:            51.75,
						Longitude:           -1.25,
						AccuracyRadiusKm:    100,
					},
				},
			},
		},
		{
			name: "Multiple IPs",
			ips:  []string{"2.125.160.216", "127.0.0.1"},
			want: map[string]*annotator.ClientAnnotations{
				"2.125.160.216": {
					Network: &annotator.Network{
						CIDR:     "2.120.0.0/13",
						ASNumber: 5607,
						ASName:   "Sky UK Limited",
						Systems: []annotator.System{
							{ASNs: []uint32{5607}},
						},
					},
					Geo: &annotator.Geolocation{
						ContinentCode:       "EU",
						CountryCode:         "GB",
						CountryName:         "United Kingdom",
						Subdivision1ISOCode: "ENG",
						Subdivision1Name:    "England",
						Subdivision2ISOCode: "WBK",
						Subdivision2Name:    "West Berkshire",
						City:                "Boxford",
						PostalCode:          "OX1",
						Latitude:            51.75,
						Longitude:           -1.25,
						AccuracyRadiusKm:    100,
					},
				},
				"127.0.0.1": {
					Network: &annotator.Network{
						Missing: true,
					},
					Geo: &annotator.Geolocation{
						Missing: true,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Annotate(ctx, tt.ips)
			if (err != nil) != tt.wantErr {
				t.Errorf("Annotate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			gotStr, _ := json.Marshal(got)
			wantStr, _ := json.Marshal(tt.want)
			if string(gotStr) != string(wantStr) {
				t.Errorf("Annotate() = %q, want %q", string(gotStr), string(wantStr))
			}
		})
	}

	srv.Close()
	wg.Wait()
}

func TestNewServerWithExistingFile(t *testing.T) {
	// Server creation should succeed even when the socket file already exists.
	// So make a file and use its name to start the server, hopefully without error.
	f, err := ioutil.TempFile("", "TextNewServerError")
	rtx.Must(err, "Could not create tempfile")
	defer os.Remove(f.Name())

	_, err = NewServer(f.Name(), asn, geo)
	rtx.Must(err, "Should not have had an error when starting NewServer wth an existing file")
}

func TestNewClient(t *testing.T) {
	d, err := ioutil.TempDir("", "TestNewClient")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(d)

	sock := d + "/annotator.sock"

	srv, err := NewServer(sock, asn, geo)
	rtx.Must(err, "Could not create server")
	go srv.Serve()
	defer srv.Close()

	c := NewClient(sock)
	ctx := context.Background()
	_, err = c.Annotate(ctx, []string{"127.0.0.1"})
	rtx.Must(err, "Could not annotate localhost")
}

func TestNewClientWithNoServer(t *testing.T) {
	d, err := ioutil.TempDir("", "TestNewClientWithNoServer")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(d)

	sock := d + "/annotator.sock"

	c := NewClient(sock)
	ctx := context.Background()
	_, err = c.Annotate(ctx, []string{"127.0.0.1"})
	if err == nil {
		t.Error("We should have gotten an error from the http client")
	}
}

func TestNewClient404(t *testing.T) {
	d, err := ioutil.TempDir("", "TestNewClient404")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(d)

	sock := d + "/annotator.sock"

	srv, err := NewServer(sock, asn, geo)
	rtx.Must(err, "Could not create server")
	srv.(*server).srv.Handler = nil
	go srv.Serve()
	defer srv.Close()

	c := NewClient(sock)
	ctx := context.Background()
	_, err = c.Annotate(ctx, []string{"127.0.0.1"})
	if err == nil {
		t.Error("We should have gotten an error from the http client")
	}
}

type unreadableBody struct{}

func (ub *unreadableBody) Read(p []byte) (int, error) {
	return 0, errors.New("Error for testing")
}

func (ub *unreadableBody) Close() error {
	return nil
}

type getterWithSpecificBody struct {
	body io.ReadCloser
}

func (g *getterWithSpecificBody) Get(url string) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: 200,
		Body:       g.body,
	}
	return resp, nil
}

func TestNewClientWithUnreadableBody(t *testing.T) {
	c := NewClient("this does not exist and that is ok")
	c.(*client).httpc = &getterWithSpecificBody{&unreadableBody{}}
	ctx := context.Background()
	_, err := c.Annotate(ctx, []string{"127.0.0.1"})
	if err == nil {
		t.Error("We should have gotten an error from the http client")
	}
}

func TestNewClientWithUnmarshalableBody(t *testing.T) {
	c := NewClient("this does not exist and that is ok")
	c.(*client).httpc = &getterWithSpecificBody{ioutil.NopCloser(bytes.NewReader([]byte("]}not json")))}
	ctx := context.Background()
	_, err := c.Annotate(ctx, []string{"127.0.0.1"})
	if err == nil {
		t.Error("We should have gotten an error from the http client")
	}
}

type badResp struct{}

func (b *badResp) Header() http.Header        { return make(http.Header) }
func (b *badResp) WriteHeader(statusCode int) {}
func (b *badResp) Write([]byte) (int, error) {
	return 0, errors.New("Error for testing")
}

func TestServerWriteError(t *testing.T) {
	h := handler{
		asn: asn,
		geo: geo,
	}
	req, err := http.NewRequest("GET", "http://unix/ip?ip=127.0.0.1", &bytes.Buffer{})
	rtx.Must(err, "Could not create error")
	h.ServeHTTP(&badResp{}, req)
	// No crash and 100% coverage == success!
}
