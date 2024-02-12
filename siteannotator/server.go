package siteannotator

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/rtx"

	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
)

// siteAnnotator is the central struct for this module.
type siteAnnotator struct {
	m              sync.RWMutex
	localIPs       []net.IP
	siteinfoSource content.Provider
	hostname       string
	server         *annotator.ServerAnnotations
	v4             net.IPNet
	v6             net.IPNet
}

// ErrHostnameNotFound is generated when the given hostname cannot be found in the
// downloaded siteinfo annotations.
var ErrHostnameNotFound = errors.New("hostname not found")

// New makes a new server Annotator using metadata from siteinfo JSON.
func New(ctx context.Context, hostname string, js content.Provider, localIPs []net.IP) (annotator.Annotator, []net.IP) {
	g := &siteAnnotator{
		siteinfoSource: js,
		hostname:       hostname,
	}
	var err error
	g.server, localIPs, err = g.load(ctx, localIPs)
	g.localIPs = localIPs
	rtx.Must(err, "Could not load annotation db")
	return g, localIPs
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

// NOTE: in a cloud environment, the local IP and public IPs will be in
// different netblocks. The siteinfo configuration only knows about the public
// IP address. Rather than exclude annotations for these cases, `annotate()`
// uses the v4 config (if present) for IPv4 src addresses, and the v6 config (if
// present) for IPv6 src addresses.
func (g *siteAnnotator) annotate(src string, server *annotator.ServerAnnotations) {
	n := net.ParseIP(src)
	switch {
	case n.To4() != nil && g.v4.IP != nil:
		// If src and config are IPv4 addresses.
		*server = *g.server
		(*server).Network.CIDR = g.v4.String()
	case n.To4() == nil && g.v6.IP != nil:
		// If src and config are IPv6 addresses.
		*server = *g.server
		(*server).Network.CIDR = g.v6.String()
	}
}

type siteinfoAnnotation struct {
	Annotation annotator.ServerAnnotations
	Network    struct {
		IPv4 string
		IPv6 string
	}
	Type string
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
func (g *siteAnnotator) load(ctx context.Context, localIPs []net.IP) (*annotator.ServerAnnotations, []net.IP, error) {
	js, err := g.siteinfoSource.Get(ctx)
	if err != nil {
		return nil, nil, err
	}
	var s map[string]siteinfoAnnotation
	err = json.Unmarshal(js, &s)
	if err != nil {
		return nil, nil, err
	}
	if v, ok := s[g.hostname]; ok {
		g.v4, g.v6, err = parseCIDR(v.Network.IPv4, v.Network.IPv6)
		if err != nil {
			return nil, nil, err
		}
		// If this is a virtual site, append the site's public IP address to
		// localIPs. The public address of the load balancer is not known on any
		// interface on the machine. Without adding it to localIPs,
		// uuid-annotator will fail to recognize its own public address in
		// either the Src or Dest fields of incoming tcp-info events, and will
		// fail to annotate anything.
		if v.Type == "virtual" {
			localIPs = append(localIPs, g.v4.IP, g.v6.IP)
		}

		return &v.Annotation, localIPs, nil
	}
	return nil, nil, ErrHostnameNotFound
}
