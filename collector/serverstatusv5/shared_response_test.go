package serverstatusv5_test

import (
	"io/ioutil"
	"testing"

	"github.com/percona/mongodb_exporter/collector/mongod"
	"github.com/percona/mongodb_exporter/collector/serverstatusv5"
	"github.com/percona/mongodb_exporter/collector/serverstatusv5/fixturetool"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
)

func TestOneRawResponseServesLegacyAndModernPaths(t *testing.T) {
	source, err := ioutil.ReadFile("testdata/source/server-status-primary-5.0.34.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := fixturetool.ReconstructJSON(source)
	if err != nil {
		t.Fatal(err)
	}
	var raw bson.Raw
	if err := bson.UnmarshalExtJSON(data, true, &raw); err != nil {
		t.Fatal(err)
	}
	var legacy mongod.ServerStatus
	if err := bson.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Version != "5.0.34" {
		t.Fatalf("legacy decode version=%q", legacy.Version)
	}
	metrics := make(chan prometheus.Metric, 1000)
	report := serverstatusv5.New().Collect(raw, metrics)
	close(metrics)
	if len(report.Errors) > 0 {
		t.Fatalf("modern decode errors: %+v", report.Errors)
	}
	if report.Emitted == 0 {
		t.Fatal("modern path emitted no samples")
	}
}
