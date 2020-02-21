// Package annotator provides structs and interfaces used throughout the
// program.
package annotator

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/tcp-info/inetdiag"
)

var (
	// ErrNoAnnotation is for when we could not perform an annotation for some
	// reason. It is intended to convey that the annotation system is still
	// functioning fine, but one or more of the annotations you asked for could
	// not be performed.
	ErrNoAnnotation = errors.New("Could not annotate IP address")
)

// Annotations contains the standard columns we would like to add as annotations for every UUID.
type Annotations struct {
	UUID      string
	Timestamp time.Time
	Server    api.Annotations `bigquery:"server"` // Use Standard Top-Level Column names.
	Client    api.Annotations `bigquery:"client"` // Use Standard Top-Level Column names.
}

// Annotator is the interface that all systems that want to add metadata should implement.
type Annotator interface {
	Annotate(ID *inetdiag.SockID, annotations *Annotations) error
}

// direction gives us an enum to keep track of which end of the connection is
// the server, because we are informed of connections without regard to which
// end is the local server.
type direction int

const (
	unknown direction = iota
	s2c
	c2s
)

// Direction determines whether the IPs in the given ID map to the server or client annotations.
// Direction returns the corresponding "src" and "dst" annotation fields from the given annotator.Annotations.
func Direction(ID *inetdiag.SockID, localIPs []net.IP, ann *Annotations) (*api.Annotations, *api.Annotations, error) {
	dir := unknown
	for _, local := range localIPs {
		if ID.SrcIP == local.String() {
			dir = s2c
			break
		}
		if ID.DstIP == local.String() {
			dir = c2s
			break
		}
	}

	var src, dst *api.Annotations
	switch dir {
	case s2c:
		src = &ann.Server
		dst = &ann.Client
	case c2s:
		src = &ann.Client
		dst = &ann.Server
	case unknown:
		return nil, nil, fmt.Errorf("Can't annotate connection: Unknown direction for %+v", ID)
	}

	return src, dst, nil
}
