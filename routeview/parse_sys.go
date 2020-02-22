package routeview

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"sort"

	"github.com/m-lab/uuid-annotator/annotator"
)

// SIPNet represents a parsed row in a RouteView file.
type SIPNet struct {
	net.IPNet
	Systems []annotator.System
}

// SIPNetSlice is a sortable (and searchable) array of IPNets.
type SIPNetSlice []SIPNet

type SIndex struct {
	n SIPNetSlice
}

func (p SIPNetSlice) Len() int {
	return len(p)
}
func (p SIPNetSlice) Less(i, j int) bool {
	return bytes.Compare(p[i].IP, p[j].IP) < 0
}
func (p SIPNetSlice) Swap(i, j int) {
	n := p[j]
	p[j] = p[i]
	p[i] = n
}

func equalSystems(a []annotator.System, b []annotator.System) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i].ASNs) != len(b[i].ASNs) {
			return false
		}
		for j := range a[i].ASNs {
			if a[i].ASNs[j] != b[i].ASNs[j] {
				return false
			}
		}
	}
	return true
}

// SParseRouteView reads the given csv file and generates a sorted IP list.
func SParseRouteView(file []byte) *SIndex {
	result := SIPNetSlice{}
	sys := map[string][]annotator.System{}

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
		if _, ok := sys[record[2]]; !ok {
			sys[record[2]] = ParseSystems(record[2])
		}
		if len(result) > 1 &&
			result[len(result)-1].Contains(n.IP) &&
			equalSystems(result[len(result)-1].Systems, sys[record[2]]) {
			// If the last thing contains the current one witht he same systems, skip it.
			skip++
			continue
		}
		result = append(result, SIPNet{IPNet: *n, Systems: sys[record[2]]})
	}
	fmt.Println("skip:", skip)

	// Sort list so that it can be searched.
	sort.Sort(result)
	return &SIndex{n: result}
}

// Search attempts to find the given src IP in n.
func (rv *SIndex) Search(src string) (SIPNet, error) {
	// bytes.Compare will only work correctly when both net.IPs have the same byte count.
	ip := net.ParseIP(src)
	if ip.To4() != nil {
		ip = ip.To4()
	}
	x := sort.Search(len(rv.n), func(i int) bool {
		if rv.n[i].Contains(ip) {
			// Becaue sort.Search finds the lowest index where f(i) is true, we must return
			// true when the IPNet contains the given IP to prevent off by 1 errors.
			return true
		}
		return bytes.Compare(rv.n[i].IP, ip) >= 0
	})
	if rv.n[x].Contains(ip) {
		return rv.n[x], nil
	}
	return SIPNet{}, ErrNoASNFound
}
