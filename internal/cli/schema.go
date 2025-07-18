package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/expression"
	"github.com/lacquerai/lacquer/internal/provider"
	"github.com/lacquerai/lacquer/internal/provider/anthropic"
	"github.com/lacquerai/lacquer/internal/provider/claudecode"
	"github.com/lacquerai/lacquer/internal/provider/openai"
	"github.com/spf13/cobra"
)

// SchemaOutput represents the combined output structure
type SchemaOutput struct {
	Schema         json.RawMessage                  `json:"schema"`
	Expressions    []expression.ExpressionDef       `json:"expressions"`
	Functions      []*expression.FunctionDefinition `json:"functions"`
	ModelProviders []Model                          `json:"model_providers"`
}

type Model struct {
	Provider string   `json:"provider"`
	Models   []string `json:"models"`
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

		openaiProvider, err := openai.NewProvider(nil)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error creating OpenAI provider: %v\n", err)
			os.Exit(1)
			return
		}
		anthropicProvider, err := anthropic.NewProvider(nil)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error creating Anthropic provider: %v\n", err)
			os.Exit(1)
			return
		}
		claudecodeProvider, err := claudecode.NewProvider(nil)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error creating Claude Code provider: %v\n", err)
			os.Exit(1)
			return
		}

		modelProviders := []provider.Provider{
			openaiProvider,
			anthropicProvider,
			claudecodeProvider,
		}

		availableModels := []Model{}
		for _, modelProvider := range modelProviders {
			models, err := modelProvider.ListModels(context.Background())
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error listing models: %v\n", err)
				return
			}

			modelNames := []string{}
			for _, model := range models {
				modelNames = append(modelNames, model.ID)
			}

			availableModels = append(availableModels, Model{
				Provider: modelProvider.GetName(),
				Models:   modelNames,
			})
		}

		output := SchemaOutput{
			Schema:         json.RawMessage(schemaBytes),
			Expressions:    expression.ExpressionDefs,
			Functions:      expression.FunctionDefs,
			ModelProviders: availableModels,
		}

		// Marshal to JSON
		outputBytes, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error marshaling output: %v\n", err)
			os.Exit(1)
			return
		}

		fmt.Fprintln(cmd.OutOrStdout(), string(outputBytes))
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
