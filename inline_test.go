package yamlinline

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	yaml "github.com/goccy/go-yaml"
)

func TestInlineYAML_Success(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		input string
		want  string
	}{
		{
			name: "scalar include",
			files: map[string]string{
				"scalar.yaml": "hello\n",
			},
			input: "value: !include scalar.yaml\n",
			want:  "value: hello\n",
		},
		{
			name: "mapping include",
			files: map[string]string{
				"config.yaml": "name: app\ncount: 2\n",
			},
			input: "config: !include config.yaml\n",
			want:  "config:\n  name: app\n  count: 2\n",
		},
		{
			name: "sequence include",
			files: map[string]string{
				"items.yaml": "- one\n- two\n",
			},
			input: "items: !include items.yaml\n",
			want:  "items:\n  - one\n  - two\n",
		},
		{
			name: "nested includes resolve from cwd",
			files: map[string]string{
				"configs/child.yaml": "leaf: !include values/item.yaml\n",
				"values/item.yaml":   "7\n",
			},
			input: "config: !include configs/child.yaml\n",
			want:  "config:\n  leaf: 7\n",
		},
		{
			name: "sequence items include field definitions",
			files: map[string]string{
				"field1.yaml": "name: title\ntype: string\n",
				"field2.yaml": "name: profile\ntype: object\nfields:\n  - name: age\n    type: integer\n",
			},
			input: "fields:\n  - !include field1.yaml\n  - !include field2.yaml\n",
			want: "fields:\n" +
				"  - name: title\n" +
				"    type: string\n" +
				"  - name: profile\n" +
				"    type: object\n" +
				"    fields:\n" +
				"      - name: age\n" +
				"        type: integer\n",
		},
		{
			name: "sequence items include field definitions with trailing whitespace",
			files: map[string]string{
				"field1.yaml": "name: title   \n" +
					"type: string   \n" +
					"   \n",
				"field2.yaml": "name: profile   \n" +
					"type: object   \n" +
					"fields:   \n" +
					"  - name: age   \n" +
					"    type: integer   \n" +
					"  - name: active   \n" +
					"    type: boolean   \n" +
					"\n",
			},
			input: "fields:\n  - !include field1.yaml\n  - !include field2.yaml\n",
			want: "fields:\n" +
				"  - name: title\n" +
				"    type: string\n" +
				"  - name: profile\n" +
				"    type: object\n" +
				"    fields:\n" +
				"      - name: age\n" +
				"        type: integer\n" +
				"      - name: active\n" +
				"        type: boolean\n",
		},
		{
			name: "nested object field includes inside fields list",
			files: map[string]string{
				"field1.yaml":                "name: profile\ntype: object\nfields:\n  - !include nested/first_name.yaml\n  - !include nested/address.yaml\n",
				"nested/first_name.yaml":     "name: firstName\ntype: string\n",
				"nested/address.yaml":        "name: address\ntype: object\nfields:\n  - !include nested/address_street.yaml\n  - !include nested/address_city.yaml\n",
				"nested/address_street.yaml": "name: street\ntype: string\n",
				"nested/address_city.yaml":   "name: city\ntype: string\n",
			},
			input: "fields:\n  - !include field1.yaml\n",
			want: "fields:\n" +
				"  - name: profile\n" +
				"    type: object\n" +
				"    fields:\n" +
				"      - name: firstName\n" +
				"        type: string\n" +
				"      - name: address\n" +
				"        type: object\n" +
				"        fields:\n" +
				"          - name: street\n" +
				"            type: string\n" +
				"          - name: city\n" +
				"            type: string\n",
		},
		{
			name: "nested list item fields include deeper definitions",
			files: map[string]string{
				"field1.yaml":                     "name: addresses\ntype: list\nitems:\n  type: object\n  fields:\n    - !include nested/street.yaml\n    - !include nested/metadata.yaml\n",
				"nested/street.yaml":              "name: street\ntype: string\n",
				"nested/metadata.yaml":            "name: metadata\ntype: object\nfields:\n  - !include nested/metadata_created_at.yaml\n  - !include nested/metadata_tags.yaml\n",
				"nested/metadata_created_at.yaml": "name: createdAt\ntype: string\n",
				"nested/metadata_tags.yaml":       "name: tags\ntype: list\nitems:\n  type: string\n",
			},
			input: "fields:\n  - !include field1.yaml\n",
			want: "fields:\n" +
				"  - name: addresses\n" +
				"    type: list\n" +
				"    items:\n" +
				"      type: object\n" +
				"      fields:\n" +
				"        - name: street\n" +
				"          type: string\n" +
				"        - name: metadata\n" +
				"          type: object\n" +
				"          fields:\n" +
				"            - name: createdAt\n" +
				"              type: string\n" +
				"            - name: tags\n" +
				"              type: list\n" +
				"              items:\n" +
				"                type: string\n",
		},
		{
			name: "nested list item fields include deeper definitions with trailing whitespace",
			files: map[string]string{
				"field1.yaml": "name: addresses   \n" +
					"type: list   \n" +
					"items:   \n" +
					"  type: object   \n" +
					"  fields:   \n" +
					"    - !include nested/street.yaml   \n" +
					"    - !include nested/metadata.yaml   \n" +
					"   \n",
				"nested/street.yaml": "name: street   \n" +
					"type: string   \n",
				"nested/metadata.yaml": "name: metadata   \n" +
					"type: object   \n" +
					"fields:   \n" +
					"  - !include nested/metadata_created_at.yaml   \n" +
					"  - !include nested/metadata_tags.yaml   \n" +
					"\n",
				"nested/metadata_created_at.yaml": "name: createdAt   \n" +
					"type: string   \n",
				"nested/metadata_tags.yaml": "name: tags   \n" +
					"type: list   \n" +
					"items:   \n" +
					"  type: string   \n" +
					"   \n",
			},
			input: "fields:\n  - !include field1.yaml\n",
			want: "fields:\n" +
				"  - name: addresses\n" +
				"    type: list\n" +
				"    items:\n" +
				"      type: object\n" +
				"      fields:\n" +
				"        - name: street\n" +
				"          type: string\n" +
				"        - name: metadata\n" +
				"          type: object\n" +
				"          fields:\n" +
				"            - name: createdAt\n" +
				"              type: string\n" +
				"            - name: tags\n" +
				"              type: list\n" +
				"              items:\n" +
				"                type: string\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)
			chdir(t, dir)

			got, err := InlineYAML([]byte(tt.input))
			if err != nil {
				t.Fatalf("InlineYAML returned error: %v", err)
			}
			if strings.Contains(string(got), "!include") {
				t.Fatalf("InlineYAML left include tags in output:\n%s", got)
			}

			assertYAMLEqual(t, got, tt.want)
		})
	}
}

