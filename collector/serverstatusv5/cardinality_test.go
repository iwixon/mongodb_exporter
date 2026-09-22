package serverstatusv5

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
)

func TestDynamicCollectorsRejectUnknownKeysAndLabels(t *testing.T) {
	document := bson.M{
		"metrics": bson.M{
			"commands":         bson.M{"find": bson.M{"total": 1, "failed": 0}, "evilCommand": bson.M{"total": 99}},
			"aggStageCounters": bson.M{"$group": 1, "$evilStage": 99},
			"operatorCounters": bson.M{"expressions": bson.M{"$eq": 1, "$evilOperator": 99}},
		},
		"network": bson.M{
			"compression":      bson.M{"zstd": bson.M{"compressor": bson.M{"bytesIn": 1}}, "brotli": bson.M{"compressor": bson.M{"bytesIn": 99}}},
			"serviceExecutors": bson.M{"fixed": bson.M{"threadsRunning": 1}, "unbounded": bson.M{"threadsRunning": 99}},
		},
		"security": bson.M{"authentication": bson.M{"mechanisms": bson.M{"PLAIN": bson.M{"authenticate": bson.M{"successful": 99}}}}},
		"repl":     bson.M{"primaryOnlyServices": bson.M{"ReshardingDonorService": bson.M{"state": "new-state"}}},
	}
	data, err := bson.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	collector := &rawModuleCollector{module: New(), raw: bson.Raw(data)}
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"evilCommand", "$evilStage", "$evilOperator", "brotli", "unbounded", "PLAIN", "new-state"}
	contract := loadPolicyContract(t)
	ceilings := map[string]int{}
	for _, entry := range contract.Entries {
		if entry.Metric != nil {
			ceilings[entry.Metric.Family] = entry.Metric.SeriesCeiling
		}
	}
	for _, family := range families {
		if limit, ok := ceilings[family.GetName()]; ok && len(family.Metric) > limit {
			t.Errorf("%s emitted %d series above ceiling %d", family.GetName(), len(family.Metric), limit)
		}
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				for _, value := range forbidden {
					if label.GetValue() == value {
						t.Errorf("%s emitted forbidden label value %q", family.GetName(), value)
					}
				}
			}
		}
		for _, value := range forbidden {
			if strings.Contains(family.GetName(), value) {
				t.Errorf("unknown key created metric family %s", family.GetName())
			}
		}
	}
}
