package routeview

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/m-lab/go/logx"
	"github.com/m-lab/uuid-annotator/annotator"
	"github.com/m-lab/uuid-annotator/metrics"
)

// IPNet represents a parsed row in a RouteView file.
type IPNet struct {
	net.IPNet
	Systems string
}

// NetIndex is a sortable and searchable array of IPNets.
type NetIndex []IPNet

// Index is searchable array of NetIndexes.
type Index []NetIndex

// Len, Less, and Swap make Index sortable.
func (ns NetIndex) Len() int {
	return len(ns)
}
func (ns NetIndex) Less(i, j int) bool {
	return bytes.Compare(ns[i].IP, ns[j].IP) < 0
}
func (ns NetIndex) Swap(i, j int) {
	n := ns[j]
	ns[j] = ns[i]
	ns[i] = n
}

// ParseSystems converts the RouteView AS string to an annotator.System array.
// Invalid values are ignored.
//
// RouteViews may contain:
// * a single AS number, e.g. "32", one System with one ASN
// * an AS set, e.g. "32,54", one System with multiple ASNs
// * a Multi-Origin AS (MOAS), e.g. "10_20", two Systems each with one or more ASNs.
func ParseSystems(s string) []annotator.System {
	// Split systems on "_"
	systems := strings.Split(s, "_")
	result := make([]annotator.System, 0, len(systems))
	for _, asn := range systems {
		// Split the ASN sets on commas.
		asns := strings.Split(asn, ",")
		ints := make([]uint32, 0, len(asns))
		for _, asn := range asns {
			value, err := strconv.ParseUint(asn, 10, 32)
			if err != nil {
				log.Println(err)
				continue
			}
			ints = append(ints, uint32(value))
		}
		result = append(result, annotator.System{ASNs: ints})
	}
	return result
}

// ParseRouteView reads the given csv file and generates a sorted IP list.
func ParseRouteView(file []byte) Index {
	sm := map[string]string{}

	skip := 0
	parsed := 0
	b := bytes.NewBuffer(file)
	r := csv.NewReader(b)
	r.Comma = '\t'
	r.ReuseRecord = true

	nim := map[int64]NetIndex{}

	for {
		record, err := r.Read()
		if err == io.EOF {
			metrics.RouteViewParsed.Inc()
			break
		}
		if len(record) < 3 {
			metrics.RouteViewRows.WithLabelValues("missing-fields").Inc()
			continue
		}
		nb, err := strconv.ParseInt(record[1], 10, 32)
		if err != nil {
			// Skip malformed line.
			skip++
			log.Println("failed to convert netblock size:", record[1])
			metrics.RouteViewRows.WithLabelValues("corrupt-netblock").Inc()
			continue
		}
		_, n, err := net.ParseCIDR(record[0] + "/" + record[1])
		if err != nil {
			// Skip malformed line.
			skip++
			log.Println("failed to parse CIDR prefix:", record[0], "with netblock:", record[1])
			metrics.RouteViewRows.WithLabelValues("corrupt-prefix").Inc()
			continue
		}
		if _, ok := sm[record[2]]; !ok {
			// Break string connection to underlying RAM allocated by the CSV reader.
			sm[record[2]] = strings.Repeat(record[2], 1)
		}
		parsed++
		metrics.RouteViewRows.WithLabelValues("parsed").Inc()
		nim[nb] = append(nim[nb], IPNet{IPNet: *n, Systems: sm[record[2]]})
	}
	logx.Debug.Println("Skipped:", skip, "routeview netblocks of", parsed+skip)

	// For each netblock, sort each NetIndex array.
	netblocks := []int64{}
	for k := range nim {
		netblocks = append(netblocks, k)
		sort.Sort(nim[k])
	}
	// Sort descending order.
	sort.Slice(netblocks, func(i, j int) bool { return netblocks[i] > netblocks[j] })

	// Construct the final index, from largest to smallest netblock.
	ix := Index{}
	for _, k := range netblocks {
		ix = append(ix, nim[k])
	}
	return ix
}

// ErrNoASNFound is returned when search fails to identify a network for the given src IP.
var ErrNoASNFound = errors.New("no ASN found for address")

// Search attempts to find the given IP in the Index.
func (ix Index) Search(s string) (IPNet, error) {
	// bytes.Compare will only work correctly when both net.IPs have the same byte count.
	ip := net.ParseIP(s)
	if ip.To4() != nil {
		ip = ip.To4()
	}
	// Search each set of NetIndexes from longest to shortest, returning the first (longest) match.
	for i := range ix {
		ns := ix[i]
		node := sort.Search(len(ns), func(i int) bool {
			if ns[i].Contains(ip) {
				// Becaue sort.Search finds the lowest index where f(i) is true, we must return
				// true when the IPNet contains the given IP to prevent off by 1 errors.
				return true
			}
			return bytes.Compare(ns[i].IP, ip) >= 0
		})
		if node < len(ns) && ns[node].Contains(ip) {
			return ns[node], nil
		}
	}
	return IPNet{}, ErrNoASNFound
}
