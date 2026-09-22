package serverstatusv5

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.mongodb.org/mongo-driver/bson"
)

func TestMalformedValueIsolatesMetricFamily(t *testing.T) {
	original := metricDefinitions
	defer func() { metricDefinitions = original }()
	metricDefinitions = []metricDefinition{
		{Path: "broken.good", Family: "mongodb_server_status_test_broken", Help: "broken", Type: "gauge", Conversion: 1},
		{Path: "broken.bad", Family: "mongodb_server_status_test_broken", Help: "broken", Type: "gauge", Conversion: 1},
		{Path: "healthy.value", Family: "mongodb_server_status_test_healthy", Help: "healthy", Type: "gauge", Conversion: 1},
	}
	rawBytes, err := bson.Marshal(bson.M{"broken": bson.M{"good": 1, "bad": "invalid"}, "healthy": bson.M{"value": 2}})
	if err != nil {
		t.Fatal(err)
	}
	metrics := make(chan prometheus.Metric, 16)
	report := New().Collect(bson.Raw(rawBytes), metrics)
	close(metrics)
	if len(report.Errors) != 1 || report.Errors[0].Family != "mongodb_server_status_test_broken" {
		t.Fatalf("errors=%+v", report.Errors)
	}
	if report.Emitted != 1 {
		t.Fatalf("emitted=%d want 1", report.Emitted)
	}
	seenHealthy, seenBroken, diagnostic := false, false, float64(0)
	for metric := range metrics {
		description := metric.Desc().String()
		var wire dto.Metric
		if err := metric.Write(&wire); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(description, "mongodb_server_status_test_healthy") {
			seenHealthy = true
		}
		if strings.Contains(description, "mongodb_server_status_test_broken") {
			seenBroken = true
		}
		if strings.Contains(description, "mongodb_server_status_decode_errors_total") && wire.Counter != nil {
			diagnostic += wire.Counter.GetValue()
		}
	}
	if !seenHealthy {
		t.Error("healthy family was suppressed")
	}
	if seenBroken {
		t.Error("malformed family emitted partial samples")
	}
	if diagnostic != 1 {
		t.Fatalf("decode diagnostic=%v want 1", diagnostic)
	}
}
