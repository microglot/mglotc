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
					val: *newTokenLineSpan(1, 21, 20, 10, idl.TokenTypeText, "microglot0"),
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
				uid: astValueLiteralInt{
					token: *newTokenLineSpan(1, 13, 12, 3, idl.TokenTypeIntegerDecimal, "123"),
					val:   123,
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
					val:   123,
				},
				comments: &astCommentBlock{
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
					val: *newTokenLineSpan(1, 12, 11, 3, idl.TokenTypeText, "foo"),
				},
				name: *newTokenLineSpan(1, 17, 17, 1, idl.TokenTypeDot, "."),
			},
		},
		{
			name:   "annotation",
			input:  "annotation foo (api, sdk,) :bar @1234\n//comment",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementAnnotation() },
			expected: &astStatementAnnotation{
				identifier: *newTokenLineSpan(1, 14, 13, 3, idl.TokenTypeIdentifier, "foo"),
				annotationScopes: []astAnnotationScope{
					astAnnotationScope{
						scope: *newTokenLineSpan(1, 19, 18, 3, idl.TokenTypeKeywordAPI, "api"),
					},
					astAnnotationScope{
						scope: *newTokenLineSpan(1, 24, 23, 3, idl.TokenTypeKeywordSDK, "sdk"),
					},
				},
				typeSpecifier: astTypeSpecifier{
					qualifier: nil,
					typeName: astTypeName{
						identifier: *newTokenLineSpan(1, 31, 30, 3, idl.TokenTypeIdentifier, "bar"),
						parameters: nil,
					},
				},
				uid: &astValueLiteralInt{
					token: *newTokenLineSpan(1, 37, 36, 4, idl.TokenTypeIntegerDecimal, "1234"),
					val:   1234,
				},
				comments: &astCommentBlock{
					comments: []idl.Token{
						*newTokenLineSpan(2, 9, 47, 7, idl.TokenTypeComment, "comment"),
					},
				},
			},
		},
		{
			name:   "const",
			input:  "const foo :bar = 1 @1234 $(baz(2))\n//comment",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementConst() },
			expected: &astStatementConst{
				identifier: *newTokenLineSpan(1, 9, 8, 3, idl.TokenTypeIdentifier, "foo"),
				typeSpecifier: astTypeSpecifier{
					qualifier: nil,
					typeName: astTypeName{
						identifier: *newTokenLineSpan(1, 14, 13, 3, idl.TokenTypeIdentifier, "bar"),
						parameters: nil,
					},
				},
				value: astValue{astValueLiteralInt{
					token: *newTokenLineSpan(1, 18, 17, 1, idl.TokenTypeIntegerDecimal, "1"),
					val:   1,
				}},
				meta: astMetadata{
					uid: &astValueLiteralInt{
						token: *newTokenLineSpan(1, 24, 23, 4, idl.TokenTypeIntegerDecimal, "1234"),
						val:   1234,
					},
					annotationApplication: &astAnnotationApplication{
						annotationInstances: []astAnnotationInstance{
							astAnnotationInstance{
								namespaceIdentifier: nil,
								identifier:          *newTokenLineSpan(1, 30, 29, 3, idl.TokenTypeIdentifier, "baz"),
								value: astValue{astValueLiteralInt{
									token: *newTokenLineSpan(1, 32, 31, 1, idl.TokenTypeIntegerDecimal, "2"),
									val:   2,
								}},
							},
						},
					},
					comments: &astCommentBlock{
						comments: []idl.Token{
							*newTokenLineSpan(2, 9, 44, 7, idl.TokenTypeComment, "comment"),
						},
					},
				},
			},
		},
		{
			name:   "enum",
			input:  "enum foo {\n//comment\nbar baz}",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementEnum() },
			expected: &astStatementEnum{
				identifier: *newTokenLineSpan(1, 8, 7, 3, idl.TokenTypeIdentifier, "foo"),
				innerComments: &astCommentBlock{
					comments: []idl.Token{
						*newTokenLineSpan(2, 9, 20, 7, idl.TokenTypeComment, "comment"),
					},
				},
				enumerants: []astEnumerant{
					astEnumerant{
						identifier: *newTokenLineSpan(3, 3, 25, 3, idl.TokenTypeIdentifier, "bar"),
					},
					astEnumerant{
						identifier: *newTokenLineSpan(3, 7, 29, 3, idl.TokenTypeIdentifier, "baz"),
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
		{
			name:   "struct",
			input:  "struct foo { bar: int }",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementStruct() },
			expected: &astStatementStruct{
				typeName: astTypeName{
					identifier: *newTokenLineSpan(1, 10, 9, 3, idl.TokenTypeIdentifier, "foo"),
				},
				elements: []structelement{
					&astField{
						identifier: *newTokenLineSpan(1, 16, 15, 3, idl.TokenTypeIdentifier, "bar"),
						typeSpecifier: astTypeSpecifier{
							typeName: astTypeName{
								identifier: *newTokenLineSpan(1, 21, 20, 3, idl.TokenTypeIdentifier, "int"),
							},
						},
					},
				},
			},
		},
		{
			name:   "api",
			input:  "api foo extends (:bar,) { baz(:int) returns (:bool) }",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementAPI() },
			expected: &astStatementAPI{
				typeName: astTypeName{
					identifier: *newTokenLineSpan(1, 7, 6, 3, idl.TokenTypeIdentifier, "foo"),
				},
				extends: &astExtension{
					extensions: []astTypeSpecifier{
						astTypeSpecifier{
							typeName: astTypeName{
								identifier: *newTokenLineSpan(1, 21, 20, 3, idl.TokenTypeIdentifier, "bar"),
							},
						},
					},
				},
				methods: []astAPIMethod{
					astAPIMethod{
						identifier: *newTokenLineSpan(1, 29, 28, 3, idl.TokenTypeIdentifier, "baz"),
						methodInput: astAPIMethodInput{
							typeSpecifier: astTypeSpecifier{
								typeName: astTypeName{
									identifier: *newTokenLineSpan(1, 34, 33, 3, idl.TokenTypeIdentifier, "int"),
								},
							},
						},
						methodReturns: astAPIMethodReturns{
							typeSpecifier: astTypeSpecifier{
								typeName: astTypeName{
									identifier: *newTokenLineSpan(1, 50, 49, 4, idl.TokenTypeIdentifier, "bool"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:   "sdk",
			input:  "sdk foo { baz(x :int) returns (:bool) }",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementSDK() },
			expected: &astStatementSDK{
				typeName: astTypeName{
					identifier: *newTokenLineSpan(1, 7, 6, 3, idl.TokenTypeIdentifier, "foo"),
				},
				methods: []astSDKMethod{
					astSDKMethod{
						identifier: *newTokenLineSpan(1, 13, 12, 3, idl.TokenTypeIdentifier, "baz"),
						methodInput: astSDKMethodInput{
							parameters: []astSDKMethodParameter{
								astSDKMethodParameter{
									identifier: *newTokenLineSpan(1, 15, 14, 1, idl.TokenTypeIdentifier, "x"),
									typeSpecifier: astTypeSpecifier{
										typeName: astTypeName{
											identifier: *newTokenLineSpan(1, 20, 19, 3, idl.TokenTypeIdentifier, "int"),
										},
									},
								},
							},
						},
						methodReturns: &astSDKMethodReturns{
							typeSpecifier: astTypeSpecifier{
								typeName: astTypeName{
									identifier: *newTokenLineSpan(1, 36, 35, 4, idl.TokenTypeIdentifier, "bool"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:   "impl",
			input:  "impl foo as(:bar,) { requires { x: y } baz(x :int) {} barney(:int) returns (:int) {}}",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementImpl() },
			expected: &astStatementImpl{
				typeName: astTypeName{
					identifier: *newTokenLineSpan(1, 8, 7, 3, idl.TokenTypeIdentifier, "foo"),
				},
				as: astImplAs{
					types: []astTypeSpecifier{
						astTypeSpecifier{
							typeName: astTypeName{
								identifier: *newTokenLineSpan(1, 16, 15, 3, idl.TokenTypeIdentifier, "bar"),
							},
						},
					},
				},
				requires: &astImplRequires{
					requirements: []astImplRequirement{
						astImplRequirement{
							identifier: *newTokenLineSpan(1, 33, 32, 1, idl.TokenTypeIdentifier, "x"),
							typeSpecifier: astTypeSpecifier{
								typeName: astTypeName{
									identifier: *newTokenLineSpan(1, 36, 35, 1, idl.TokenTypeIdentifier, "y"),
								},
							},
						},
					},
				},
				methods: []implmethod{
					astImplSDKMethod{
						identifier: *newTokenLineSpan(1, 42, 41, 3, idl.TokenTypeIdentifier, "baz"),
						methodInput: astSDKMethodInput{
							parameters: []astSDKMethodParameter{
								astSDKMethodParameter{
									identifier: *newTokenLineSpan(1, 44, 43, 1, idl.TokenTypeIdentifier, "x"),
									typeSpecifier: astTypeSpecifier{
										typeName: astTypeName{
											identifier: *newTokenLineSpan(1, 46, 46, 3, idl.TokenTypeIdentifier, "int"),
										},
									},
								},
							},
						},
						block: astImplBlock{},
					},
					astImplAPIMethod{
						identifier: *newTokenLineSpan(1, 60, 59, 6, idl.TokenTypeIdentifier, "barney"),
						methodInput: astAPIMethodInput{
							typeSpecifier: astTypeSpecifier{
								typeName: astTypeName{
									identifier: *newTokenLineSpan(1, 65, 64, 3, idl.TokenTypeIdentifier, "int"),
								},
							},
						},
						methodReturns: astAPIMethodReturns{
							typeSpecifier: astTypeSpecifier{
								typeName: astTypeName{
									identifier: *newTokenLineSpan(1, 80, 79, 3, idl.TokenTypeIdentifier, "int"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:   "non-namespaced annotation instance",
			input:  "foo(1)",
			parser: func(p *parserMicroglotTokens) node { return p.parseAnnotationInstance() },
			expected: &astAnnotationInstance{
				namespaceIdentifier: nil,
				identifier:          *newTokenLineSpan(1, 3, 2, 3, idl.TokenTypeIdentifier, "foo"),
				value: astValue{astValueLiteralInt{
					token: *newTokenLineSpan(1, 5, 4, 1, idl.TokenTypeIntegerDecimal, "1"),
					val:   1,
				}},
			},
		},
		{
			name:   "namespaced annotation instance",
			input:  "foo.bar(1)",
			parser: func(p *parserMicroglotTokens) node { return p.parseAnnotationInstance() },
			expected: &astAnnotationInstance{
				namespaceIdentifier: newTokenLineSpan(1, 3, 2, 3, idl.TokenTypeIdentifier, "foo"),
				identifier:          *newTokenLineSpan(1, 7, 6, 3, idl.TokenTypeIdentifier, "bar"),
				value: astValue{astValueLiteralInt{
					token: *newTokenLineSpan(1, 9, 8, 1, idl.TokenTypeIntegerDecimal, "1"),
					val:   1,
				}},
			},
		},
		{
			name:   "unary operator",
			input:  "-x",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueUnary() },
			expected: &astValueUnary{
				operator: *newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeMinus, "-"),
				operand: astValue{astValueIdentifier{
					components: []idl.Token{
						*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
					},
				}},
			},
		},
		{
			name:   "binary operator",
			input:  "(x*x)",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueBinary() },
			expected: &astValueBinary{
				leftOperand: astValue{astValueIdentifier{
					components: []idl.Token{
						*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
					},
				}},
				operator: *newTokenLineSpan(1, 3, 3, 1, idl.TokenTypeStar, "*"),
				rightOperand: astValue{astValueIdentifier{
					components: []idl.Token{
						*newTokenLineSpan(1, 4, 3, 1, idl.TokenTypeIdentifier, "x"),
					},
				}},
			},
		},
		{
			name:   "literal list (empty)",
			input:  "[]",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueLiteralList() },
			expected: &astValueLiteralList{
				vals: []astValue{},
			},
		},
		{
			name:   "literal list (non-empty)",
			input:  "[x]",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueLiteralList() },
			expected: &astValueLiteralList{
				vals: []astValue{
					astValue{astValueIdentifier{
						components: []idl.Token{
							*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
						},
					}},
				},
			},
		},
		{
			name:   "literal list (non-empty, with trailing comma)",
			input:  "[x,]",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueLiteralList() },
			expected: &astValueLiteralList{
				vals: []astValue{
					astValue{astValueIdentifier{
						components: []idl.Token{
							*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
						},
					}},
				},
			},
		},
		{
			name:   "qualified identifier",
			input:  "a.b.c",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueIdentifier() },
			expected: &astValueIdentifier{
				components: []idl.Token{
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
				vals: []astLiteralStructPair{
					astLiteralStructPair{
						identifier: *newTokenLineSpan(1, 3, 2, 1, idl.TokenTypeIdentifier, "a"),
						value: astValue{astValueLiteralInt{
							token: *newTokenLineSpan(1, 6, 5, 1, idl.TokenTypeIntegerDecimal, "2"),
							val:   2,
						}},
					},
				},
			},
		},
		{
			name:   "type specifier with qualifier",
			input:  ":foo.bar",
			parser: func(p *parserMicroglotTokens) node { return p.parseTypeSpecifier() },
			expected: &astTypeSpecifier{
				qualifier: newTokenLineSpan(1, 4, 3, 3, idl.TokenTypeIdentifier, "foo"),
				typeName: astTypeName{
					identifier: *newTokenLineSpan(1, 8, 7, 3, idl.TokenTypeIdentifier, "bar"),
					parameters: nil,
				},
			},
		},
		{
			name:   "type specifier without qualifier",
			input:  ":foo",
			parser: func(p *parserMicroglotTokens) node { return p.parseTypeSpecifier() },
			expected: &astTypeSpecifier{
				qualifier: nil,
				typeName: astTypeName{
					identifier: *newTokenLineSpan(1, 4, 3, 3, idl.TokenTypeIdentifier, "foo"),
					parameters: nil,
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
