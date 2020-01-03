package handler

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/m-lab/tcp-info/eventsocket"
	"github.com/m-lab/tcp-info/inetdiag"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/metrics"
)

type job struct {
	timestamp time.Time
	uuid      string
	id        *inetdiag.SockID
}

func (j *job) WriteFile(dir string, data *annotator.Annotations) error {
	// Serialize to JSON
	contents, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Create the necessary subdirectories.
	dir = dir + j.timestamp.Format("/2006/01/02/")
	err = os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}

	// Write the serialized data
	return ioutil.WriteFile(dir+j.uuid+".json", contents, 0666)
}

type handler struct {
	datadir    string
	jobs       chan *job
	annotators []annotator.Annotator
}

// Open adds a new .json file to the work queue.
func (h *handler) Open(ctx context.Context, timestamp time.Time, uuid string, ID *inetdiag.SockID) {
	select {
	case h.jobs <- &job{
		timestamp: timestamp,
		uuid:      uuid,
		id:        ID,
	}:
	default:
		metrics.MissedJobs.WithLabelValues("pipefull").Inc()
	}
}

// Close is a no-op, implemented here to ensure that handler implements all of eventsocket.Handler.
func (*handler) Close(ctx context.Context, timestamp time.Time, uuid string) {}

func (h *handler) ProcessIncomingRequests(ctx context.Context) {
	for ctx.Err() == nil {
		select {
		case j, ok := <-h.jobs:
			if ok && j != nil {
				annotations := &annotator.Annotations{
					UUID:      j.uuid,
					Timestamp: j.timestamp,
				}
				for _, ann := range h.annotators {
					err := ann.Annotate(j.id, annotations)
					if err != nil {
						metrics.AnnotationErrors.Inc()
					}
				}

				if err := j.WriteFile(h.datadir, annotations); err != nil {
					log.Println("Could not write metadata to file:", err)
					metrics.MissedJobs.WithLabelValues("writefail").Inc()
				}
			}
		case <-ctx.Done():
		}
	}
}

// ThreadedHandler is an eventsocket.Handler that has a separate method for
// handling incoming requests in a separate goroutine, to ensure that event
// notifications are not missed.
type ThreadedHandler interface {
	eventsocket.Handler
	ProcessIncomingRequests(ctx context.Context)
}

// New creates an eventsocket.Handler that saves the metadata for each file. The
// actual handling of each event is in a separate goroutine that should be
// started by calling ProcessIncomingRequests. This two-part handling is there
// to ensure that events arriving close together are not missed, even if disk IO
// latency is high.
func New(datadir string, buffersize int, annotators []annotator.Annotator) ThreadedHandler {
	return &handler{
		datadir:    datadir,
		annotators: annotators,
		// Buffer jobs in case a burst of IOps makes the disk slow.
		jobs: make(chan *job, buffersize),
	}
}
