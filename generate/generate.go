// Package generate implements a tooling to generate Golang structs from a JSON schema file.
// It is intended to be used with the go generate, https://blog.golang.org/generate
package generate

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GannettDigital/jstransform/jsonschema"
	"github.com/tinylib/msgp/gen"
	"github.com/tinylib/msgp/parse"
	"github.com/tinylib/msgp/printer"
)

const disclaimer = "// Code generated by github.com/GannettDigital/jstransform; DO NOT EDIT."
const msgpSuffix = "_msgp"
const msgpMode = gen.Encode | gen.Decode | gen.Marshal | gen.Unmarshal | gen.Size | gen.Test

// BuildStructs takes a JSON Schema and generates Golang structs that match the schema.
// The structs include struct tags for marshaling/unmarshaling to/from JSON.
// One file will be created for each included allOf/oneOf file in the root schema with any allOf files resulting in
// structs which are embedded in the oneOf files.
//
// The JSON schema can specify more information than the structs enforce (like field size) and so validation of
// any JSON generated from the structs is still necessary.
//
// If undefined outputPath defaults to the current working directory.
//
// The package name is set to the outputPath directory name.
//
// NOTE: If oneOf/allOf entries exist than any JSON schema instances in the root schema file will be skipped.
func BuildStructs(schemaPath string, outputDir string, useMessagePack bool) error {
	if outputDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine working directory: %v", err)
		}
		outputDir = wd
	}

	packageName := filepath.Base(outputDir)

	allOfTypes, oneOfTypes, err := jsonschema.SchemaTypes(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to discover oneOfTypes: %v", err)
	}

	if len(allOfTypes) == 0 && len(oneOfTypes) == 0 {
		path, err := filepath.Abs(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to determine absolute path of %q: %v", schemaPath, err)
		}
		allOfTypes = append(allOfTypes, path)
	}

	var embeds []string
	for _, allOfPath := range allOfTypes {
		name := strings.Split(filepath.Base(allOfPath), ".")[0]
		embeds = append(embeds, exportedName(name))

		if err := buildStructFile(schemaPath, allOfPath, name, packageName, nil, outputDir); err != nil {
			return fmt.Errorf("failed to build struct file for %q: %v", name, err)
		}
	}

	for _, oneOfPath := range oneOfTypes {
		name := strings.Split(filepath.Base(oneOfPath), ".")[0]

		if err := buildStructFile(schemaPath, oneOfPath, name, packageName, embeds, outputDir); err != nil {
			return fmt.Errorf("failed to build struct file for %q: %v", name, err)
		}
	}

	if useMessagePack {
		if err := buildMessagePackFile(outputDir, msgpMode); err != nil {
			return fmt.Errorf("failed to build MessagePack file for %q: %v", packageName, err)
		}
	}

	return nil
}

// buildMessagePackFile generates MessagePack serialization methods for the entire package.
func buildMessagePackFile(outputDir string, mode gen.Method) error {
	fs, err := parse.File(outputDir, false)
	if err != nil {
		return err
	}

	if len(fs.Identities) == 0 {
		return nil
	}

	return printer.PrintFile(filepath.Join(outputDir, fs.Package+msgpSuffix+".go"), fs, mode)
}

// buildStructFile generates the specified struct file.
func buildStructFile(schemaPath, childPath, name, packageName string, embeds []string, outputDir string) error {
	if !filepath.IsAbs(childPath) {
		childPath = filepath.Join(filepath.Dir(schemaPath), childPath)
	}
	schema, err := jsonschema.SchemaFromFile(childPath, name)
	if err != nil {
		return err
	}

	generated, err := newGeneratedStruct(schema, name, packageName, embeds)
	if err != nil {
		return fmt.Errorf("failed to build generated struct: %v", err)
	}

	outPath := filepath.Join(outputDir, name+".go")
	gfile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %v", outPath, err)
	}
	defer gfile.Close()

	return generated.write(gfile)
}

