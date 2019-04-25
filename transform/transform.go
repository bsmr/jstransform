package transform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/antchfx/xmlquery"
)

type transformMethod int32

const (
	first transformMethod = iota
	last
	concatenate
)

// transformOperation defines the interface for operations that are implemented within the transform schema.
type transformOperation interface {
	init(args map[string]string) error
	transform(in interface{}) (interface{}, error)
}

type transformOperationJSON struct {
	Name string            `json:"type"`
	Args map[string]string `json:"args"`
}

type transformInstructionXML struct {
	// For JSONPath format see http://goessner.net/articles/JsonPath/
	xmlPath    string
	Operations []transformOperation `json:"operations"`
}

type transformInstructionJSON struct {
	XmlPath    string                   `json:"xmlPath"`
	Operations []transformOperationJSON `json:"operations"`
}

// UnmarshalJSON implements the json.Unmarshaler interface, this function exists to properly map the transformOperation.
func (ti *transformInstructionXML) UnmarshalJSON(data []byte) error {
	var jti transformInstructionJSON

	if err := json.Unmarshal(data, &jti); err != nil {
		return fmt.Errorf("failed to extract transform from JSON: %v", err)
	}

	ti.xmlPath = jti.XmlPath
	ti.Operations = []transformOperation{}

	for _, toj := range jti.Operations {
		var op transformOperation
		switch toj.Name {
		case "changeCase":
			op = &changeCase{}
		case "duration":
			op = &duration{}
		case "inverse":
			op = &inverse{}
		case "max":
			op = &max{}
		case "replace":
			op = &replace{}
		case "split":
			op = &split{}
		default:
			return fmt.Errorf("unsupported operation %q", toj.Name)
		}

		if err := op.init(toj.Args); err != nil {
			return fmt.Errorf("failed initializing transform operation: %v", err)
		}
		ti.Operations = append(ti.Operations, op)
	}
	return nil
}

// transform runs the instructions in this object returning the new transformed value or an error if unable to.
// It handles the logic for finding the value to be transformed and chaining the Operations.
// It will not error if the value is not found, rather it returns nil for the value.
// If a conversion or operation fails an error is returned.
func (ti *transformInstructionXML) transform(in interface{}, fieldType string, modifier pathModifier) (interface{}, error) {
	path := ti.xmlPath
	if modifier != nil {
		path = modifier(path)
	}

	node := in.(*xmlquery.Node)
	rawValue := xmlquery.Find(node, path)

	if rawValue == nil {
		return nil, nil
	}

	value, err := convert(rawValue, fieldType)
	if err != nil {
		// In some cases the conversion is helpful but in others like before a max operation it isn't
		value = rawValue
	}
	if value == nil {
		return nil, nil
	}

	for _, op := range ti.Operations {
		value, err = op.transform(value)
		if err != nil {
			return nil, fmt.Errorf("failed operation on value from JSONPath %q: %v", path, err)
		}
	}
	return value, nil
}

// trransformInstructions defines a set of instructions and a method for combining their results.
// The default method is to take the first non-nil result.
type transformInstructions struct {
	From          []*transformInstructionXML `json:"from"`
	Method        transformMethod         `json:"method"`
	MethodOptions methodOptions           `json:"methodOptions"`
}

type transformInstructionsJSON struct {
	From          []*transformInstructionXML `json:"from"`
	Method        string                  `json:"method"`
	MethodOptions methodOptions           `json:"methodOptions"`
}

type methodOptions struct {
	ConcatenateDelimiter string `json:"concatenateDelimiter"`
}

// UnmarshalJSON implements the json.Unmarshaler interface, this function exists to properly map the method.
func (tis *transformInstructions) UnmarshalJSON(data []byte) error {
	var jtis transformInstructionsJSON

	if err := json.Unmarshal(data, &jtis); err != nil {
		return fmt.Errorf("failed to extract transform from JSON: %v", err)
	}

	tis.From = jtis.From
	tis.MethodOptions = jtis.MethodOptions

	switch jtis.Method {
	case "":
		tis.Method = 0
	case "first":
		tis.Method = first
	case "last":
		tis.Method = last
	case "concatenate":
		tis.Method = concatenate
	default:
		return fmt.Errorf("unknown method %q", jtis.Method)
	}

	return nil
}

// transform runs the instructions in this object returning the new transformed value or nil if none is found.
// It handles the logic for concatenation, first or last methods.
func (tis *transformInstructions) transform(in interface{}, fieldType string, modifier pathModifier) (interface{}, error) {
	var concatResult bool
	switch tis.Method {
	case last:
		var newFrom []*transformInstructionXML
		for i := len(tis.From) - 1; i >= 0; i-- {
			newFrom = append(newFrom, tis.From[i])
		}
		tis.From = newFrom
	case concatenate:
		concatResult = true
	}

	var result interface{}

	for _, from := range tis.From {
		value, err := from.transform(in, fieldType, modifier)
		if err != nil {
			return nil, err
		}
		if concatResult {
			delimiter := tis.MethodOptions.ConcatenateDelimiter
			result, err = concat(result, value, delimiter)
			if err != nil {
				return nil, fmt.Errorf("failed to concat values: %v", err)
			}
			continue
		}
		if value != nil {
			result = value
			break
		}
	}

	return result, nil
}

// replaceJSONPathPrefix will switch old for new in the path of the transform instructions if the path starts with
// old.
func (tis *transformInstructions) replaceJSONPathPrefix(old, new string) {
	for _, instruction := range tis.From {
		if strings.HasPrefix(instruction.xmlPath, old) {
			instruction.xmlPath = strings.Replace(instruction.xmlPath, old, new, 1)
		}
	}
}

type transform map[string]transformInstructions
