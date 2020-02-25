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

	"github.com/m-lab/uuid-annotator/annotator"
)

// IPNet represents a parsed row in a RouteView file.
type IPNet struct {
	net.IPNet
	Systems string
}

// IPNetSlice is a sortable (and searchable) array of IPNets.
type IPNetSlice []IPNet

// Len, Less, and Swap make IPNetSlice sortable.
func (ns IPNetSlice) Len() int {
	return len(ns)
}
func (ns IPNetSlice) Less(i, j int) bool {
	return bytes.Compare(ns[i].IP, ns[j].IP) < 0
}
func (ns IPNetSlice) Swap(i, j int) {
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
func ParseRouteView(file []byte) IPNetSlice {
	result := IPNetSlice{}
	sm := map[string]string{}

	skip := 0
	b := bytes.NewBuffer(file)
	r := csv.NewReader(b)
	r.Comma = '\t'
	r.ReuseRecord = true

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if len(record) < 3 {
			continue
		}
		_, n, err := net.ParseCIDR(record[0] + "/" + record[1])
		if _, ok := sm[record[2]]; !ok {
			// Break string connection to underlying RAM allocated by the CSV reader.
			dst := make([]byte, len(record[2]))
			copy(dst, []byte(record[2]))
			sm[record[2]] = string(dst)
		}
		if len(result) > 1 && result[len(result)-1].Contains(n.IP) && result[len(result)-1].Systems == record[2] {
			// If the last network contains the current one with the same systems, skip it.
			skip++
			continue
		}
		result = append(result, IPNet{IPNet: *n, Systems: sm[record[2]]})
	}
	log.Println("Skipped:", skip, "routeview netblocks of", len(result)+skip)

	// Sort list so that it can be searched.
	sort.Sort(result)
	return result
}

// ErrNoASNFound is returned when search fails to identify a network for the given src IP.
var ErrNoASNFound = errors.New("No ASN found for address")

// Search attempts to find the given IP in the IPNetSlice.
func (ns IPNetSlice) Search(IP string) (IPNet, error) {
	// bytes.Compare will only work correctly when both net.IPs have the same byte count.
	ip := net.ParseIP(IP)
	if ip.To4() != nil {
		ip = ip.To4()
	}
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
	return IPNet{}, ErrNoASNFound
}