func TestInlineYAML_Errors(t *testing.T) {
	tests := []struct {
		name           string
		files          map[string]string
		input          string
		wantSubstrings []string
	}{
		{
			name:  "missing file",
			input: "value: !include missing.yaml\n",
			wantSubstrings: []string{
				"!include at line 1, column 8",
				`for "missing.yaml"`,
				"read included file",
			},
		},
		{
			name: "invalid included yaml",
			files: map[string]string{
				"broken.yaml": "foo: [\n",
			},
			input: "value: !include broken.yaml\n",
			wantSubstrings: []string{
				"!include at line 1, column 8",
				`for "broken.yaml"`,
				"parse included YAML",
			},
		},
		{
			name: "cycle detection",
			files: map[string]string{
				"a.yaml": "next: !include b.yaml\n",
				"b.yaml": "next: !include a.yaml\n",
			},
			input: "value: !include a.yaml\n",
			wantSubstrings: []string{
				"include cycle detected",
				"include chain: a.yaml -> b.yaml -> a.yaml",
			},
		},
		{
			name: "multi document include",
			files: map[string]string{
				"child.yaml": "---\na: 1\n---\nb: 2\n",
			},
			input: "value: !include child.yaml\n",
			wantSubstrings: []string{
				`for "child.yaml"`,
				"included file must contain exactly one YAML document",
			},
		},
		{
			name:  "non scalar include path",
			input: "value: !include {a: 1}\n",
			wantSubstrings: []string{
				"include path must be a scalar value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)
			chdir(t, dir)

			_, err := InlineYAML([]byte(tt.input))
			if err == nil {
				t.Fatal("InlineYAML returned nil error")
			}

			for _, want := range tt.wantSubstrings {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("expected error to contain %q, got %q", want, err)
				}
			}
		})
	}
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for path, content := range files {
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", fullPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", fullPath, err)
		}
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd to %q: %v", cwd, err)
		}
	})
}

func assertYAMLEqual(t *testing.T, got []byte, want string) {
	t.Helper()

	var gotValue any
	if err := yaml.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got YAML: %v\n%s", err, got)
	}

	var wantValue any
	if err := yaml.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("unmarshal want YAML: %v\n%s", err, want)
	}

	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("YAML mismatch\n got: %s\nwant: %s", got, want)
	}
}
