// Package annotator provides structs and interfaces used throughout the
// program.
package annotator

import (
	"errors"
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
