package serverstatusv5

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/percona/mongodb_exporter/collector/serverstatusv5/fixturetool"
)

func loadCoverageInputs(t *testing.T) (*Contract, []string) {
	t.Helper()
	contractData, err := ioutil.ReadFile("contract.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := ParseContract(contractData)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, name := range []string{"server-status-primary-5.0.34.json", "server-status-secondary-5.0.34.json"} {
		data, err := ioutil.ReadFile(filepath.Join("testdata", "source", name))
		if err != nil {
			t.Fatal(err)
		}
		var value interface{}
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		for _, path := range fixturetool.SortedLeafPaths(value) {
			paths[path] = true
		}
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	return contract, result
}

func TestCoverageContractClassifiesFixtureUnionExactlyOnce(t *testing.T) {
	contract, paths := loadCoverageInputs(t)
	if err := contract.Audit(paths); err != nil {
		t.Fatal(err)
	}
}
func TestCoverageAuditRejectsMissingClassification(t *testing.T) {
	contract, paths := loadCoverageInputs(t)
	contract.Entries = contract.Entries[1:]
	if err := contract.Audit(paths); err == nil || !strings.Contains(err.Error(), "missing=") {
		t.Fatalf("got %v", err)
	}
}
func TestCoverageAuditRejectsDuplicateClassification(t *testing.T) {
	contract, paths := loadCoverageInputs(t)
	contract.Entries = append(contract.Entries, contract.Entries[0])
	if err := contract.Audit(paths); err == nil || !strings.Contains(err.Error(), "duplicate=") {
		t.Fatalf("got %v", err)
	}
}
func TestCoverageAuditRejectsStaleClassification(t *testing.T) {
	contract, paths := loadCoverageInputs(t)
	contract.Entries = append(contract.Entries, ContractEntry{Path: "not.present", Classification: "metadata_omitted"})
	if err := contract.Audit(paths); err == nil || !strings.Contains(err.Error(), "stale=") {
		t.Fatalf("got %v", err)
	}
}
