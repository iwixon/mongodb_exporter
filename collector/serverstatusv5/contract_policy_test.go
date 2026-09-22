package serverstatusv5

import (
	"io/ioutil"
	"testing"
)

func loadPolicyContract(t *testing.T) *Contract {
	t.Helper()
	data, err := ioutil.ReadFile("contract.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := ParseContract(data)
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func TestContractMappingsOmissionsVocabulariesAndCeilings(t *testing.T) {
	if err := loadPolicyContract(t).ValidatePolicy(); err != nil {
		t.Fatal(err)
	}
}

func TestContractCeilingTestDetectsLowerLimit(t *testing.T) {
	contract := loadPolicyContract(t)
	contract.SeriesCeilings.Union--
	if err := contract.ValidatePolicy(); err == nil {
		t.Fatal("lower union ceiling was accepted")
	}
}
