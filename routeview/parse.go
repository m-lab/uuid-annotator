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
	Systems []annotator.System
}

// IPNetSlice is a sortable (and searchable) array of IPNets.
type IPNetSlice []IPNet

func (p IPNetSlice) Len() int {
	return len(p)
}
func (p IPNetSlice) Less(i, j int) bool {
	return bytes.Compare(p[i].IP, p[j].IP) < 0
}
func (p IPNetSlice) Swap(i, j int) {
	n := p[j]
	p[j] = p[i]
	p[i] = n
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
		ints := make([]uint32, len(asns))
		for i, asn := range asns {
			value, err := strconv.ParseUint(asn, 10, 32)
			if err != nil {
				log.Println(err)
				continue
			}
			ints[i] = uint32(value)
		}
		result = append(result, annotator.System{ASNs: ints})
	}
	return result
}

// ParseRouteView reads the given csv file and generates a sorted IP list.
func ParseRouteView(file []byte) IPNetSlice {
	result := IPNetSlice{}

	b := bytes.NewBuffer(file)
	r := csv.NewReader(b)
	r.Comma = '\t'

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if len(record) < 3 {
			continue
		}
		_, n, err := net.ParseCIDR(record[0] + "/" + record[1])
		result = append(result, IPNet{IPNet: *n, Systems: ParseSystems(record[2])})
	}

	// Sort list so that it can be searched.
	sort.Sort(result)
	return result
}

// ErrNoASNFound is returned when search fails to identify a network for the given src IP.
var ErrNoASNFound = errors.New("No ASN found for address")

// Search attempts to find the given src IP in n.
func Search(n IPNetSlice, src string) (IPNet, error) {
	// bytes.Compare will only work correctly when both net.IPs have the same byte count.
	ip := net.ParseIP(src)
	if ip.To4() != nil {
		ip = ip.To4()
	}
	x := sort.Search(len(n), func(i int) bool {
		// fmt.Println("s:", ip, i, n[i], bytes.Compare(ip, n[i].IP) >= 0, n[i].Contains(ip))
		if n[i].Contains(ip) {
			// Becaue sort.Search finds the lowest index where f(i) is true, we must return
			// true when the IPNet contains the given IP to prevent off by 1 errors.
			return true
		}
		return bytes.Compare(n[i].IP, ip) >= 0
	})
	if n[x].Contains(ip) {
		return n[x], nil
	}
	return IPNet{}, ErrNoASNFound
}
