package jsonschema

import (
	"errors"
	"io/ioutil"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		description string
		schemaPath  string
		jsonPath    string
		wantValid   bool
		wantErr     error
	}{
		{
			description: "valid image JSON",
			schemaPath:  "test_data/image.json",
			jsonPath:    "test_data/imageRaw.json",
			wantValid:   true,
		},
		{
			description: "valid image JSON using parent schema",
			schemaPath:  "test_data/parent.json",
			jsonPath:    "test_data/imageRaw.json",
			wantValid:   true,
		},
		{
			description: "invalid image JSON",
			schemaPath:  "test_data/image.json",
			jsonPath:    "test_data/embed.json",
			wantValid:   false,
			wantErr:     errors.New("invalid schema: [URL: URL is required caption: caption is required credit: credit is required crops: crops is required datePhotoTaken: datePhotoTaken is required orientation: orientation is required originalSize: originalSize is required type: type must be one of the following: \"image\"]"),
		},
		{
			description: "nil pointer in a JSON array doesn't crash",
			schemaPath:  "test_data/nil-in-a-slice.json",
			jsonPath:    "test_data/tag_Iowa_Star.json",
			wantValid:   true,
		},
	}
	for _, test := range tests {
		v, err := SchemaFromFile(test.schemaPath, "")
		if err != nil {
			t.Fatalf("Test %q - failed to load schema: %v", test.description, err)
		}
		raw, err := ioutil.ReadFile(test.jsonPath)
		if err != nil {
			t.Fatalf("Test %q - failed to load test JSON: %v", test.description, err)
		}

		got, err := v.Validate(raw)
		if err != nil && test.wantErr != nil {
			if err.Error() != test.wantErr.Error() {
				t.Errorf("Test %q - got error %s, want error %s", test.description, err.Error(), test.wantErr.Error())
			}
		} else if err != nil {
			t.Errorf("Test %q - got error %s, want error %s", test.description, err.Error(), test.wantErr)
		}
		if got != test.wantValid {
			t.Errorf("Test %q - got valid %t, want %t", test.description, got, test.wantValid)
		}
	}
}
