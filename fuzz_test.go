package yamlinline

import (
	"testing"

	yaml "github.com/goccy/go-yaml"
)

func FuzzInlineYAML(f *testing.F) {
	f.Add("plain: value\n", "hello\n", "7\n")
	f.Add("value: !include child.yaml\n", "nested: !include grandchild.yaml\n", "42\n")

	f.Fuzz(func(t *testing.T, root, child, grandchild string) {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"child.yaml":      child,
			"grandchild.yaml": grandchild,
		})
		chdir(t, dir)

		if got, err := InlineYAML([]byte(root)); err == nil {
			assertValidYAML(t, got)
		}

		if got, err := InlineYAML([]byte("value: !include child.yaml\n")); err == nil {
			assertValidYAML(t, got)
		}
	})
}

func assertValidYAML(t *testing.T, data []byte) {
	t.Helper()

	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		t.Fatalf("InlineYAML returned invalid YAML: %v\n%s", err, data)
	}
}
