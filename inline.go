package yamlinline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

const includeTag = "!include"

// InlineYAML expands whole-node !include tags and returns normalized YAML bytes.
func InlineYAML(src []byte) ([]byte, error) {
	file, err := parser.ParseBytes(src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	resolver := includeResolver{cwd: cwd}
	for _, doc := range file.Docs {
		if doc == nil || doc.Body == nil {
			continue
		}

		body, err := resolver.resolveNode(doc.Body)
		if err != nil {
			return nil, err
		}
		doc.Body = body
	}

	return []byte(file.String()), nil
}

type includeResolver struct {
	cwd   string
	stack []string
}

func (r *includeResolver) resolveNode(node ast.Node) (ast.Node, error) {
	if node == nil {
		return nil, nil
	}

	switch n := node.(type) {
	case *ast.DocumentNode:
		if n.Body == nil {
			return n, nil
		}
		body, err := r.resolveNode(n.Body)
		if err != nil {
			return nil, err
		}
		n.Body = body
		return n, nil
	case *ast.MappingNode:
		for _, value := range n.Values {
			if value == nil || value.Value == nil {
				continue
			}

			resolved, err := r.resolveNode(value.Value)
			if err != nil {
				return nil, err
			}
			if resolved == nil {
				resolved, err = newNullNode()
				if err != nil {
					return nil, err
				}
			}
			if err := value.Replace(resolved); err != nil {
				return nil, fmt.Errorf("replace mapping value: %w", err)
			}
		}
		return n, nil
	case *ast.SequenceNode:
		for idx, value := range n.Values {
			resolved, err := r.resolveNode(value)
			if err != nil {
				return nil, err
			}
			if resolved == nil {
				resolved, err = newNullNode()
				if err != nil {
					return nil, err
				}
			}
			if err := n.Replace(idx, resolved); err != nil {
				return nil, fmt.Errorf("replace sequence value: %w", err)
			}
		}
		return n, nil
	case *ast.TagNode:
		if n.Start != nil && n.Start.Value == includeTag {
			return r.resolveInclude(n)
		}
		if n.Value == nil {
			return n, nil
		}
		resolved, err := r.resolveNode(n.Value)
		if err != nil {
			return nil, err
		}
		if resolved == nil {
			resolved, err = newNullNode()
			if err != nil {
				return nil, err
			}
		}
		n.Value = resolved
		return n, nil
	case *ast.AnchorNode:
		if n.Value == nil {
			return n, nil
		}
		resolved, err := r.resolveNode(n.Value)
		if err != nil {
			return nil, err
		}
		if resolved == nil {
			resolved, err = newNullNode()
			if err != nil {
				return nil, err
			}
		}
		n.Value = resolved
		return n, nil
	default:
		return node, nil
	}
}

func (r *includeResolver) resolveInclude(tag *ast.TagNode) (ast.Node, error) {
	path, err := includePath(tag)
	if err != nil {
		return nil, r.includeError(tag, "", nil, err)
	}

	resolvedPath, err := r.resolvePath(path)
	if err != nil {
		return nil, r.includeError(tag, path, nil, fmt.Errorf("resolve include path: %w", err))
	}

	chain := r.chainWith(resolvedPath)
	if r.inStack(resolvedPath) {
		return nil, r.includeError(tag, resolvedPath, chain, fmt.Errorf("include cycle detected"))
	}

	src, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, r.includeError(tag, resolvedPath, chain, fmt.Errorf("read included file: %w", err))
	}

	file, err := parser.ParseBytes(src, 0)
	if err != nil {
		return nil, r.includeError(tag, resolvedPath, chain, fmt.Errorf("parse included YAML: %w", err))
	}
	if len(file.Docs) != 1 {
		return nil, r.includeError(tag, resolvedPath, chain, fmt.Errorf("included file must contain exactly one YAML document"))
	}

	body := file.Docs[0].Body
	if body == nil {
		body, err = newNullNode()
		if err != nil {
			return nil, r.includeError(tag, resolvedPath, chain, err)
		}
	}

	r.stack = append(r.stack, resolvedPath)
	defer func() {
		r.stack = r.stack[:len(r.stack)-1]
	}()

	return r.resolveNode(body)
}

func includePath(tag *ast.TagNode) (string, error) {
	if tag == nil || tag.Value == nil {
		return "", fmt.Errorf("include path must be a scalar value")
	}

	switch value := tag.Value.(type) {
	case *ast.LiteralNode:
		if value.Value == nil {
			return "", fmt.Errorf("include path must be a scalar value")
		}
		path := strings.TrimSpace(value.Value.Value)
		if path == "" {
			return "", fmt.Errorf("include path cannot be empty")
		}
		if strings.ContainsAny(path, "\r\n") {
			return "", fmt.Errorf("include path must be a single line")
		}
		return path, nil
	case ast.ScalarNode:
		path := value.GetToken().Value
		if path == "" {
			return "", fmt.Errorf("include path cannot be empty")
		}
		if strings.ContainsAny(path, "\r\n") {
			return "", fmt.Errorf("include path must be a single line")
		}
		return path, nil
	default:
		return "", fmt.Errorf("include path must be a scalar value, got %s", tag.Value.Type())
	}
}

func (r *includeResolver) resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	absPath, err := filepath.Abs(filepath.Join(r.cwd, path))
	if err != nil {
		return "", err
	}
	return filepath.Clean(absPath), nil
}

func (r *includeResolver) includeError(node ast.Node, path string, chain []string, err error) error {
	message := "!include"
	if position := nodePosition(node); position != "" {
		message += " " + position
	}
	if path != "" {
		message += fmt.Sprintf(" for %q", r.displayPath(path))
	}
	if len(chain) > 0 {
		message += fmt.Sprintf(" (include chain: %s)", strings.Join(r.displayChain(chain), " -> "))
	}
	return fmt.Errorf("%s: %w", message, err)
}

func (r *includeResolver) chainWith(path string) []string {
	chain := make([]string, 0, len(r.stack)+1)
	chain = append(chain, r.stack...)
	chain = append(chain, path)
	return chain
}

func (r *includeResolver) inStack(path string) bool {
	for _, current := range r.stack {
		if current == path {
			return true
		}
	}
	return false
}

func (r *includeResolver) displayChain(chain []string) []string {
	display := make([]string, 0, len(chain))
	for _, path := range chain {
		display = append(display, r.displayPath(path))
	}
	return display
}

func (r *includeResolver) displayPath(path string) string {
	if path == "" {
		return ""
	}

	rel, err := filepath.Rel(r.cwd, path)
	if err != nil {
		return path
	}
	if rel == "." {
		return rel
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

func nodePosition(node ast.Node) string {
	if node == nil || node.GetToken() == nil || node.GetToken().Position == nil {
		return ""
	}

	position := node.GetToken().Position
	return fmt.Sprintf("at line %d, column %d", position.Line, position.Column)
}

func newNullNode() (ast.Node, error) {
	file, err := parser.ParseBytes([]byte("null\n"), 0)
	if err != nil {
		return nil, fmt.Errorf("create null node: %w", err)
	}
	if len(file.Docs) != 1 || file.Docs[0] == nil || file.Docs[0].Body == nil {
		return nil, fmt.Errorf("create null node: unexpected AST shape")
	}
	return file.Docs[0].Body, nil
}
