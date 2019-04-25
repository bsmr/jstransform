// Package transform implements code which can use a JSON schema with transform sections to convert a JSON file to
// match the schema format.
package transform

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/GannettDigital/jstransform/jsonschema"

	"github.com/buger/jsonparser"
)

// Transformer uses a JSON schema and the transform sections within it to take a set of JSON and transform it to
// matching the schema.
// More details on the transform section of the schema are found at
// https://github.com/GannettDigital/jstransform/blob/master/transform.adoc
type JSONTransformer struct {
	schema              *jsonschema.Schema
	transformIdentifier string // Used to select the proper transform Instructions
	root                instanceTransformer
}

// NewJSONTransformer returns a JSON Transformer using the schema given.
// The transformIdentifier is used to select the appropriate transform section from the schema.
func NewJSONTransformer(schema *jsonschema.Schema, tranformIdentifier string) (Transformer, error) {
	tr := &JSONTransformer{schema: schema, transformIdentifier: tranformIdentifier}

	emptyJSON := []byte(`{}`)
	var err error
	if schema.Properties != nil {
		tr.root, err = newObjectTransformer("$", tranformIdentifier, emptyJSON)
	} else if schema.Items != nil {
		tr.root, err = newArrayTransformer("$", tranformIdentifier, emptyJSON)
	} else {
		return nil, errors.New("no Properties nor Items found for schema")
	}
	if err != nil {
		return nil, fmt.Errorf("failed initializing root transformer: %v", err)
	}

	if err := jsonschema.WalkRaw(schema, tr.walker); err != nil {
		return nil, err
	}

	return tr, nil
}

// Transform takes the provided JSON and converts the JSON to match the pre-defined JSON Schema using the transform
// sections in the schema.
//
// By default fields with no Transform section but with matching path and type are copied verbatim into the new
// JSON structure. Fields which are missing from the input are set to a default value in the output.
//
// Errors are returned for failures to perform operations but are not returned for empty fields which are either
// omitted from the output or set to an empty value.
//
// Validation of the output against the schema is the final step in the process.
func (tr *JSONTransformer) Transform(raw []byte) (json.RawMessage, error) {
	var in interface{}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("failed to parse input JSON: %v", err)
	}

	transformed, err := tr.root.transform(in, nil)
	if err != nil {
		return nil, fmt.Errorf("failed transformation: %v", err)
	}

	out, err := json.Marshal(transformed)
	if err != nil {
		return nil, fmt.Errorf("failed to JSON marsal transformed data: %v", err)
	}

	valid, err := tr.schema.Validate(out)
	if err != nil {
		return nil, fmt.Errorf("transformed result validation error: %v", err)
	}
	if !valid {
		return nil, errors.New("schema validation of the transformed result reports invalid")
	}

	return out, nil
}

// findParent walks the instanceTransformer tree to find the parent of the given path
func (tr *JSONTransformer) findParent(path string) (instanceTransformer, error) {
	path = strings.Replace(path, "[", ".[", -1)
	splits := strings.Split(path, ".")
	if splits[0] != "$" {
		// TODO this will probably choke on a root level array
		return nil, errors.New("paths must start with '$'")
	}
	parentSplits := splits[1 : len(splits)-1]

	parent := tr.root
	for _, sp := range parentSplits {
		if sp == "[*]" {
			parent = parent.child()
			continue
		}

		parent = parent.selectChild(sp)
	}

	return parent, nil
}

// walker is a WalkFunc for the Transformer which builds an representation of the fields and transforms in the schema.
// This is later used to do the actual transform for incoming data
func (tr *JSONTransformer) walker(path string, value json.RawMessage) error {
	instanceType, err := jsonparser.GetString(value, "type")
	if err != nil {
		return fmt.Errorf("failed to extract instance type: %v", err)
	}

	var iTransformer instanceTransformer
	switch instanceType {
	case "object":
		iTransformer, err = newObjectTransformer(path, tr.transformIdentifier, value)
	case "array":
		iTransformer, err = newArrayTransformer(path, tr.transformIdentifier, value)
	default:
		iTransformer, err = newScalarTransformer(path, tr.transformIdentifier, value, instanceType)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize transformer: %v", err)
	}

	parent, err := tr.findParent(path)
	if err != nil {
		return err
	}
	if err := parent.addChild(iTransformer); err != nil {
		return err
	}

	return nil
}
