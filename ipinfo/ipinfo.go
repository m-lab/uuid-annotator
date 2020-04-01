package ipinfo

import (
	"bytes"
	"encoding/csv"
	"log"
	"strconv"
)

// ASNames is the type holding a map from AS numbers to their names.
type ASNames map[uint32]string

// Parse the data read from the file given to us by the folks at IPInfo.io.
func Parse(data []byte) (ASNames, error) {
	newmap := make(ASNames)
	rows, err := csv.NewReader(bytes.NewBuffer(data)).ReadAll()
	if err != nil {
		return nil, err
	}
	// Start from row[1] not row[0] to skip the csv header.
	for _, row := range rows[1:] {
		if len(row) < 2 {
			log.Println("Bad CSV row (not enough entries). This should never happen.", row)
			continue
		}
		asnstring := row[0]
		asname := row[1]
		asn, err := strconv.ParseUint(asnstring[2:], 10, 32)
		if err != nil {
			log.Println("Parse error on a single CSV row (this should never happen):", err, row)
			continue
		}
		newmap[uint32(asn)] = asname
	}
	return newmap, nil
}
