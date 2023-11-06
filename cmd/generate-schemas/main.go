package main

import (
	"flag"
	"os"

	"github.com/m-lab/go/cloud/bqx"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/uuid-annotator/annotator"

	"cloud.google.com/go/bigquery"
)

var (
	ann2schema string
	hop2schema string
)

func init() {
	flag.StringVar(&ann2schema, "ann2", "/var/spool/datatypes/annotation2.json", "filename to write annotation2 schema")
	flag.StringVar(&hop2schema, "hop2", "/var/spool/datatypes/hopannotation2.json", "filename to write hopannotation2 schema")
}

func main() {
	flag.Parse()
	// Generate and save annotation2 schema for autoloading.
	ann2 := annotator.Annotations{}
	sch, err := bigquery.InferSchema(ann2)
	rtx.Must(err, "failed to generate annotation2 schema")
	sch = bqx.RemoveRequired(sch)
	b, err := sch.ToJSONFields()
	rtx.Must(err, "failed to marshal schema")
	os.WriteFile(ann2schema, b, 0o644)

	// Generate and save hopannotation2 schema for autoloading.
	hop2 := annotator.ClientAnnotations{}
	sch, err = bigquery.InferSchema(hop2)
	rtx.Must(err, "failed to generate hopannotation2 schema")
	sch = bqx.RemoveRequired(sch)
	b, err = sch.ToJSONFields()
	rtx.Must(err, "failed to marshal schema")
	os.WriteFile(hop2schema, b, 0o644)
}
