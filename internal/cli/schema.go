package cli

import (
	"encoding/json"
	"fmt"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/expression"
	"github.com/spf13/cobra"
)

// SchemaOutput represents the combined output structure
type SchemaOutput struct {
	Schema      json.RawMessage                  `json:"schema"`
	Expressions []expression.ExpressionDef       `json:"expressions"`
	Functions   []*expression.FunctionDefinition `json:"functions"`
}

// schemaCmd represents the schema command
var schemaCmd = &cobra.Command{
	Use:    "schema",
	Short:  "Output JSON schema and definitions",
	Long:   `Output JSON schema, expression definitions, and function definitions for the Lacquer DSL.`,
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		schemaBytes, err := ast.NewSchema()
		if err != nil {
			fmt.Printf("Error generating schema: %v\n", err)
			return
		}

		output := SchemaOutput{
			Schema:      json.RawMessage(schemaBytes),
			Expressions: expression.ExpressionDefs,
			Functions:   expression.FunctionDefs,
		}

		// Marshal to JSON
		outputBytes, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling output: %v\n", err)
			return
		}

		fmt.Println(string(outputBytes))
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
