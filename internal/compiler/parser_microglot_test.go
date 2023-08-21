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
				syntax: astValueLiteralText{
					value: *newTokenLineSpan(1, 21, 20, 10, idl.TokenTypeText, "microglot0"),
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
				uid: astValueLiteralInt{
					token: *newTokenLineSpan(1, 13, 12, 3, idl.TokenTypeIntegerDecimal, "123"),
					value: 123,
				},
			},
		},
		{
			name:   "module with comment block",
			input:  "module = @123\n//comment\n//another\n",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementModuleMeta() },
			expected: &astStatementModuleMeta{
				uid: astValueLiteralInt{
					token: *newTokenLineSpan(1, 13, 12, 3, idl.TokenTypeIntegerDecimal, "123"),
					value: 123,
				},
				comments: astCommentBlock{
					comments: []idl.Token{
						*newTokenLineSpan(2, 9, 23, 7, idl.TokenTypeComment, "comment"),
						*newTokenLineSpan(3, 9, 34, 7, idl.TokenTypeComment, "another"),
					},
				},
			},
		},
		{
			name:   "import",
			input:  "import \"foo\" as .",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementImport() },
			expected: &astStatementImport{
				uri: astValueLiteralText{
					value: *newTokenLineSpan(1, 12, 11, 3, idl.TokenTypeText, "foo"),
				},
				name: *newTokenLineSpan(1, 17, 17, 1, idl.TokenTypeDot, "."),
				comments: astCommentBlock{
					comments: nil,
				},
			},
		},
		{
			name:     "invalid module statement",
			input:    "module lemon",
			parser:   func(p *parserMicroglotTokens) node { return p.parseStatementModuleMeta() },
			expected: (*astStatementModuleMeta)(nil),
		},
		{
			name:   "non-namespaced annotation instance",
			input:  "foo(1)",
			parser: func(p *parserMicroglotTokens) node { return p.parseAnnotationInstance() },
			expected: &astAnnotationInstance{
				namespaceIdentifier: nil,
				identifier:          *newTokenLineSpan(1, 3, 2, 3, idl.TokenTypeIdentifier, "foo"),
				value: (expression)(&astValueLiteralInt{
					token: *newTokenLineSpan(1, 5, 4, 1, idl.TokenTypeIntegerDecimal, "1"),
					value: 1,
				}),
			},
		},
		{
			name:   "namespaced annotation instance",
			input:  "foo.bar(1)",
			parser: func(p *parserMicroglotTokens) node { return p.parseAnnotationInstance() },
			expected: &astAnnotationInstance{
				namespaceIdentifier: newTokenLineSpan(1, 3, 2, 3, idl.TokenTypeIdentifier, "foo"),
				identifier:          *newTokenLineSpan(1, 7, 6, 3, idl.TokenTypeIdentifier, "bar"),
				value: (expression)(&astValueLiteralInt{
					token: *newTokenLineSpan(1, 9, 8, 1, idl.TokenTypeIntegerDecimal, "1"),
					value: 1,
				}),
			},
		},
		{
			name:   "unary operator",
			input:  "-x",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueUnary() },
			expected: &astValueUnary{
				operator: *newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeMinus, "-"),
				operand: &astValueIdentifier{
					qualifiedIdentifier: []idl.Token{
						*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
					},
				},
			},
		},
		{
			name:   "binary operator",
			input:  "(x*x)",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueBinary() },
			expected: &astValueBinary{
				leftOperand: &astValueIdentifier{
					qualifiedIdentifier: []idl.Token{
						*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
					},
				},
				operator: *newTokenLineSpan(1, 3, 3, 1, idl.TokenTypeStar, "*"),
				rightOperand: &astValueIdentifier{
					qualifiedIdentifier: []idl.Token{
						*newTokenLineSpan(1, 4, 3, 1, idl.TokenTypeIdentifier, "x"),
					},
				},
			},
		},
		{
			name:   "literal list (empty)",
			input:  "[]",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueLiteralList() },
			expected: &astValueLiteralList{
				values: []expression{},
			},
		},
		{
			name:   "literal list (non-empty)",
			input:  "[x]",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueLiteralList() },
			expected: &astValueLiteralList{
				values: []expression{
					&astValueIdentifier{
						qualifiedIdentifier: []idl.Token{
							*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
						},
					},
				},
			},
		},
		{
			name:   "literal list (non-empty, with trailing comma)",
			input:  "[x,]",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueLiteralList() },
			expected: &astValueLiteralList{
				values: []expression{
					&astValueIdentifier{
						qualifiedIdentifier: []idl.Token{
							*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
						},
					},
				},
			},
		},
		{
			name:   "qualified identifier",
			input:  "a.b.c",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueIdentifier() },
			expected: &astValueIdentifier{
				qualifiedIdentifier: []idl.Token{
					*newTokenLineSpan(1, 1, 0, 1, idl.TokenTypeIdentifier, "a"),
					*newTokenLineSpan(1, 3, 2, 1, idl.TokenTypeIdentifier, "b"),
					*newTokenLineSpan(1, 5, 4, 1, idl.TokenTypeIdentifier, "c"),
				},
			},
		},
		{
			name:   "literal struct",
			input:  "{ a: 2, }",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueLiteralStruct() },
			expected: &astValueLiteralStruct{
				values: []*astLiteralStructPair{
					&astLiteralStructPair{
						identifier: astValueIdentifier{
							qualifiedIdentifier: []idl.Token{
								*newTokenLineSpan(1, 3, 2, 1, idl.TokenTypeIdentifier, "a"),
							},
						},
						value: &astValueLiteralInt{
							token: *newTokenLineSpan(1, 6, 5, 1, idl.TokenTypeIntegerDecimal, "2"),
							value: 2,
						},
					},
				},
			},
		},
	}
	for _, testCase := range testCases {
		name := testCase.name
		if name == "" {
			name = testCase.input
		}
		t.Run(name, func(t *testing.T) {
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
