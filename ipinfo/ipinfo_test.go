package ipinfo

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    ASNames
		wantErr bool
	}{
		{
			name: "Good data",
			data: []byte("asn,name\nAS0001,test\nAS65535,t2\n"),
			want: ASNames{
				1:     "test",
				65535: "t2",
			},
		},
		{
			name: "Bad row",
			data: []byte("asn,name\nAS0001,test\nASERROR,t2\n"),
			want: ASNames{
				1: "test",
			},
		},
		{
			name: "Correctly formatted csv with a too-short row length",
			data: []byte("asn\nAS0001\n"),
			want: ASNames{},
		},
		{
			name:    "Not a CSV",
			data:    []byte("two,records\nonerecord\n"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
