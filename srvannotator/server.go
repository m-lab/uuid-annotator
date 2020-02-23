package srvannotator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/m-lab/go/rtx"

	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

// srvannotator is the central struct for this module.
type srvannotator struct {
	mut               sync.RWMutex
	localIPs          []net.IP
	backingDataSource rawfile.Provider
	hostname          string
	server            *annotator.ServerAnnotations
}

var ErrNotFound = errors.New("Not Found")

// New makes a new server Annotator using metadata from siteinfo JSON.
func New(ctx context.Context, hostname string, js rawfile.Provider, localIPs []net.IP) annotator.Annotator {
	g := &srvannotator{
		backingDataSource: js,
		hostname:          hostname,
		localIPs:          localIPs,
	}
	var err error
	g.server, err = g.load(ctx)
	rtx.Must(err, "Could not load annotation db")
	return g
}

// Annotate puts into geolocation data and ASN data into the passed-in annotations map.
func (g *srvannotator) Annotate(ID *inetdiag.SockID, annotations *annotator.Annotations) error {
	g.mut.RLock()
	defer g.mut.RUnlock()

	dir, err := annotator.FindDirection(ID, g.localIPs)
	if err != nil {
		return err
	}

	switch dir {
	case annotator.DstIsServer:
		err = g.annotate(ID.DstIP, &annotations.Server)
	case annotator.SrcIsServer:
		err = g.annotate(ID.SrcIP, &annotations.Server)
	}
	if err != nil {
		return fmt.Errorf("Could not annotate ip: %w", err)
	}
	return nil
}

func (g *srvannotator) annotate(src string, server *annotator.ServerAnnotations) error {
	ip := net.ParseIP(src)
	if ip == nil {
		return fmt.Errorf("failed to parse IP %q", src)
	}
	// TODO: verify that the given IP actually matches the current server IP block.
	*server = *g.server
	return nil
}

// load unconditionally loads datasets and returns them.
func (g *srvannotator) load(ctx context.Context) (*annotator.ServerAnnotations, error) {
	js, err := g.backingDataSource.Get(ctx)
	if err != nil {
		return nil, err
	}
	var s []annotator.ServerAnnotations
	err = json.Unmarshal(js, &s)
	if err != nil {
		return nil, err
	}
	f := strings.Split(g.hostname, ".")
	if len(f) < 2 {
		return nil, ErrNotFound
	}
	site := f[1]
	for i := range s {
		if s[i].Site == site {
			s[i].Machine = f[0]
			return &s[i], nil
		}
	}
	return nil, ErrNotFound
}
