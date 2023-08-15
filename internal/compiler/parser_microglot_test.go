package compiler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/fs"
	"gopkg.microglot.org/compiler.go/internal/idl"
)

func TestParser(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected *astStatementSyntax
	}{
		{
			name:  "valid syntax statement",
			input: "syntax = \"microglot0\"",
			expected: &astStatementSyntax{
				syntax: astTextLit{
					text: "microglot0",
				},
			},
		},
		{
			name:     "invalid syntax statement",
			input:    "syntax lemon",
			expected: nil,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		name := testCase.name
		if name == "" {
			name = testCase.input
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			input := fs.NewFileString("/test", testCase.input, idl.FileKindMicroglot)
			rep := exc.NewReporter(nil)
			lexer := &LexerMicroglot{
				reporter: rep,
			}
			lexerFile, err := lexer.Lex(ctx, input)
			require.Nil(t, err)
			parser := NewParserMicroglot(rep)
			ast, err := parser.Parse(ctx, lexerFile)
			require.Nil(t, err)
			require.Equal(t, testCase.expected, ast)
		})
	}
}
