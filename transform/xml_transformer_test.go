package transform

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/GannettDigital/jstransform/jsonschema"
)

func TestNewXMLTransfomer(t *testing.T) {
	schema, err := jsonschema.SchemaFromFile("teams.json", "")
	if err != nil {
		t.Fatal(err)
	}

	tr, err := NewXMLTransfomer(schema, "sport")
	if err != nil {
		t.Fatal(err)
	}

	rawXMLBytes, err := ioutil.ReadFile("teams.xml")
	if err != nil {
		t.Fatal(err)
	}

	output, err := tr.Transform(rawXMLBytes)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Sprintf("%s", output)
}