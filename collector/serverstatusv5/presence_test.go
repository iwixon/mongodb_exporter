package serverstatusv5

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.mongodb.org/mongo-driver/bson"
)

func TestPresenceAwareBSONSemantics(t *testing.T) {
	original := metricDefinitions
	defer func() { metricDefinitions = original }()
	fields := []string{"int32", "int64", "double", "date", "timestamp", "boolean", "zero", "missing", "null"}
	metricDefinitions = make([]metricDefinition, 0, len(fields))
	for _, field := range fields {
		metricDefinitions = append(metricDefinitions, metricDefinition{Path: field, Family: "mongodb_server_status_test_" + field, Help: field, Type: "gauge", Conversion: 1})
	}
	document := []byte(`{"int32":{"$numberInt":"7"},"int64":{"$numberLong":"8"},"double":{"$numberDouble":"1.5"},"date":{"$date":{"$numberLong":"1660000000000"}},"timestamp":{"$timestamp":{"t":42,"i":3}},"boolean":false,"zero":{"$numberLong":"0"},"null":null}`)
	var raw bson.Raw
	if err := bson.UnmarshalExtJSON(document, true, &raw); err != nil {
		t.Fatal(err)
	}
	metrics := make(chan prometheus.Metric, 32)
	report := New().Collect(raw, metrics)
	close(metrics)
	if len(report.Errors) > 0 {
		t.Fatalf("unexpected errors: %+v", report.Errors)
	}
	if report.Emitted != 7 {
		t.Fatalf("emitted=%d want 7", report.Emitted)
	}
	values := map[string]float64{}
	for metric := range metrics {
		var wire dto.Metric
		if err := metric.Write(&wire); err != nil {
			t.Fatal(err)
		}
		description := metric.Desc().String()
		for _, field := range fields {
			if strings.Contains(description, "fqName: \""+"mongodb_server_status_test_"+field+"\"") {
				if wire.Gauge != nil {
					values[field] = wire.Gauge.GetValue()
				}
			}
		}
	}
	expected := map[string]float64{"int32": 7, "int64": 8, "double": 1.5, "date": 1660000000, "timestamp": 42, "boolean": 0, "zero": 0}
	for field, want := range expected {
		got, ok := values[field]
		if !ok || got != want {
			t.Errorf("%s=%v present=%v want %v", field, got, ok, want)
		}
	}
	if _, ok := values["missing"]; ok {
		t.Error("missing field emitted")
	}
	if _, ok := values["null"]; ok {
		t.Error("null field emitted")
	}
}
