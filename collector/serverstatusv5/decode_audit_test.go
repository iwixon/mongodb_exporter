package serverstatusv5

import (
	"fmt"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

func auditDefinitionDecode(raws []bson.Raw, definitions []metricDefinition) error {
	for _, definition := range definitions {
		found := false
		for _, raw := range raws {
			value := raw.Lookup(strings.Split(definition.Path, ".")...)
			if value.Type == 0 || value.Type == bsontype.Null {
				continue
			}
			found = true
			if _, err := metricValue(value, definition, map[string]string{}); err != nil {
				return fmt.Errorf("%s: %w", definition.Path, err)
			}
		}
		if !found {
			return fmt.Errorf("exported contract path %s is absent from all fixtures", definition.Path)
		}
	}
	return nil
}

func TestEveryExportedContractPathDecodesFromFixture(t *testing.T) {
	raws := []bson.Raw{loadFixtureRaw(t, "primary"), loadFixtureRaw(t, "secondary")}
	if err := auditDefinitionDecode(raws, metricDefinitions); err != nil {
		t.Fatal(err)
	}
}
func TestDecodeAuditRejectsFixtureShapeChange(t *testing.T) {
	definitions := append([]metricDefinition(nil), metricDefinitions...)
	definitions[0].Path = "shape.changed"
	if err := auditDefinitionDecode([]bson.Raw{loadFixtureRaw(t, "primary"), loadFixtureRaw(t, "secondary")}, definitions); err == nil {
		t.Fatal("shape change was accepted")
	}
}
