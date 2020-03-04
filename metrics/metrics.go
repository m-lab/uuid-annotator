// Package metrics provides all metrics exported to prometheus by this repo. We
// centralize our metrics here, because then when we lint our metrics, the
// linter will not know about (and therefore not complain about) metrics created
// by other libraries.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics exported to prometheus for run-time monitoring.
var (
	MissedJobs = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "uuid_annotator_missed_uuids_total",
			Help: "The number of UUIDs that we received but could not create a file for. Should always be zero.",
		},
		[]string{"reason"},
	)
	AnnotationErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "uuid_annotator_annotation_errors_total",
			Help: "The number of times annotation returned an error",
		},
	)
	GCSFilesLoaded = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "uuid_annotator_gcs_hash_loaded",
			Help: "The hash of the loaded GCS file",
		},
		[]string{"md5"},
	)
	ServerRPCCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "uuid_annotator_server_rpcs_total",
			Help: "The number of times the server-side of the RPC service has been called, and whether it was success or not",
		},
		[]string{"status"},
	)
	ClientRPCCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "uuid_annotator_client_rpcs_total",
			Help: "The number of times the client-side of the RPC service has been called, and whether it was success or not",
		},
		[]string{"status"},
	)
)
