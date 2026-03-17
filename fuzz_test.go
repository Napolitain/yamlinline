package yamlinline

import (
	"bytes"
	"strings"
	"testing"

	yaml "github.com/goccy/go-yaml"
)

const fuzzMaxStringLen = 256

func FuzzInlineYAML_NoIncludeSemanticNoOp(f *testing.F) {
	f.Add("hello", "world", uint8(0), false, false)
	f.Add("line1\nline2", "", uint8(1), true, true)
	f.Add("0", "\r", uint8(79), false, false)
	f.Add("0", "\r", uint8(69), true, true)
	f.Add("0\r", "0", uint8(0), false, true)

	f.Fuzz(func(t *testing.T, value, nested string, newlineMode uint8, withBOM, trimFinalNewline bool) {
		if len(value) > fuzzMaxStringLen || len(nested) > fuzzMaxStringLen {
			t.Skip()
		}

		src, err := yaml.Marshal(map[string]any{
			"value": value,
			"items": []any{
				nested,
				map[string]any{"nested": value},
			},
		})
		if err != nil {
			t.Fatalf("yaml.Marshal: %v", err)
		}

		src = fuzzFileBytes(src, newlineMode, withBOM, trimFinalNewline)
		if _, err := parseYAML(src); err != nil {
			t.Skip()
		}

		got, err := InlineYAML(src)
		if err != nil {
			t.Fatalf("InlineYAML returned error for no-include document: %v\n%s", err, src)
		}

		assertValidYAML(t, got)
		assertYAMLEqual(t, got, string(stripUTF8BOM(src)))
	})
}

func FuzzInlineYAML_PathForms(f *testing.F) {
	f.Add(uint8(0), uint8(0), uint8(0), false, false, false, false, false, false)
	f.Add(uint8(1), uint8(1), uint8(2), true, true, true, true, true, true)
	f.Add(uint8(2), uint8(2), uint8(1), false, true, false, true, false, true)
	f.Add(uint8(3), uint8(0), uint8(0), true, false, true, false, false, false)

	f.Fuzz(func(t *testing.T, style, rootNewlineMode, childNewlineMode uint8, rootBOM, childBOM, useDocMarkers, withComment, trimRootFinalNewline, trimChildFinalNewline bool) {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"child.yaml": string(fuzzFileBytes([]byte("name: app\ncount: 2\n"), childNewlineMode, childBOM, trimChildFinalNewline)),
		})
		chdir(t, dir)

		root := rootIncludeBytes(style, rootNewlineMode, rootBOM, useDocMarkers, withComment, trimRootFinalNewline)

		got, err := InlineYAML(root)
		if err != nil {
			t.Fatalf("InlineYAML returned error for path-form input: %v\n%s", err, root)
		}
		if strings.Contains(string(got), "!include") {
			t.Fatalf("InlineYAML left include tag in output:\n%s", got)
		}

		assertValidYAML(t, got)
		assertYAMLEqual(t, got, "config:\n  name: app\n  count: 2\n")
	})
}

func FuzzInlineYAML_IncludeGraphs(f *testing.F) {
	f.Add(uint8(0), uint8(0), false, false)
	f.Add(uint8(1), uint8(1), true, false)
	f.Add(uint8(2), uint8(2), false, true)
	f.Add(uint8(3), uint8(0), true, false)
	f.Add(uint8(4), uint8(1), false, true)
	f.Add(uint8(5), uint8(2), true, true)

	f.Fuzz(func(t *testing.T, mode, newlineMode uint8, withBOM, trimFinalNewline bool) {
		dir := t.TempDir()
		files, want, wantErr := buildFuzzGraphCase(mode, newlineMode, withBOM, trimFinalNewline)
		writeFiles(t, dir, files)
		chdir(t, dir)

		root := fuzzFileBytes([]byte("value: !include a.yaml\n"), newlineMode, withBOM, trimFinalNewline)

		got, err := InlineYAML(root)
		if wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("expected error containing %q, got %v", wantErr, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("InlineYAML returned error for include graph: %v\n%s", err, root)
		}
		if strings.Contains(string(got), "!include") {
			t.Fatalf("InlineYAML left include tag in output:\n%s", got)
		}

		assertValidYAML(t, got)
		assertYAMLEqual(t, got, want)
	})
}

