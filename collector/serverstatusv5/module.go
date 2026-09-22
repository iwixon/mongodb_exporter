package serverstatusv5

import (
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

type metricDefinition struct {
	Path          string
	Family        string
	Help          string
	Type          string
	Conversion    float64
	FixedLabels   map[string]string
	ValueLabel    string
	AllowedValues []string
}

type FamilyError struct {
	Family string
	Path   string
	Err    error
}
type Report struct {
	Errors  []FamilyError
	Emitted int
}
type familyDescriptor struct {
	desc       *prometheus.Desc
	labelNames []string
}
type sample struct {
	definition metricDefinition
	value      float64
	labels     map[string]string
}

type Module struct {
	descriptors  map[string]familyDescriptor
	decodeErrors *prometheus.CounterVec
}

func New() *Module {
	schemas := make(map[string]map[string]bool)
	helps := make(map[string]string)
	for _, definition := range metricDefinitions {
		labels := schemas[definition.Family]
		if labels == nil {
			labels = make(map[string]bool)
			schemas[definition.Family] = labels
			helps[definition.Family] = definition.Help
		}
		for name := range definition.FixedLabels {
			labels[name] = true
		}
		if definition.ValueLabel != "" {
			labels[definition.ValueLabel] = true
		}
	}
	descriptors := make(map[string]familyDescriptor, len(schemas))
	for family, labelSet := range schemas {
		names := make([]string, 0, len(labelSet))
		for name := range labelSet {
			names = append(names, name)
		}
		sort.Strings(names)
		descriptors[family] = familyDescriptor{desc: prometheus.NewDesc(family, helps[family], names, nil), labelNames: names}
	}
	return &Module{descriptors: descriptors, decodeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: "mongodb", Subsystem: "server_status", Name: "decode_errors_total", Help: "Total modern server-status metric-family decode failures."}, []string{"family"})}
}
func (m *Module) Describe(ch chan<- *prometheus.Desc) {
	families := make([]string, 0, len(m.descriptors))
	for family := range m.descriptors {
		families = append(families, family)
	}
	sort.Strings(families)
	for _, family := range families {
		ch <- m.descriptors[family].desc
	}
	m.decodeErrors.Describe(ch)
}
func (m *Module) Collect(raw bson.Raw, ch chan<- prometheus.Metric) Report {
	grouped := make(map[string][]sample)
	failures := make(map[string]FamilyError)
	for _, definition := range metricDefinitions {
		value := raw.Lookup(strings.Split(definition.Path, ".")...)
		if value.Type == 0 || value.Type == bsontype.Null {
			continue
		}
		labels := copyLabels(definition.FixedLabels)
		numeric, err := metricValue(value, definition, labels)
		if err != nil {
			if _, exists := failures[definition.Family]; !exists {
				failures[definition.Family] = FamilyError{Family: definition.Family, Path: definition.Path, Err: err}
			}
			continue
		}
		grouped[definition.Family] = append(grouped[definition.Family], sample{definition: definition, value: numeric, labels: labels})
	}
	report := Report{}
	families := make([]string, 0, len(grouped))
	for family := range grouped {
		families = append(families, family)
	}
	sort.Strings(families)
	for _, family := range families {
		if _, failed := failures[family]; failed {
			continue
		}
		descriptor := m.descriptors[family]
		for _, candidate := range grouped[family] {
			labelValues := make([]string, len(descriptor.labelNames))
			for i, name := range descriptor.labelNames {
				labelValues[i] = candidate.labels[name]
			}
			valueType := prometheus.GaugeValue
			if candidate.definition.Type == "counter" {
				valueType = prometheus.CounterValue
			}
			ch <- prometheus.MustNewConstMetric(descriptor.desc, valueType, candidate.value, labelValues...)
			report.Emitted++
		}
	}
	failedFamilies := make([]string, 0, len(failures))
	for family := range failures {
		failedFamilies = append(failedFamilies, family)
	}
	sort.Strings(failedFamilies)
	for _, family := range failedFamilies {
		failure := failures[family]
		report.Errors = append(report.Errors, failure)
		m.decodeErrors.WithLabelValues(family).Inc()
	}
	m.decodeErrors.Collect(ch)
	return report
}

func metricValue(value bson.RawValue, definition metricDefinition, labels map[string]string) (float64, error) {
	if definition.ValueLabel != "" {
		if value.Type != bsontype.String {
			return 0, fmt.Errorf("got %s, want string", value.Type)
		}
		actual := value.StringValue()
		allowed := false
		for _, candidate := range definition.AllowedValues {
			if actual == candidate {
				allowed = true
				break
			}
		}
		if !allowed {
			return 0, fmt.Errorf("value %q is outside bounded vocabulary", actual)
		}
		labels[definition.ValueLabel] = actual
		return 1, nil
	}
	var numeric float64
	switch value.Type {
	case bsontype.Int32:
		numeric = float64(value.Int32())
	case bsontype.Int64:
		numeric = float64(value.Int64())
	case bsontype.Double:
		numeric = value.Double()
	case bsontype.Boolean:
		if value.Boolean() {
			numeric = 1
		}
	case bsontype.DateTime:
		numeric = float64(value.DateTime()) / 1000
	case bsontype.Timestamp:
		seconds, _ := value.Timestamp()
		numeric = float64(seconds)
	default:
		return 0, fmt.Errorf("got %s, want numeric, boolean, date, or timestamp", value.Type)
	}
	return numeric * definition.Conversion, nil
}
func copyLabels(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}
