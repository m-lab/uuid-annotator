package siteannotator

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"regexp"
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
	v4             net.IPNet
	v6             net.IPNet
}

// ErrHostnameNotFound is generated when the given hostname cannot be found in the
// downloaded siteinfo annotations.
var ErrHostnameNotFound = errors.New("Hostname Not Found")

// ErrInvalidHostname is generated when the given hostname appears to be invalid.
var ErrInvalidHostname = errors.New("Hostname Invalid")

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
	n := net.ParseIP(src)
	if g.v4.Contains(n) || g.v6.Contains(n) {
		// NOTE: this will not annotate private IP addrs.
		*server = *g.server
	}
}

type siteinfoAnnotation struct {
	Name    string
	Network struct {
		IPv4 string
		IPv6 string
	}
	Annotation annotator.ServerAnnotations
}

func parseCIDR(v4, v6 string) (net.IPNet, net.IPNet, error) {
	var v4ret, v6ret net.IPNet
	_, v4net, err := net.ParseCIDR(v4)
	if err != nil && v4 != "" {
		return v4ret, v6ret, err
	}
	if v4 != "" {
		v4ret = *v4net
	}
	_, v6net, err := net.ParseCIDR(v6)
	if err != nil && v6 != "" {
		return v4ret, v6ret, err
	}
	if v6 != "" {
		v6ret = *v6net
	}
	return v4ret, v6ret, nil
}

// load unconditionally loads siteinfo dataset and returns them.
func (g *siteAnnotator) load(ctx context.Context) (*annotator.ServerAnnotations, error) {
	js, err := g.siteinfoSource.Get(ctx)
	if err != nil {
		return nil, err
	}
	var s []siteinfoAnnotation
	var result annotator.ServerAnnotations
	err = json.Unmarshal(js, &s)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`^(mlab\d)[.-]([a-z]{3}\d[\dtc])`)
	matches := re.FindAllStringSubmatch(g.hostname, -1)
	if len(matches) != 1 || len(matches[0]) != 3 {
		return nil, ErrInvalidHostname
	}
	site := matches[0][2]
	for i := range s {
		if s[i].Name == site {
			result = s[i].Annotation // Copy out of array.
			result.Machine = matches[0][1]
			g.v4, g.v6, err = parseCIDR(s[i].Network.IPv4, s[i].Network.IPv6)
			if err != nil {
				return nil, err
			}
			return &result, nil
		}
	}
	return nil, ErrHostnameNotFound
}
