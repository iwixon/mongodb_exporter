package serverstatusv5

import (
	"io/ioutil"
	"math"
	"path/filepath"
	"testing"

	"github.com/percona/mongodb_exporter/collector/serverstatusv5/fixturetool"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.mongodb.org/mongo-driver/bson"
)

type rawModuleCollector struct {
	module *Module
	raw    bson.Raw
}

func (c *rawModuleCollector) Describe(ch chan<- *prometheus.Desc) { c.module.Describe(ch) }
func (c *rawModuleCollector) Collect(ch chan<- prometheus.Metric) { c.module.Collect(c.raw, ch) }

func loadFixtureRaw(t *testing.T, role string) bson.Raw {
	t.Helper()
	source, err := ioutil.ReadFile(filepath.Join("testdata", "source", "server-status-"+role+"-5.0.34.json"))
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
	return raw
}
func gatherFixture(t *testing.T, role string) map[string]*dto.MetricFamily {
	t.Helper()
	collector := &rawModuleCollector{module: New(), raw: loadFixtureRaw(t, role)}
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]*dto.MetricFamily, len(families))
	for _, family := range families {
		result[family.GetName()] = family
	}
	return result
}
func metricWithLabels(family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	if family == nil {
		return nil
	}
	for _, metric := range family.Metric {
		actual := map[string]string{}
		for _, label := range metric.Label {
			actual[label.GetName()] = label.GetValue()
		}
		match := true
		for key, value := range labels {
			if actual[key] != value {
				match = false
				break
			}
		}
		if match {
			return metric
		}
	}
	return nil
}
func metricValueDTO(metric *dto.Metric) float64 {
	if metric == nil {
		return 0
	}
	if metric.Gauge != nil {
		return metric.Gauge.GetValue()
	}
	if metric.Counter != nil {
		return metric.Counter.GetValue()
	}
	return 0
}

