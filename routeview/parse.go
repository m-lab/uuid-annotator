package routeview

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/m-lab/uuid-annotator/annotator"
)

type IPNet struct {
	net.IPNet
	Systems []annotator.System
}

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

func AsSystems(s string) []annotator.System {
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

// Parse reads the given csv file and generates a sorted IP list.
func Parse(file []byte) IPNetSlice {
	result := IPNetSlice{}

	b := bytes.NewBuffer(file)
	r := csv.NewReader(b)
	r.Comma = '\t'
	c := 0

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		_, n, err := net.ParseCIDR(record[0] + "/" + record[1])
		result = append(result, IPNet{IPNet: *n, Systems: AsSystems(record[2])})
		c++
		//if c > 50 {
		//	break
		//}
	}

	sort.Sort(result)
	return result
}

func TrySearch(n IPNetSlice) {

	tests := []struct {
		src  string
		want net.IPNet
	}{
		{src: "1.0.0.1"},
		{src: "1.0.174.1"},
		{src: "1.0.192.1"},
		{src: "9.9.0.1"},
	}
	for _, t := range tests {
		// bytes.Compare is not helpful unless both net.IPs have the same byte count.
		ip := net.ParseIP(t.src).To4()
		x := sort.Search(len(n), func(i int) bool {
			fmt.Println("s:", ip, i, n[i], bytes.Compare(ip, n[i].IP) <= 0, n[i].Contains(ip))
			if n[i].Contains(ip) {
				return true
			}
			return bytes.Compare(n[i].IP, ip) >= 0
		})
		if x < len(n) {
			fmt.Println("found:", n[x], n[x].Contains(ip))
		}
	}

}
