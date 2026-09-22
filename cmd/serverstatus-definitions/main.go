package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
)

type contract struct {
	Entries []entry `json:"entries"`
}
type entry struct {
	Path           string  `json:"path"`
	Classification string  `json:"classification"`
	Metric         *metric `json:"metric"`
}
type metric struct {
	Family        string            `json:"family"`
	Help          string            `json:"help"`
	Type          string            `json:"type"`
	Conversion    float64           `json:"conversion"`
	Labels        map[string]string `json:"labels"`
	ValueLabel    string            `json:"value_label"`
	AllowedValues []string          `json:"allowed_values"`
}

func quote(s string) string { return strconv.Quote(s) }
func render(c contract) []byte {
	var b bytes.Buffer
	b.WriteString("// Code generated from contract.json; DO NOT EDIT.\n\npackage serverstatusv5\n\nvar metricDefinitions = []metricDefinition{\n")
	for _, e := range c.Entries {
		if e.Classification != "new_metric" || e.Metric == nil {
			continue
		}
		m := e.Metric
		b.WriteString("\t{Path: " + quote(e.Path) + ", Family: " + quote(m.Family) + ", Help: " + quote(m.Help) + ", Type: " + quote(m.Type) + ", Conversion: " + strconv.FormatFloat(m.Conversion, 'g', -1, 64) + ", FixedLabels: map[string]string{")
		keys := make([]string, 0, len(m.Labels))
		for k := range m.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quote(k) + ": " + quote(m.Labels[k]))
		}
		b.WriteString("}, ValueLabel: " + quote(m.ValueLabel) + ", AllowedValues: []string{")
		for i, v := range m.AllowedValues {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quote(v))
		}
		b.WriteString("}},\n")
	}
	b.WriteString("}\n")
	return b.Bytes()
}
func main() {
	output := flag.String("output", "collector/serverstatusv5/definitions_generated.go", "generated Go file")
	check := flag.Bool("check", false, "fail when output differs")
	contractPath := flag.String("contract", "collector/serverstatusv5/contract.json", "coverage contract")
	flag.Parse()
	data, err := ioutil.ReadFile(*contractPath)
	if err != nil {
		panic(err)
	}
	var c contract
	if err := json.Unmarshal(data, &c); err != nil {
		panic(err)
	}
	generated := render(c)
	if *check {
		current, err := ioutil.ReadFile(*output)
		if err != nil || !bytes.Equal(current, generated) {
			fmt.Fprintln(os.Stderr, "definitions_generated.go is stale; run serverstatus-definitions")
			os.Exit(1)
		}
		return
	}
	if err := ioutil.WriteFile(*output, generated, 0644); err != nil {
		panic(err)
	}
}