// extractedField represents a Golang struct field as extracted from a JSON schema file. It is an intermediate format
// that is populated while parsing the JSON schema file then used when generating the Golang code for the struct.
type extractedField struct {
	array          bool
	fields         extractedFields
	jsonName       string
	jsonType       string
	name           string
	requiredFields map[string]bool
}

// write outputs the Golang representation of this field to the writer with prefix before each line.
// It handles inline structs by calling this method recursively adding a new \t to the prefix for each layer.
// If required is set to false 'omitempty' is added in the JSON struct tag for the field
func (ef *extractedField) write(w io.Writer, prefix string, required bool) error {
	structTag := "`"
	structTag = structTag + fmt.Sprintf(`json:"%s`, ef.jsonName)
	if !required {
		structTag = structTag + ",omitempty"
	}
	structTag = structTag + `"` + "`\n"

	if ef.jsonType != "object" {
		_, err := w.Write([]byte(fmt.Sprintf("%s%s\t%s\t%s", prefix, ef.name, goType(ef.jsonType, ef.array), structTag)))
		return err
	}

	if _, err := w.Write([]byte(fmt.Sprintf("%s%s\t%s {\n", prefix, ef.name, goType(ef.jsonType, ef.array)))); err != nil {
		return err
	}

	for _, field := range ef.fields.Sorted() {
		fieldRequired := ef.requiredFields[field.jsonName]
		if err := field.write(w, prefix+"\t", fieldRequired); err != nil {
			return fmt.Errorf("failed writing field %q: %v", field.name, err)
		}
	}

	if _, err := w.Write([]byte(fmt.Sprintf("%s\t}\t%s", prefix, structTag))); err != nil {
		return err
	}
	return nil
}

// extractedFields is a map of fields keyed on the field name.
type extractedFields map[string]*extractedField

// IncludeTime does a depth-first recursive search to see if any field or child field is of type "date-time"
func (efs extractedFields) IncludeTime() bool {
	for _, field := range efs {
		if field.fields != nil {
			if field.fields.IncludeTime() {
				return true
			}
		}
		if field.jsonType == "date-time" {
			return true
		}
	}
	return false
}

// Sorted will return the fields in a sorted list. The sort is a string sort on the keys
func (efs extractedFields) Sorted() []*extractedField {
	var sorted []*extractedField
	var sortedKeys sort.StringSlice
	fieldsByName := make(map[string]*extractedField)
	for _, f := range efs {
		sortedKeys = append(sortedKeys, f.name)
		fieldsByName[f.name] = f
	}

	sortedKeys.Sort()

	for _, key := range sortedKeys {
		sorted = append(sorted, fieldsByName[key])
	}

	return sorted
}

type generatedStruct struct {
	extractedField

	packageName    string
	embededStructs []string
}

func newGeneratedStruct(schema *jsonschema.Schema, name, packageName string, embeds []string) (*generatedStruct, error) {
	required := map[string]bool{}
	for _, fname := range schema.Required {
		required[fname] = true
	}
	generated := &generatedStruct{extractedField: extractedField{
		name:           name,
		fields:         make(map[string]*extractedField),
		requiredFields: required,
	},
		embededStructs: embeds,
		packageName:    packageName,
	}
	if err := jsonschema.Walk(schema, generated.walkFunc); err != nil {
		return nil, fmt.Errorf("failed to walk schema for %q: %v", name, err)
	}

	return generated, nil
}

// walkFunc is a jsonschema.WalkFunc which builds the fields in the generatedStructFile as the JSON schema file is
// walked.
func (gen *generatedStruct) walkFunc(path string, i jsonschema.Instance) error {
	if err := addField(gen.fields, splitJSONPath(path), i); err != nil {
		return err
	}
	return nil
}

