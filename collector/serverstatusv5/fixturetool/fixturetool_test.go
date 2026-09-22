package fixturetool

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestReconstructionIsDeterministic(t *testing.T) {
	source, err := ioutil.ReadFile(filepath.Join("..", "testdata", "source", "server-status-primary-5.0.34.json"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := ReconstructJSON(source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReconstructJSON(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("reconstruction is not deterministic")
	}
}

func TestReconstructionRejectsIdentities(t *testing.T) {
	if _, err := ReconstructJSON([]byte("{\"host\":\"prod4.evergage.com\"}")); err == nil {
		t.Fatal("identity-bearing fixture accepted")
	}
}
