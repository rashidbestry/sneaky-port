// Package quality enforces the user's repository-wide function documentation requirement.
package quality

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryFunctionCommented audits declarations and anonymous functions in owned Go source.
func TestEveryFunctionCommented(t *testing.T) {
	root := filepath.Join("..", "..")
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.HasPrefix(string(module), "module sneaky-port\n") && !strings.HasPrefix(string(module), "module sneaky-port\r\n") {
		t.Fatal("comment audit must run from this repository's internal/quality package directory")
	}
	count := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error { // Inspect owned source only; exclude generated artifacts and restricted application data.
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".cache" || d.Name() == "build" || d.Name() == "test-output" || d.Name() == "client-data" || d.Name() == "server-data" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool { // Require purpose comments without relying on exported-name conventions alone.
			switch fn := n.(type) {
			case *ast.FuncDecl:
				count++
				if fn.Doc == nil || strings.TrimSpace(fn.Doc.Text()) == "" {
					t.Errorf("%s: missing function comment for %s", set.Position(fn.Pos()), fn.Name.Name)
				}
			case *ast.FuncLit:
				count++
				start, end := set.Position(fn.Pos()).Line, set.Position(fn.Body.Lbrace).Line
				documented := false
				for _, group := range file.Comments {
					line := set.Position(group.Pos()).Line
					if line >= start-1 && line <= end+1 && strings.TrimSpace(group.Text()) != "" {
						documented = true
						break
					}
				}
				if !documented {
					t.Errorf("%s: missing anonymous-function purpose comment", set.Position(fn.Pos()))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("checked %d documented functions", count)
}
