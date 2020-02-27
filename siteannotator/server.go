package siteannotator

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"

	"github.com/m-lab/go/rtx"

	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/rawfile"
)

// siteAnnotator is the central struct for this module.
type siteAnnotator struct {
	m              sync.RWMutex
	localIPs       []net.IP
	siteinfoSource rawfile.Provider
	hostname       string
	server         *annotator.ServerAnnotations
}

// ErrNotFound is generated when the given hostname cannot be found in the
// downloaded siteinfo annotations.
var ErrNotFound = errors.New("Not Found")

// New makes a new server Annotator using metadata from siteinfo JSON.
func New(ctx context.Context, hostname string, js rawfile.Provider, localIPs []net.IP) annotator.Annotator {
	g := &siteAnnotator{
		siteinfoSource: js,
		hostname:       hostname,
		localIPs:       localIPs,
	}
	var err error
	g.server, err = g.load(ctx)
	rtx.Must(err, "Could not load annotation db")
	return g
}

// Annotate assigns the server geolocation and ASN metadata.
func (g *siteAnnotator) Annotate(ID *inetdiag.SockID, annotations *annotator.Annotations) error {
	g.m.RLock()
	defer g.m.RUnlock()

	dir, err := annotator.FindDirection(ID, g.localIPs)
	if err != nil {
		return err
	}

	switch dir {
	case annotator.DstIsServer:
		g.annotate(ID.DstIP, &annotations.Server)
	case annotator.SrcIsServer:
		g.annotate(ID.SrcIP, &annotations.Server)
	}
	return nil
}

func (g *siteAnnotator) annotate(src string, server *annotator.ServerAnnotations) {
	// TODO: verify that the given IP actually matches the public server IP block.
	*server = *g.server
}

// load unconditionally loads siteinfo dataset and returns them.
func (g *siteAnnotator) load(ctx context.Context) (*annotator.ServerAnnotations, error) {
	js, err := g.siteinfoSource.Get(ctx)
	if err != nil {
		return nil, err
	}
	var s []annotator.ServerAnnotations
	var result annotator.ServerAnnotations
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
			result = s[i] // Copy out of array.
			return &result, nil
		}
	}
	return nil, ErrNotFound
}
