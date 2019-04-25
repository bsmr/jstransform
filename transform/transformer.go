// Package transform implements code which can use a JSON schema with transform sections to convert a JSON file to
// match the schema format.
package transform

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/GannettDigital/jstransform/jsonschema"
)

// JSONTransformer - a type implemented by the jstransform.Transformer
type Transformer interface {
	Transform(raw []byte) (json.RawMessage, error)
}

func NewTransformer(schema *jsonschema.Schema, tranformIdentifier string) (Transformer, error) {
	return NewJSONTransformer(schema, tranformIdentifier)
}

// saveInTree is used recursively to add values the tree based on the path even if the parents are nil.
func saveInTree(tree map[string]interface{}, path string, value interface{}) error {
	if value == nil {
		return nil
	}

	splits := strings.Split(path, ".")
	if splits[0] == "$" {
		path = path[2:]
		splits = splits[1:]
	}

	if len(splits) == 1 {
		return saveLeaf(tree, splits[0], value)
	}

	arraySplits := strings.Split(splits[0], "[")
	if len(arraySplits) != 1 { // the case of an array or nested arrays with an object in them
		var sValue []interface{}
		if rawSlice, ok := tree[arraySplits[0]]; ok {
			sValue = rawSlice.([]interface{})
		}

		newTreeMap := make(map[string]interface{})
		newValue, err := saveInSlice(sValue, arraySplits[1:], newTreeMap)
		if err != nil {
			return err
		}

		tree[arraySplits[0]] = newValue
		return saveInTree(newTreeMap, strings.Join(splits[1:], "."), value)
	}

	var newTreeMap map[string]interface{}
	newTree, ok := tree[splits[0]]
	if !ok || newTree == nil {
		newTreeMap = make(map[string]interface{})
	} else {
		newTreeMap, ok = newTree.(map[string]interface{})
		if !ok {
			return fmt.Errorf("value at %q is not a map[string]interface{}", splits[0])
		}
	}
	tree[splits[0]] = newTreeMap
	return saveInTree(newTreeMap, strings.Join(splits[1:], "."), value)
}

// saveLeaf will save a leaf value in the tree at the given path. If the path specifies an array or set of nested
// arrays it will build the array items as needed to reach the specified index. New array items are created as nil.
// Any nested array items will be recursively treated the same way.
func saveLeaf(tree map[string]interface{}, path string, value interface{}) error {
	arraySplits := strings.Split(path, "[")
	if len(arraySplits) == 1 {
		tree[path] = value
		return nil
	}

	var sValue []interface{}
	if rawSlice, ok := tree[arraySplits[0]]; ok {
		sValue = rawSlice.([]interface{})
	}

	newValue, err := saveInSlice(sValue, arraySplits[1:], value)
	if err != nil {
		return err
	}
	tree[arraySplits[0]] = newValue
	return nil
}

func saveInSlice(current []interface{}, arraySplits []string, value interface{}) ([]interface{}, error) {
	index, err := strconv.Atoi(strings.Trim(arraySplits[0], "]"))
	if err != nil {
		return nil, fmt.Errorf("failed to determine index of %q", arraySplits[0])
	}

	if current == nil {
		current = make([]interface{}, 0, index)
	}

	// fill up the slice slots with nil if the slice isn't the right size
	for j := len(current); j <= index; j++ {
		current = append(current, nil)
	}

	if len(arraySplits) == 1 {
		// if this is the last array split save the value and break
		if newValue, ok := value.(map[string]interface{}); ok { // special case combine existing values into new value if a map
			if oldValue, ok := current[index].(map[string]interface{}); ok {
				for k, v := range oldValue {
					if _, ok := newValue[k]; !ok {
						newValue[k] = v
					}
				}
				value = newValue
			}
		}
		current[index] = value
		return current, nil
	}

	// recurse as needed
	nested, ok := current[index].([]interface{})
	if !ok {
		nested = nil
	}

	newValue, err := saveInSlice(nested, arraySplits[1:], value)
	current[index] = newValue
	return current, nil
}
