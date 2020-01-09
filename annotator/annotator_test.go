package annotator

import (
	"encoding/json"
	"testing"

	"github.com/m-lab/go/rtx"
)

func TestJSONSerialization(t *testing.T) {
	_, err := json.Marshal(Annotations{})
	rtx.Must(err, "Could not serialize annotations to JSON")
}
