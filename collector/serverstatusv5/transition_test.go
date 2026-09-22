package serverstatusv5

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func gatherRegistered(t *testing.T, registry *prometheus.Registry) map[string]*dto.MetricFamily {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]*dto.MetricFamily{}
	for _, family := range families {
		result[family.GetName()] = family
	}
	return result
}
func TestBidirectionalRoleTransitionsDoNotLeakSamples(t *testing.T) {
	for _, sequence := range [][]string{{"primary", "secondary"}, {"secondary", "primary"}} {
		t.Run(sequence[0]+"-to-"+sequence[1], func(t *testing.T) {
			module := New()
			collector := &rawModuleCollector{module: module, raw: loadFixtureRaw(t, sequence[0])}
			registry := prometheus.NewPedanticRegistry()
			if err := registry.Register(collector); err != nil {
				t.Fatal(err)
			}
			first := gatherRegistered(t, registry)
			firstInsert := metricWithLabels(first["mongodb_server_status_commands_total"], map[string]string{"command": "insert", "outcome": "total"})
			collector.raw = loadFixtureRaw(t, sequence[1])
			second := gatherRegistered(t, registry)
			secondInsert := metricWithLabels(second["mongodb_server_status_commands_total"], map[string]string{"command": "insert", "outcome": "total"})
			if sequence[0] == "primary" {
				if firstInsert == nil {
					t.Error("primary insert sample missing")
				}
				if secondInsert != nil {
					t.Error("primary-only insert sample leaked into secondary gather")
				}
			} else {
				if firstInsert != nil {
					t.Error("secondary unexpectedly emitted insert sample")
				}
				if secondInsert == nil {
					t.Error("primary insert sample missing after role transition")
				}
			}
		})
	}
}
