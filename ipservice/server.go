package ipservice

import (
	"encoding/json"
	"log"
	"net"
	"net/http"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/asnannotator"
	"github.com/m-lab/uuid-annotator/geoannotator"
	"github.com/m-lab/uuid-annotator/metrics"
)

// Server provides the http-over-unix-domain-socket service that serves up annotated IP addresses on request.
type Server interface {
	Close() error
	Serve() error
}

type handler struct {
	asn asnannotator.ASNAnnotator
	geo geoannotator.GeoAnnotator
}

func (h *handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ipstring := req.URL.Query().Get("ip")
	ip := net.ParseIP(ipstring)
	if ipstring == "" || ip == nil {
		log.Println("Could not process request ip argument")
		rw.WriteHeader(http.StatusBadRequest)
		metrics.ServerRPCCount.WithLabelValues("badip_error").Inc()
		return
	}
	a := &annotator.ClientAnnotations{}
	if h.asn != nil {
		a.Network = h.asn.AnnotateIP(ipstring) // Should this error be ignored?
	}
	if h.geo != nil {
		h.geo.AnnotateIP(ip, &a.Geo) // Should this error be ignored?
	}

	b, err := json.Marshal(a)
	rtx.Must(err, "Could not marshal the response. This should never happen and is a bug.")

	_, err = rw.Write(b)
	if err != nil {
		log.Println("Could not write response due to error:", err)
		metrics.ServerRPCCount.WithLabelValues("write_error").Inc()
		return
	}
	metrics.ServerRPCCount.WithLabelValues("success").Inc()
}

type server struct {
	listener net.Listener
	srv      *http.Server
}

func (s *server) Serve() error {
	return s.srv.Serve(s.listener)
}

func (s *server) Close() error {
	return s.srv.Close()
}

// NewServer creates an RPC service for annotating IP addresses. The RPC service
// can be called by the returned objects from NewClient.
//
// The returned object should have its Serve() method called, likely in a
// goroutine. To stop the server, call Close().
//
// The recommended sockfilename value to pass into this function is the value of
// the command-line flag `--ipservice.SocketFilename`, which is pointed to by
// `ipservice.SocketFilename`.
//
// If you would like to set up a server for use in unit tests outside this
// package, the easiest way of doing that is to pass in `nil` for `asn` and
// `geo`. The server will still work and run and exercise all its parsing an
// deserialization logic, but it will never fill in any data. If you need the
// server to contain dummy data for your test to work, then please file a bug
// in this repo asking the maintainer of this package to build a fake.
func NewServer(sockfilename string, asn asnannotator.ASNAnnotator, geo geoannotator.GeoAnnotator) (Server, error) {
	if sockfilename != *SocketFilename {
		log.Printf("WARNING: socket filename of %q differs from command-line flag value of %q\n", sockfilename, *SocketFilename)
	}
	listener, err := net.Listen("unix", sockfilename)
	if err != nil {
		return nil, err
	}

	h := &handler{
		asn: asn,
		geo: geo,
	}

	mux := http.NewServeMux()
	mux.Handle("/ip", h)
	srv := &http.Server{
		Handler: mux,
	}

	return &server{
		listener: listener,
		srv:      srv,
	}, nil
}