// write will write the generated file to the given io.Writer.
func (gen *generatedStruct) write(w io.Writer) error {
	buf := &bytes.Buffer{} // the formatter uses the entire output, so buffer for that

	if _, err := buf.Write([]byte(fmt.Sprintf("package %s\n\n%s\n\n", gen.packageName, disclaimer))); err != nil {
		return fmt.Errorf("failed writing struct: %v", err)
	}

	if gen.fields.IncludeTime() {
		if _, err := buf.Write([]byte("import \"time\"\n")); err != nil {
			return fmt.Errorf("failed writing struct: %v", err)
		}
	}

	embeds := strings.Join(gen.embededStructs, "\n")
	if embeds != "" {
		embeds += "\n"
	}
	if _, err := buf.Write([]byte(fmt.Sprintf("type %s struct {\n%s\n", exportedName(gen.name), embeds))); err != nil {
		return fmt.Errorf("failed writing struct: %v", err)
	}

	for _, field := range gen.fields.Sorted() {
		req := gen.requiredFields[field.jsonName]
		if err := field.write(buf, "\t", req); err != nil {
			return fmt.Errorf("failed writing field %q: %v", field.name, err)
		}
	}

	if _, err := buf.Write([]byte("}")); err != nil {
		return fmt.Errorf("failed writing struct: %v", err)
	}

	final, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format source: %v", err)
	}

	if _, err := w.Write(final); err != nil {
		return fmt.Errorf("error writing to io.Writer: %v", err)
	}
	return nil
}

// addField will create a new field or add to an existing field in the extractedFields.
// Nested fields are handled by recursively calling this function until the leaf field is reached.
// For all fields the name and jsonType are set, for arrays the array bool is set for true and for JSON objects,
// the fields map is created and if it exists the requiredFields section populated.
func addField(fields extractedFields, tree []string, inst jsonschema.Instance) error {
	if len(tree) > 1 {
		if f, ok := fields[tree[0]]; ok {
			return addField(f.fields, tree[1:], inst)
		}
		f := &extractedField{jsonName: tree[0], jsonType: "object", name: exportedName(tree[0]), fields: make(map[string]*extractedField)}
		fields[tree[0]] = f
		if err := addField(f.fields, tree[1:], inst); err != nil {
			return fmt.Errorf("failed field %q: %v", tree[0], err)
		}
		return nil
	}

	f := &extractedField{
		name:     exportedName(tree[0]),
		jsonName: tree[0],
		jsonType: inst.Type,
	}
	// Second processing of an array type
	if exists, ok := fields[f.jsonName]; ok {
		f = exists
		if f.array && f.jsonType == "" {
			f.jsonType = inst.Type
		} else {
			return fmt.Errorf("field %q already exists but is not an array field", f.name)
		}
	}
	if inst.Type == "string" && inst.Format == "date-time" {
		f.jsonType = "date-time"
	}

	switch f.jsonType {
	case "array":
		f.jsonType = ""
		f.array = true
	case "object":
		f.requiredFields = make(map[string]bool)
		for _, name := range inst.Required {
			f.requiredFields[name] = true
		}
		f.fields = make(map[string]*extractedField)
	}

	fields[tree[0]] = f

	return nil
}

// exportedName returns a name that is usable as an exported field in Go.
// Only minimal checking on naming is done rather it is assumed the name from the JSON schema is reasonable any
// unacceptable names will likely fail during formatting.
func exportedName(name string) string {
	return strings.Title(name)
}

// goType maps a jsonType to a string representation of the go type.
// If Array is true it makes the type into an array.
// If the JSON Schema had a type of "string" and a format of "date-time" it is expected the input jsonType will be
// "date-time".
func goType(jsonType string, array bool) string {
	var goType string
	switch jsonType {
	case "boolean":
		goType = "bool"
	case "number":
		goType = "float64"
	case "integer":
		goType = "int64"
	case "string":
		goType = "string"
	case "date-time":
		goType = "time.Time"
	case "object":
		goType = "struct"
	}

	if array {
		return "[]" + goType
	}

	return goType
}

// splitJSONPath takes a JSON path and returns an array of path items each of which represents a JSON object with the
// name normalized in a way suitable for using it as a Go struct filed name.
func splitJSONPath(path string) []string {
	var tree []string
	for _, split := range strings.Split(path, ".") {
		if split == "$" {
			continue
		}
		if strings.HasSuffix(split, "[*]") {
			split = split[:len(split)-3]
		}

		tree = append(tree, split)
	}

	return tree
}
