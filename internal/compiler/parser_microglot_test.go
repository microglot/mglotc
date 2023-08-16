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
		expected *ast
	}{
		{
			name:  "valid syntax statement",
			input: "syntax = \"microglot0\"",
			expected: &ast{
				statements: []statement{
					&astStatementSyntax{
						syntax: astTextLit{
							value: "microglot0",
						},
					},
				},
			},
		},
		{
			name:     "invalid syntax statement",
			input:    "syntax lemon",
			expected: nil,
		},
		{
			name:  "simple versioned module statement",
			input: "module = @123",
			expected: &ast{
				statements: []statement{
					&astStatementModuleMeta{
						uid: astIntLit{
							value: 123,
						},
					},
				},
			},
		},
		{
			name:  "module with comment block",
			input: "module = @123\n//comment\n//another\n",
			expected: &ast{
				statements: []statement{
					&astStatementModuleMeta{
						uid: astIntLit{
							value: 123,
						},
						comments: []*astComment{
							&astComment{
								value: "comment",
							},
							&astComment{
								value: "another",
							},
						},
					},
				},
			},
		},
		{
			name:     "invalid module statement",
			input:    "module lemon",
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
