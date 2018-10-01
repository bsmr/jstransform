package transform

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/GannettDigital/jstransform/jsonschema"
)

// used for the Transformer test and benchmark
var (
	imageSchema, _           = jsonschema.SchemaFromFile("./test_data/image.json", "")
	arrayTransformsSchema, _ = jsonschema.SchemaFromFile("./test_data/array-transforms.json", "")
	doublearraySchema, _     = jsonschema.SchemaFromFile("./test_data/double-array.json", "")
	operationsSchema, _      = jsonschema.SchemaFromFile("./test_data/operations.json", "")
	dateTimesSchema, _       = jsonschema.SchemaFromFile("./test_data/date-times.json", "")

	transformerTests = []struct {
		description         string
		schema              *jsonschema.Schema
		transformIdentifier string
		in                  json.RawMessage
		want                json.RawMessage
		wantErr             bool
	}{
		{
			description:         "Use basic transforms, copy from input and default to build result",
			schema:              imageSchema,
			transformIdentifier: "cumulo",
			in: json.RawMessage(`
							{
								"type": "image",
								"crops": [
									{
										"height": 0,
										"path": "path",
										"relativePath": "",
										"width": 0
									},
									{
										"name": "aname",
										"height": 0,
										"path": "empty",
										"relativePath": "empty",
										"width": 0
									}
								],
								"publishUrl": "publishURL",
								"absoluteUrl": "absoluteURL"
							}`),
			want: json.RawMessage(`{"URL":{"absolute":"absoluteURL","publish":"publishURL"},"crops":[{"height":0,"name":"name","path":"path","relativePath":"","width":0},{"height":0,"name":"aname","path":"empty","relativePath":"empty","width":0}],"type":"image"}`),
		},
		{
			description:         "Input too simple, fails validation",
			schema:              imageSchema,
			transformIdentifier: "cumulo",
			in: json.RawMessage(`
							{
								"type": "image",
								"crops": [
									{
										"path": "path"
									},
									{
										"name": "aname",
										"relativePath": "empty"
									}
								],
								"publishUrl": "publishURL",
								"absoluteUrl": "absoluteURL"
							}`),
			want:    json.RawMessage(`{"URL":{"absolute":"absoluteURL","publish":"publishURL"},"crops":[{"name":"name","path":"path"},{"name":"aname","relativePath":"empty"}],"type":"image"}`),
			wantErr: true,
		},
		{
			description:         "Array transforms, tests arrays with string type and with a single object type",
			schema:              arrayTransformsSchema,
			transformIdentifier: "cumulo",
			in: json.RawMessage(`
							{
								"type": "image",
								"data": {
									"contributors": [
										{"id": 1, "fullname": "one"},
										{"id": 2, "fullname": "two"}
									],
									"lines": [
										"line1",
										"line2"
									]
								},
								"aSingleObject": [
									{
										"id": 1,
										"name": "test1"
									}
								]
							}`),
			want: json.RawMessage(`{"contributors":[{"id":"1","name":"one"},{"id":"2","name":"two"}],"lines":["line1","line2"],"wasSingleObject":[{"id":"1","name":"test1"}]}`),
		},
		{
			description:         "Test all operations",
			schema:              operationsSchema,
			transformIdentifier: "cumulo",
			in: json.RawMessage(`
						{
							"type": "image",
							"data": {
								"attributes": [
									{
										"name": "length",
										"value": "00:13"
									}
								],
								"contributors": [
									{"id": 1, "fullname": "one"},
									{"id": 2, "fullname": "two"}
								]
							},
							"mixedCase": "a|B|c|D",
							"invalid": false,
							"url": "http://foo.com/blah"
						}`),
			want: json.RawMessage(`{"caseSplit":["a","b","c","d"],"contributor":"two","duration":13,"url":"http://gannettdigital.com/blah","valid":true}`),
		},
		{
			description:         "Test empty non-required object",
			schema:              imageSchema,
			transformIdentifier: "cumulo",
			in: json.RawMessage(`
						{
							"type": "image",
							"crops": [
								{
									"height": 0,
									"path": "path",
									"relativePath": "",
									"width": 0
								},
								{
									"name": "aname",
									"height": 0,
									"path": "empty",
									"relativePath": "empty",
									"width": 0
								}
							]
						}`),
			want: json.RawMessage(`{"crops":[{"height":0,"name":"name","path":"path","relativePath":"","width":0},{"height":0,"name":"aname","path":"empty","relativePath":"empty","width":0}],"type":"image"}`),
		},
		{
			description:         "Test empty non-required array",
			schema:              arrayTransformsSchema,
			transformIdentifier: "cumulo",
			in: json.RawMessage(`
						{
							"type": "image",
							"data": {
								"lines": [
									"line1",
									"line2"
								]
							},
							"aSingleObject": [
								{
									"id": 1,
									"name": "test1"
								}
							]
						}`),
			want: json.RawMessage(`{"lines":["line1","line2"],"wasSingleObject":[{"id":"1","name":"test1"}]}`),
		},
		{
			description:         "Test nested arrays",
			schema:              doublearraySchema,
			transformIdentifier: "cumulo",
			in: json.RawMessage(`
				{
					"data" : {
						"double": [
							["1-1", "1-2"],
							["2-1", "2-2"]
						]
					},
					"array1": [
						{
							"name": "array1-1",
							"array2": [
								{
									"name": "array1-1-1"
								},
								{
									"name": "array1-1-2"
								}
							]
						},
						{
							"name": "array1-2",
							"array2": [
								{
									"name": "array1-2-1"
								}
							]
						}
					]
				}`),
			want: json.RawMessage(`{"array1":[{"array2":[{"level2Name":"array1-1-1"},{"level2Name":"array1-1-2"}],"level1Name":"array1-1"},{"array2":[{"level2Name":"array1-2-1"}],"level1Name":"array1-2"}],"double":[["1-1","1-2"],["2-1","2-2"]]}`),
		},
		{
			description:         "Test format: date-time strings",
			schema:              dateTimesSchema,
			transformIdentifier: "cumulo",
			in: json.RawMessage(`
				{
					"dates": [
						1529958073,
						"2018-06-25T20:21:13Z"
					],
					"requiredDate": "2018-06-25T20:21:13Z",
					"optionalDate": ""
				}`),
			want: json.RawMessage(`{"dates":["2018-06-25T20:21:13Z","2018-06-25T20:21:13Z"],"requiredDate":"2018-06-25T20:21:13Z"}`),
		},
	}

	saveValueTests = []struct {
		description string
		tree        map[string]interface{}
		jsonPath    string
		value       interface{}
		want        map[string]interface{}
		wantErr     bool
	}{
		{
			description: "Simple string value at empty root",
			tree:        make(map[string]interface{}),
			jsonPath:    "$.test",
			value:       "string",
			want:        map[string]interface{}{"test": "string"},
		},
		{
			description: "nil value",
			tree:        make(map[string]interface{}),
			jsonPath:    "$.test",
			value:       nil,
			want:        map[string]interface{}{},
		},
		{
			description: "Simple string value at existing root",
			tree:        map[string]interface{}{"test1": 1},
			jsonPath:    "$.test",
			value:       "string",
			want:        map[string]interface{}{"test": "string", "test1": 1},
		},
		{
			description: "Simple string value overwriting existing value",
			tree:        map[string]interface{}{"test1": 1},
			jsonPath:    "$.test1",
			value:       "string",
			want:        map[string]interface{}{"test1": "string"},
		},
		{
			description: "Simple string value non-existent parent",
			tree:        map[string]interface{}{},
			jsonPath:    "$.test1.test2",
			value:       "string",
			want:        map[string]interface{}{"test1": map[string]interface{}{"test2": "string"}},
		},
		{
			description: "Simple int value at empty root",
			tree:        make(map[string]interface{}),
			jsonPath:    "$.test",
			value:       1,
			want:        map[string]interface{}{"test": 1},
		},
		{
			description: "New Map at empty root",
			tree:        map[string]interface{}{},
			jsonPath:    "$.test1",
			value:       map[string]interface{}{},
			want:        map[string]interface{}{"test1": map[string]interface{}{}},
		},
		{
			description: "Map with values at empty root",
			tree:        map[string]interface{}{},
			jsonPath:    "$.test1",
			value:       map[string]interface{}{"testA": "a"},
			want:        map[string]interface{}{"test1": map[string]interface{}{"testA": "a"}},
		},
		{
			description: "Save new value in existing Map",
			tree:        map[string]interface{}{"test1": map[string]interface{}{"testA": "a"}},
			jsonPath:    "$.test1.testB",
			value:       "B",
			want:        map[string]interface{}{"test1": map[string]interface{}{"testA": "a", "testB": "B"}},
		},
		{
			description: "Array at empty root",
			tree:        map[string]interface{}{},
			jsonPath:    "$.test1",
			value:       []interface{}{"a", "b"},
			want:        map[string]interface{}{"test1": []interface{}{"a", "b"}},
		},
		{
			description: "Save value in existing Array",
			tree:        map[string]interface{}{"test1": []interface{}{"a", "b"}},
			jsonPath:    "$.test1[2]",
			value:       "c",
			want:        map[string]interface{}{"test1": []interface{}{"a", "b", "c"}},
		},
		{
			description: "Save value in new Array of objects",
			tree:        map[string]interface{}{},
			jsonPath:    "$.test1[0].a",
			value:       "aValue",
			want:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": "aValue"}}},
		},
		{
			description: "Save value in existing Array of objects",
			tree:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": "aValue"}}},
			jsonPath:    "$.test1[0].b",
			value:       "bValue",
			want:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": "aValue", "b": "bValue"}}},
		},
		{
			description: "Save new array item value in existing Array of objects",
			tree:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": "aValue"}}},
			jsonPath:    "$.test1[1].a",
			value:       "a2ndValue",
			want:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": "aValue"}, map[string]interface{}{"a": "a2ndValue"}}},
		},
		{
			description: "Save new array item simple nested Array",
			tree:        map[string]interface{}{},
			jsonPath:    "$.test1[0][1]",
			value:       "nestedValue",
			want:        map[string]interface{}{"test1": []interface{}{[]interface{}{nil, "nestedValue"}}},
		},
		{
			description: "Save new array item new nested Array",
			tree:        map[string]interface{}{},
			jsonPath:    "$.test1[0].a[1]",
			value:       "nestedValue",
			want:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": []interface{}{nil, "nestedValue"}}}},
		},
		{
			description: "Save object field in new nested Array",
			tree:        map[string]interface{}{},
			jsonPath:    "$.test1[0].a[1].name",
			value:       "nestedName",
			want:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": []interface{}{nil, map[string]interface{}{"name": "nestedName"}}}}},
		},
		{
			description: "Save new array item existing nested Array",
			tree:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": []interface{}{"existingValue"}}}},
			jsonPath:    "$.test1[0].a[1]",
			value:       "nestedValue",
			want:        map[string]interface{}{"test1": []interface{}{map[string]interface{}{"a": []interface{}{"existingValue", "nestedValue"}}}},
		},
	}
)

