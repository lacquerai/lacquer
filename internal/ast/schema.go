package ast

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/invopop/jsonschema"
	"github.com/stoewer/go-strcase"
)

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

	err := r.AddGoComments("github.com/lacquerai/lacquer", "./internal/ast")
	if err != nil {
		log.Fatalf("Failed to add go comments: %v", err)
	}

	return &CustomReflector{Reflector: r}
}

func NewSchema() ([]byte, error) {
	reflector := NewCustomReflector()

	fullSchema := reflector.Reflect(&Workflow{})
	return json.MarshalIndent(fullSchema, "", "  ")
}