func assertValidYAML(t *testing.T, data []byte) {
	t.Helper()

	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		t.Fatalf("InlineYAML returned invalid YAML: %v\n%s", err, data)
	}
}

func fuzzFileBytes(src []byte, newlineMode uint8, withBOM, trimFinalNewline bool) []byte {
	data := applyLineEndingMode(src, newlineMode)
	if trimFinalNewline {
		data = trimOneTrailingLineBreak(data)
	}
	if withBOM {
		data = withUTF8BOM(data)
	}
	return data
}

func applyLineEndingMode(src []byte, newlineMode uint8) []byte {
	src = append([]byte(nil), src...)

	switch newlineMode % 3 {
	case 0:
		return src
	case 1:
		return bytes.ReplaceAll(src, []byte("\n"), []byte("\r\n"))
	default:
		return bytes.ReplaceAll(src, []byte("\n"), []byte("\r"))
	}
}

func trimOneTrailingLineBreak(src []byte) []byte {
	switch {
	case bytes.HasSuffix(src, []byte("\r\n")):
		return src[:len(src)-2]
	case bytes.HasSuffix(src, []byte("\n")), bytes.HasSuffix(src, []byte("\r")):
		return src[:len(src)-1]
	default:
		return src
	}
}

func withUTF8BOM(src []byte) []byte {
	data := make([]byte, 0, len(utf8BOM)+len(src))
	data = append(data, utf8BOM...)
	data = append(data, src...)
	return data
}

func rootIncludeBytes(style, newlineMode uint8, withBOM, useDocMarkers, withComment, trimFinalNewline bool) []byte {
	var body string

	switch style % 4 {
	case 0:
		body = "config: !include child.yaml"
		if withComment {
			body += " # trailing comment"
		}
		body += "\n"
	case 1:
		body = "config: !include \" \tchild.yaml\t \""
		if withComment {
			body += " # trailing comment"
		}
		body += "\n"
	case 2:
		body = "config: !include ' \tchild.yaml\t '"
		if withComment {
			body += " # trailing comment"
		}
		body += "\n"
	default:
		body = "config: !include |\n  child.yaml\n"
	}

	if useDocMarkers {
		body = "---\n" + body + "...\n"
	}

	return fuzzFileBytes([]byte(body), newlineMode, withBOM, trimFinalNewline)
}

func buildFuzzGraphCase(mode, newlineMode uint8, withBOM, trimFinalNewline bool) (map[string]string, string, string) {
	file := func(content string) string {
		return string(fuzzFileBytes([]byte(content), newlineMode, withBOM, trimFinalNewline))
	}

	switch mode % 6 {
	case 0:
		return map[string]string{
			"a.yaml": file("1\n"),
		}, "value: 1\n", ""
	case 1:
		return map[string]string{
			"a.yaml": file("next: !include b.yaml\n"),
			"b.yaml": file("2\n"),
		}, "value:\n  next: 2\n", ""
	case 2:
		return map[string]string{
			"a.yaml": file("next: !include b.yaml\n"),
			"b.yaml": file("child: !include c.yaml\n"),
			"c.yaml": file("3\n"),
		}, "value:\n  next:\n    child: 3\n", ""
	case 3:
		return map[string]string{
			"a.yaml": file("next: !include b.yaml\n"),
			"b.yaml": file("next: !include a.yaml\n"),
		}, "", "include cycle detected"
	case 4:
		return map[string]string{
			"a.yaml": file("next: !include b.yaml\n"),
			"b.yaml": file("child: !include c.yaml\n"),
			"c.yaml": file("loop: !include a.yaml\n"),
		}, "", "include cycle detected"
	default:
		return map[string]string{
			"a.yaml": file(""),
		}, "value: null\n", ""
	}
}
