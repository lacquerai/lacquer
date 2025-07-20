package ast

import (
	"embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/stoewer/go-strcase"
)

//go:embed types.go
var typesGoFile embed.FS

// CustomReflector extends the default reflector to handle custom types
type CustomReflector struct {
	*jsonschema.Reflector
}

// NewCustomReflector creates a new custom reflector with specific handling for lacquer types
func NewCustomReflector() *CustomReflector {
	r := &jsonschema.Reflector{
		KeyNamer: strcase.SnakeCase,
		Namer: func(t reflect.Type) string {
			return strcase.SnakeCase(t.Name())
		},
		ExpandedStruct: true,
	}

	// _, filename, _, ok := runtime.Caller(0)
	// if !ok {
	// 	log.Fatalf("Failed to get caller information")
	// }

	return &CustomReflector{Reflector: r}
}

func NewSchema() ([]byte, error) {
	reflector := NewCustomReflector()
	err := reflector.extractGoComments(reflect.TypeOf(Workflow{}).PkgPath())
	if err != nil {
		return nil, err
	}

	fullSchema := reflector.Reflect(&Workflow{})
	return json.MarshalIndent(fullSchema, "", "  ")
}

func (r *CustomReflector) extractGoComments(pkg string) error {
	commentMap := make(map[string]string)
	fset := token.NewFileSet()
	typesFile, err := typesGoFile.ReadFile("types.go")
	if err != nil {
		return err
	}

	f, err := parser.ParseFile(fset, "types.go", typesFile, parser.ParseComments)
	if err != nil {
		return err
	}

	gtxt := ""
	typ := ""
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			typ = x.Name.String()
			if !ast.IsExported(typ) {
				typ = ""
			} else {
				txt := x.Doc.Text()
				if txt == "" && gtxt != "" {
					txt = gtxt
					gtxt = ""
				}

				commentMap[fmt.Sprintf("%s.%s", pkg, typ)] = strings.TrimSpace(txt)
			}
		case *ast.Field:
			txt := x.Doc.Text()
			if txt == "" {
				txt = x.Comment.Text()
			}
			if typ != "" && txt != "" {
				for _, n := range x.Names {
					if ast.IsExported(n.String()) {
						k := fmt.Sprintf("%s.%s.%s", pkg, typ, n)
						commentMap[k] = strings.TrimSpace(txt)
					}
				}
			}
		case *ast.GenDecl:
			// remember for the next type
			gtxt = x.Doc.Text()
		}
		return true
	})

	r.CommentMap = commentMap

	return nil
}
