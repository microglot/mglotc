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
		parser   func(p *parserMicroglotTokens) node
		expected node
	}{
		{
			name:   "valid syntax statement",
			input:  "syntax = \"microglot0\"",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementSyntax() },
			expected: &astStatementSyntax{
				syntax: astTextLit{
					value: idl.Token{
						Value: "microglot0",
					},
				},
			},
		},
		{
			name:     "invalid syntax statement",
			input:    "syntax lemon",
			parser:   func(p *parserMicroglotTokens) node { return p.parseStatementSyntax() },
			expected: (*astStatementSyntax)(nil),
		},
		{
			name:   "simple versioned module statement",
			input:  "module = @123",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementModuleMeta() },
			expected: &astStatementModuleMeta{
				comments: astCommentBlock{
					comments: nil,
				},
				uid: astIntLit{
					token: idl.Token{
						Value: "123",
					},
					value: 123,
				},
			},
		},
		{
			name:   "module with comment block",
			input:  "module = @123\n//comment\n//another\n",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementModuleMeta() },
			expected: &astStatementModuleMeta{
				uid: astIntLit{
					token: idl.Token{
						Value: "123",
					},
					value: 123,
				},
				comments: astCommentBlock{
					comments: []idl.Token{
						idl.Token{
							Value: "comment",
						},
						idl.Token{
							Value: "another",
						},
					},
				},
			},
		},
		{
			name:     "invalid module statement",
			input:    "module lemon",
			parser:   func(p *parserMicroglotTokens) node { return p.parseStatementModuleMeta() },
			expected: (*astStatementModuleMeta)(nil),
		},
		/*
			{
				name:   "non-namespaced annotation instance",
				input:  "foo(1)",
				parser: func(p *parserMicroglotTokens) node { return p.parseAnnotationInstance() },
				expected: &astAnnotationInstance{
					namespace_identifier: nil,
					identifier: idl.Token{
						Value: "foo",
					},
					value: astValue{},
				},
			},
			{
				name:   "namespaced annotation instance",
				input:  "foo.bar(1)",
				parser: func(p *parserMicroglotTokens) node { return p.parseAnnotationInstance() },
				expected: &astAnnotationInstance{
					namespace_identifier: &idl.Token{
						Value: "foo",
					},
					identifier: idl.Token{
						Value: "bar",
					},
					value: astValue{},
				},
			},
		*/
	}
	for _, testCase := range testCases {
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
			p, err := parser.PrepareParse(ctx, lexerFile)
			require.Nil(t, err)
			require.Equal(t, testCase.expected, testCase.parser(p))
		})
	}
}
