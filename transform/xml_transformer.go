package transform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/buger/jsonparser"

	"github.com/GannettDigital/jstransform/jsonschema"
)

type XMLTransformer struct {
	schema              *jsonschema.Schema
	transformIdentifier string // Used to select the proper transform Instructions
	root                instanceTransformer
}

func NewXMLTransfomer(schema *jsonschema.Schema, tranformIdentifier string) (Transformer, error) {
	tr := &XMLTransformer{schema: schema, transformIdentifier: tranformIdentifier}

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

func (tr *XMLTransformer) Transform(raw []byte) (json.RawMessage, error) {
	node, err := xmlquery.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to parse input XML: %v", err)
	}

	transformed, err := tr.root.transform(node, nil)
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

func (tr *XMLTransformer) walker(path string, value json.RawMessage) error {
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

func (tr *XMLTransformer) findParent(path string) (instanceTransformer, error) {
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