func TestRoleElectionAndPrimaryOnlyServiceCoverage(t *testing.T) {
	primary := gatherFixture(t, "primary")
	secondary := gatherFixture(t, "secondary")
	roleFamily := "mongodb_server_status_repl_role"
	if got := metricValueDTO(metricWithLabels(primary[roleFamily], map[string]string{"state": "primary"})); got != 1 {
		t.Errorf("primary role=%v", got)
	}
	if got := metricValueDTO(metricWithLabels(secondary[roleFamily], map[string]string{"state": "primary"})); got != 0 {
		t.Errorf("secondary primary-role=%v", got)
	}
	if got := metricValueDTO(metricWithLabels(secondary[roleFamily], map[string]string{"state": "secondary"})); got != 1 {
		t.Errorf("secondary role=%v", got)
	}
	election := primary["mongodb_server_status_election_calls_total"]
	if metricWithLabels(election, map[string]string{"reason": "step_up_cmd", "outcome": "successful"}) == nil {
		t.Error("primary step-up success metric missing")
	}
	service := "mongodb_server_status_repl_primary_only_service_state"
	if metricWithLabels(primary[service], map[string]string{"service": "resharding_donor_service", "state": "running"}) == nil {
		t.Error("primary running service state missing")
	}
	if metricWithLabels(secondary[service], map[string]string{"service": "resharding_donor_service", "state": "paused"}) == nil {
		t.Error("secondary paused service state missing")
	}
}
func TestFlowControlCoverageAndConversions(t *testing.T) {
	primary := gatherFixture(t, "primary")
	checks := map[string]float64{
		"mongodb_server_status_flow_control_enabled":                1,
		"mongodb_server_status_flow_control_is_lagged":              0,
		"mongodb_server_status_flow_control_is_lagged_count_total":  54,
		"mongodb_server_status_flow_control_is_lagged_time_seconds": 243.249782,
		"mongodb_server_status_flow_control_time_acquiring_seconds": 206603.115128,
		"mongodb_server_status_flow_control_sustainer_rate":         5000,
		"mongodb_server_status_flow_control_target_rate_limit":      1000000000,
	}
	for family, want := range checks {
		metric := metricWithLabels(primary[family], nil)
		if metric == nil {
			t.Errorf("%s missing", family)
			continue
		}
		if got := metricValueDTO(metric); math.Abs(got-want) > 1e-9 {
			t.Errorf("%s=%v want %v", family, got, want)
		}
	}
}
func TestShardingStatisticsCoverage(t *testing.T) {
	primary := gatherFixture(t, "primary")
	moves := primary["mongodb_server_status_sharding_move_chunk_total"]
	checks := []struct {
		labels map[string]string
		want   float64
	}{
		{map[string]string{"direction": "donor", "outcome": "started"}, 777},
		{map[string]string{"direction": "donor", "outcome": "committed"}, 761},
		{map[string]string{"direction": "donor", "outcome": "aborted"}, 16},
		{map[string]string{"direction": "recipient", "outcome": "started"}, 1159},
	}
	for _, check := range checks {
		metric := metricWithLabels(moves, check.labels)
		if metric == nil {
			t.Errorf("move metric missing: %v", check.labels)
			continue
		}
		if got := metricValueDTO(metric); got != check.want {
			t.Errorf("move %v=%v want %v", check.labels, got, check.want)
		}
	}
	static := map[string]float64{
		"mongodb_server_status_sharding_statistics_count_bytes_cloned_on_recipient_total": 20518281469,
		"mongodb_server_status_sharding_statistics_count_bytes_cloned_on_donor_total":     22622841111,
		"mongodb_server_status_sharding_statistics_count_stale_config_errors_total":       168416,
	}
	for family, want := range static {
		metric := metricWithLabels(primary[family], nil)
		if metric == nil {
			t.Errorf("%s missing", family)
			continue
		}
		if got := metricValueDTO(metric); got != want {
			t.Errorf("%s=%v want %v", family, got, want)
		}
	}
}
func TestSessionTransactionReadConcernAndOplogCoverage(t *testing.T) {
	primary := gatherFixture(t, "primary")
	checks := map[string]float64{
		"mongodb_server_status_logical_session_record_cache_active_sessions_count":                         3899,
		"mongodb_server_status_logical_session_record_cache_last_sessions_collection_job_duration_seconds": 0.837,
		"mongodb_server_status_transactions_current_open":                                                  0,
		"mongodb_server_status_transactions_total_committed_total":                                         0,
		"mongodb_server_status_read_concern_counters_non_transaction_ops_majority_total":                   0,
		"mongodb_server_status_oplog_truncation_total_time_truncating_seconds":                             789.430184,
		"mongodb_server_status_oplog_truncation_truncate_count_total":                                      1736,
		"mongodb_server_status_repl_rbid":                                                                  1,
	}
	for family, want := range checks {
		metric := metricWithLabels(primary[family], nil)
		if metric == nil {
			t.Errorf("%s missing", family)
			continue
		}
		if got := metricValueDTO(metric); math.Abs(got-want) > 1e-9 {
			t.Errorf("%s=%v want %v", family, got, want)
		}
	}
	var sparse bson.Raw
	sparseBytes, err := bson.Marshal(bson.M{"transactions": bson.M{"currentOpen": 0}})
	if err != nil {
		t.Fatal(err)
	}
	sparse = bson.Raw(sparseBytes)
	metrics := make(chan prometheus.Metric, 32)
	report := New().Collect(sparse, metrics)
	close(metrics)
	if len(report.Errors) > 0 {
		t.Fatalf("sparse errors: %+v", report.Errors)
	}
	for metric := range metrics {
		if metric.Desc().String() == "" {
			t.Fatal("invalid metric")
		}
	}
}
func TestBoundedWorkloadCoverage(t *testing.T) {
	primary := gatherFixture(t, "primary")
	commands := primary["mongodb_server_status_commands_total"]
	if got := metricValueDTO(metricWithLabels(commands, map[string]string{"command": "find", "outcome": "total"})); got != 2113596255 {
		t.Errorf("find total=%v", got)
	}
	if got := metricValueDTO(metricWithLabels(commands, map[string]string{"command": "find", "outcome": "failed"})); got != 590 {
		t.Errorf("find failed=%v", got)
	}
	if got := metricValueDTO(metricWithLabels(primary["mongodb_server_status_aggregation_stage_total"], map[string]string{"stage": "$group"})); got != 369667 {
		t.Errorf("group stage=%v", got)
	}
	if got := metricValueDTO(metricWithLabels(primary["mongodb_server_status_operator_total"], map[string]string{"category": "expressions", "operator": "$eq"})); got != 686313 {
		t.Errorf("operator eq=%v", got)
	}
	checks := map[string]float64{"mongodb_server_status_metrics_query_update_many_count_total": 144579867, "mongodb_server_status_metrics_cursor_total_opened_total": 6062679, "mongodb_server_status_metrics_cursor_open": 2}
	for family, want := range checks {
		metric := metricWithLabels(primary[family], nil)
		if metric == nil {
			t.Errorf("%s missing", family)
			continue
		}
		if got := metricValueDTO(metric); got != want {
			t.Errorf("%s=%v want %v", family, got, want)
		}
	}
}
func TestNetworkSecurityAndStorageCoverage(t *testing.T) {
	primary := gatherFixture(t, "primary")
	checks := map[string]float64{
		"mongodb_server_status_network_physical_bytes_in_total":                        33523969332746,
		"mongodb_server_status_network_num_slow_ssloperations_total":                   412,
		"mongodb_server_status_network_tcp_fast_open_server_supported":                 1,
		"mongodb_server_status_security_sslserver_certificate_expiration_date_seconds": 1796194911,
		"mongodb_server_status_storage_engine_supports_committed_reads":                1,
		"mongodb_server_status_storage_engine_drop_pending_idents":                     0,
	}
	for family, want := range checks {
		metric := metricWithLabels(primary[family], nil)
		if metric == nil {
			t.Errorf("%s missing", family)
			continue
		}
		if got := metricValueDTO(metric); got != want {
			t.Errorf("%s=%v want %v", family, got, want)
		}
	}
	compression := metricWithLabels(primary["mongodb_server_status_network_compression_bytes_total"], map[string]string{"compressor": "zstd", "operation": "compressor", "direction": "in"})
	if got := metricValueDTO(compression); got != 363747234298988 {
		t.Errorf("zstd bytes in=%v", got)
	}
	executor := metricWithLabels(primary["mongodb_server_status_network_service_executor"], map[string]string{"executor": "passthrough", "state": "threads_running"})
	if got := metricValueDTO(executor); got != 2689 {
		t.Errorf("executor threads=%v", got)
	}
	auth := metricWithLabels(primary["mongodb_server_status_security_authentication_total"], map[string]string{"mechanism": "MONGODB-X509", "conversation": "authenticate", "outcome": "successful"})
	if got := metricValueDTO(auth); got != 75739 {
		t.Errorf("x509 authentication=%v", got)
	}
}
func TestSelectedWiredTigerCoverage(t *testing.T) {
	primary := gatherFixture(t, "primary")
	concurrent := primary["mongodb_server_status_wired_tiger_concurrent_transactions"]
	if got := metricValueDTO(metricWithLabels(concurrent, map[string]string{"operation": "write", "state": "out"})); got != 7 {
		t.Errorf("write tickets out=%v", got)
	}
	if got := metricValueDTO(metricWithLabels(concurrent, map[string]string{"operation": "read", "state": "available"})); got != 127 {
		t.Errorf("read tickets available=%v", got)
	}
	checks := map[string]float64{
		"mongodb_server_status_wired_tiger_snapshot_window_settings_total_number_of_snapshot_too_old_errors_total":     0,
		"mongodb_server_status_wired_tiger_snapshot_window_settings_current_available_snapshot_window_size_in_seconds": 300,
	}
	for family, want := range checks {
		metric := metricWithLabels(primary[family], nil)
		if metric == nil {
			t.Errorf("%s missing", family)
			continue
		}
		if got := metricValueDTO(metric); got != want {
			t.Errorf("%s=%v want %v", family, got, want)
		}
	}
	contract, paths := loadCoverageInputs(t)
	if err := contract.Audit(paths); err != nil {
		t.Fatal(err)
	}
}
