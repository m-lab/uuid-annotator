package ipservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/metrics"
)

// Client is the interface for all users who want an IP annotated from the
// uuid-annotator service.
//
// Behind the scenes, the client is an http client and the server is an http
// server, connecting to each other via unix-domain sockets. Except for the name
// of the socket, all details of how the IPC is done should be considered
// internal and subject to change without notice. In particular, if the overhead
// of encoding and decoding lots of HTTP transactions ends up being too high, we
// reserve the right to change away from HTTP without warning.
type Client interface {
	// Annotate gets the ClientAnnotations associated with each of the valid
	// passed-in IP addresses. Invalid IPs will not be present in the returned
	// map.
	Annotate(ctx context.Context, ips []string) (map[string]*annotator.ClientAnnotations, error)
}

// getter defines the subset of the interface of http.Client that we use, in an
// effort to enable mocking and testing.
type getter interface {
	Get(url string) (resp *http.Response, err error)
}

type client struct {
	sockfilename string
	httpc        getter
}

func (c *client) Annotate(ctx context.Context, ips []string) (map[string]*annotator.ClientAnnotations, error) {
	ipvalues := url.Values{}
	for _, ip := range ips {
		ipvalues.Add("ip", ip)
	}
	u := "http://unix/v1/annotate/ips?" + ipvalues.Encode()
	resp, err := c.httpc.Get(u)
	if err != nil {
		metrics.ClientRPCCount.WithLabelValues("get_error").Inc()
		return nil, err
	}
	if resp.StatusCode != 200 {
		metrics.ClientRPCCount.WithLabelValues("http_status_error").Inc()
		return nil, fmt.Errorf("Got HTTP %d, but wanted HTTP 200", resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		metrics.ClientRPCCount.WithLabelValues("read_error").Inc()
		return nil, err
	}
	ann := make(map[string]*annotator.ClientAnnotations)
	err = json.Unmarshal(b, &ann)
	if err == nil {
		metrics.ClientRPCCount.WithLabelValues("success").Inc()
	} else {
		metrics.ClientRPCCount.WithLabelValues("unmarshal_error").Inc()
	}
	return ann, err
}

// NewClient creates an RPC client for annotating IP addresses. The only RPC
// that is performed should happen through objects returned from this function.
// All other forms of RPC to the local IP annotation service have no long-term
// compatibility guarantees.
//
// The recommended value to pass into this function is the value of the
// command-line flag `--ipservice.SocketFilename`, which is pointed to by
// `ipservice.SocketFilename`.
func NewClient(sockfilename string) Client {
	if sockfilename != *SocketFilename {
		log.Printf("WARNING: socket filename of %q differs from command-line flag value of %q\n", sockfilename, *SocketFilename)
	}
	return &client{
		sockfilename: sockfilename,
		httpc: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", sockfilename)
				},
			},
		},
	}
}
