package serverstatusv5

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/prometheus/common/expfmt"
)

func renderFixtureMetrics(t *testing.T, role string) ([]byte, int) {
	t.Helper()
	families := gatherFixture(t, role)
	names := make([]string, 0, len(families))
	for name := range families {
		names = append(names, name)
	}
	sort.Strings(names)
	var output bytes.Buffer
	series := 0
	for _, name := range names {
		family := families[name]
		if _, err := expfmt.MetricFamilyToText(&output, family); err != nil {
			t.Fatal(err)
		}
		if name != "mongodb_server_status_decode_errors_total" {
			series += len(family.Metric)
		}
	}
	return output.Bytes(), series
}
func TestFixtureGoldenSamplesAndSeriesCeilings(t *testing.T) {
	contract := loadPolicyContract(t)
	for _, test := range []struct {
		role    string
		ceiling int
	}{{"primary", contract.SeriesCeilings.Primary}, {"secondary", contract.SeriesCeilings.Secondary}} {
		t.Run(test.role, func(t *testing.T) {
			actual, series := renderFixtureMetrics(t, test.role)
			if series > test.ceiling {
				t.Fatalf("series=%d exceeds ceiling=%d", series, test.ceiling)
			}
			path := filepath.Join("testdata", "golden", test.role+".prom")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := ioutil.WriteFile(path, actual, 0644); err != nil {
					t.Fatal(err)
				}
				return
			}
			expected, err := ioutil.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(expected, actual) {
				t.Fatalf("golden output differs; run UPDATE_GOLDEN=1 go test ./collector/serverstatusv5 -run TestFixtureGoldenSamplesAndSeriesCeilings")
			}
		})
	}
}