func TestSaveValue(t *testing.T) {
	for _, test := range saveValueTests {
		err := saveInTree(test.tree, test.jsonPath, test.value)

		switch {
		case test.wantErr && err != nil:
			continue
		case test.wantErr && err == nil:
			t.Errorf("Test %q - got nil, want error", test.description)
		case !test.wantErr && err != nil:
			t.Errorf("Test %q - got error, want nil: %v", test.description, err)
		case !reflect.DeepEqual(test.tree, test.want):
			t.Errorf("Test %q - got %v, want %v", test.description, test.tree, test.want)
		}
	}
}

func TestTransformer(t *testing.T) {
	for _, test := range transformerTests {
		tr, err := NewTransformer(test.schema, test.transformIdentifier)
		if err != nil {
			t.Fatalf("Test %q - failed to initialize transformer: %v", test.description, err)
		}

		got, err := tr.Transform(test.in)

		switch {
		case test.wantErr && err != nil:
			continue
		case test.wantErr && err == nil:
			t.Errorf("Test %q - got nil, want error", test.description)
		case !test.wantErr && err != nil:
			t.Errorf("Test %q - got error, want nil: %v", test.description, err)
		case !reflect.DeepEqual(got, test.want):
			t.Errorf("Test %q - got\n%s\nwant\n%s", test.description, got, test.want)
		}
	}
}

func BenchmarkTransformer(b *testing.B) {
	for _, test := range transformerTests {
		if test.wantErr {
			continue
		}

		tr, err := NewTransformer(test.schema, test.transformIdentifier)
		if err != nil {
			b.Fatalf("Test %q - failed to initialize transformer: %v", test.description, err)
		}

		b.Run(test.description, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := tr.Transform(test.in)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSaveInTree(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, test := range saveValueTests {
			err := saveInTree(test.tree, test.jsonPath, test.value)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
