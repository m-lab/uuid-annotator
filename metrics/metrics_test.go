package metrics

import (
	"testing"

	"github.com/m-lab/go/prometheusx/promtest"
)

func TestMetrics(t *testing.T) {
	MissedJobs.WithLabelValues("x").Inc()
	GCSFilesLoaded.WithLabelValues("x").Inc()
	ServerRPCCount.WithLabelValues("x").Inc()
	ClientRPCCount.WithLabelValues("x").Inc()
	promtest.LintMetrics(t)
}
