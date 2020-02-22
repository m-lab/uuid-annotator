package routeview

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"sort"
)

// FIPNet represents a parsed row in a RouteView file.
type FIPNet struct {
	net.IPNet
	Systems string
	// Systems []annotator.System
}

// FIPNetSlice is a sortable (and searchable) array of IPNets.
type FIPNetSlice []FIPNet

type FIndex struct {
	n FIPNetSlice
}

func (p FIPNetSlice) Len() int {
	return len(p)
}
func (p FIPNetSlice) Less(i, j int) bool {
	return bytes.Compare(p[i].IP, p[j].IP) < 0
}
func (p FIPNetSlice) Swap(i, j int) {
	n := p[j]
	p[j] = p[i]
	p[i] = n
}

// FParseRouteView reads the given csv file and generates a sorted IP list.
func FParseRouteView(file []byte) *FIndex {
	result := FIPNetSlice{}

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
		if len(result) > 1 && result[len(result)-1].Contains(n.IP) && (result[len(result)-1].Systems) == record[2] {
			// If the last thing contains the current one witht he same systems, skip it.
			skip++
			continue
		}
		result = append(result, FIPNet{IPNet: *n, Systems: record[2]})
	}
	fmt.Println("skip:", skip)

	// Sort list so that it can be searched.
	sort.Sort(result)
	return &FIndex{n: result}
}

// Search attempts to find the given src IP in n.
func (rv *FIndex) Search(src string) (FIPNet, error) {
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
	return FIPNet{}, ErrNoASNFound
}
