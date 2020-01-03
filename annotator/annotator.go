// Package annotator provides structs and interfaces used throughout the
// program.
package annotator

import (
	"time"

	"github.com/m-lab/annotation-service/api"
	"github.com/m-lab/tcp-info/inetdiag"
)

// Annotations contains the standard columns we would like to add as annotations for every UUID.
type Annotations struct {
	UUID           string
	Timestamp      time.Time
	Server, Client api.Annotations
}

// Annotator is the interface that all systems that want to add metadata should implement.
type Annotator interface {
	Annotate(ID *inetdiag.SockID, annotations *Annotations) error
}
