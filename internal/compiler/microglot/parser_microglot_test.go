// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package microglot

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
				astNode: astNode{idl.Location{Line: 1, Column: 21, Offset: 20}},
				syntax: astValueLiteralText{
					astNode: astNode{idl.Location{Line: 1, Column: 21, Offset: 20}},
					val:     *newTokenLineSpan(1, 21, 20, 10, idl.TokenTypeText, "microglot0"),
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
				astNode: astNode{idl.Location{Line: 1, Column: 13, Offset: 12}},
				uid: astValueLiteralInt{
					astNode: astNode{idl.Location{Line: 1, Column: 13, Offset: 12}},
					token:   *newTokenLineSpan(1, 13, 12, 3, idl.TokenTypeIntegerDecimal, "123"),
					val:     123,
				},
			},
		},
		{
			name:   "module with comment block",
			input:  "module = @123\n//comment\n//another\n",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementModuleMeta() },
			expected: &astStatementModuleMeta{
				astNode: astNode{idl.Location{Line: 3, Column: 9, Offset: 34}},
				uid: astValueLiteralInt{
					astNode: astNode{idl.Location{Line: 1, Column: 13, Offset: 12}},
					token:   *newTokenLineSpan(1, 13, 12, 3, idl.TokenTypeIntegerDecimal, "123"),
					val:     123,
				},
				comments: &astCommentBlock{
					astNode: astNode{idl.Location{Line: 3, Column: 9, Offset: 34}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 17, Offset: 17}},
				uri: astValueLiteralText{
					astNode: astNode{idl.Location{Line: 1, Column: 12, Offset: 11}},
					val:     *newTokenLineSpan(1, 12, 11, 3, idl.TokenTypeText, "foo"),
				},
				name: *newTokenLineSpan(1, 17, 17, 1, idl.TokenTypeDot, "."),
			},
		},
		{
			name:   "annotation",
			input:  "annotation foo (api, sdk,) :bar @1234\n//comment",
			parser: func(p *parserMicroglotTokens) node { return p.parseStatementAnnotation() },
			expected: &astStatementAnnotation{
				astNode:    astNode{idl.Location{Line: 2, Column: 9, Offset: 47}},
				identifier: *newTokenLineSpan(1, 14, 13, 3, idl.TokenTypeIdentifier, "foo"),
				annotationScopes: []astAnnotationScope{
					astAnnotationScope{
						astNode: astNode{idl.Location{Line: 1, Column: 19, Offset: 18}},
						scope:   *newTokenLineSpan(1, 19, 18, 3, idl.TokenTypeKeywordAPI, "api"),
					},
					astAnnotationScope{
						astNode: astNode{idl.Location{Line: 1, Column: 24, Offset: 23}},
						scope:   *newTokenLineSpan(1, 24, 23, 3, idl.TokenTypeKeywordSDK, "sdk"),
					},
				},
				typeSpecifier: astTypeSpecifier{
					astNode:   astNode{idl.Location{Line: 1, Column: 31, Offset: 30}},
					qualifier: nil,
					typeName: astTypeName{
						astNode:    astNode{idl.Location{Line: 1, Column: 31, Offset: 30}},
						identifier: *newTokenLineSpan(1, 31, 30, 3, idl.TokenTypeIdentifier, "bar"),
						parameters: nil,
					},
				},
				uid: &astValueLiteralInt{
					astNode: astNode{idl.Location{Line: 1, Column: 37, Offset: 36}},
					token:   *newTokenLineSpan(1, 37, 36, 4, idl.TokenTypeIntegerDecimal, "1234"),
					val:     1234,
				},
				comments: &astCommentBlock{
					astNode: astNode{idl.Location{Line: 2, Column: 9, Offset: 47}},
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
				astNode:    astNode{idl.Location{Line: 2, Column: 9, Offset: 44}},
				identifier: *newTokenLineSpan(1, 9, 8, 3, idl.TokenTypeIdentifier, "foo"),
				typeSpecifier: astTypeSpecifier{
					astNode:   astNode{idl.Location{Line: 1, Column: 14, Offset: 13}},
					qualifier: nil,
					typeName: astTypeName{
						astNode:    astNode{idl.Location{Line: 1, Column: 14, Offset: 13}},
						identifier: *newTokenLineSpan(1, 14, 13, 3, idl.TokenTypeIdentifier, "bar"),
						parameters: nil,
					},
				},
				value: astValue{astValueLiteralInt{
					astNode: astNode{idl.Location{Line: 1, Column: 18, Offset: 17}},
					token:   *newTokenLineSpan(1, 18, 17, 1, idl.TokenTypeIntegerDecimal, "1"),
					val:     1,
				}},
				meta: astMetadata{
					uid: &astValueLiteralInt{
						astNode: astNode{idl.Location{Line: 1, Column: 24, Offset: 23}},
						token:   *newTokenLineSpan(1, 24, 23, 4, idl.TokenTypeIntegerDecimal, "1234"),
						val:     1234,
					},
					annotationApplication: &astAnnotationApplication{
						astNode: astNode{idl.Location{Line: 1, Column: 34, Offset: 34}},
						annotationInstances: []astAnnotationInstance{
							astAnnotationInstance{
								astNode:             astNode{idl.Location{Line: 1, Column: 33, Offset: 33}},
								namespaceIdentifier: nil,
								identifier:          *newTokenLineSpan(1, 30, 29, 3, idl.TokenTypeIdentifier, "baz"),
								value: astValue{astValueLiteralInt{
									astNode: astNode{idl.Location{Line: 1, Column: 32, Offset: 31}},
									token:   *newTokenLineSpan(1, 32, 31, 1, idl.TokenTypeIntegerDecimal, "2"),
									val:     2,
								}},
							},
						},
					},
					comments: &astCommentBlock{
						astNode: astNode{idl.Location{Line: 2, Column: 9, Offset: 44}},
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
				astNode:    astNode{idl.Location{Line: 3, Column: 8, Offset: 31}},
				identifier: *newTokenLineSpan(1, 8, 7, 3, idl.TokenTypeIdentifier, "foo"),
				innerComments: &astCommentBlock{
					astNode: astNode{idl.Location{Line: 2, Column: 9, Offset: 20}},
					comments: []idl.Token{
						*newTokenLineSpan(2, 9, 20, 7, idl.TokenTypeComment, "comment"),
					},
				},
				enumerants: []astEnumerant{
					astEnumerant{
						astNode:    astNode{idl.Location{Line: 3, Column: 3, Offset: 25}},
						identifier: *newTokenLineSpan(3, 3, 25, 3, idl.TokenTypeIdentifier, "bar"),
					},
					astEnumerant{
						astNode:    astNode{idl.Location{Line: 3, Column: 7, Offset: 29}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 23, Offset: 23}},
				typeName: astTypeName{
					astNode:    astNode{idl.Location{Line: 1, Column: 10, Offset: 9}},
					identifier: *newTokenLineSpan(1, 10, 9, 3, idl.TokenTypeIdentifier, "foo"),
				},
				elements: []structelement{
					&astField{
						astNode:    astNode{idl.Location{Line: 1, Column: 21, Offset: 20}},
						identifier: *newTokenLineSpan(1, 16, 15, 3, idl.TokenTypeIdentifier, "bar"),
						typeSpecifier: astTypeSpecifier{
							astNode: astNode{idl.Location{Line: 1, Column: 21, Offset: 20}},
							typeName: astTypeName{
								astNode:    astNode{idl.Location{Line: 1, Column: 21, Offset: 20}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 53, Offset: 53}},
				typeName: astTypeName{
					astNode:    astNode{idl.Location{Line: 1, Column: 7, Offset: 6}},
					identifier: *newTokenLineSpan(1, 7, 6, 3, idl.TokenTypeIdentifier, "foo"),
				},
				extends: &astExtension{
					astNode: astNode{idl.Location{Line: 1, Column: 23, Offset: 23}},
					extensions: []astTypeSpecifier{
						astTypeSpecifier{
							astNode: astNode{idl.Location{Line: 1, Column: 21, Offset: 20}},
							typeName: astTypeName{
								astNode:    astNode{idl.Location{Line: 1, Column: 21, Offset: 20}},
								identifier: *newTokenLineSpan(1, 21, 20, 3, idl.TokenTypeIdentifier, "bar"),
							},
						},
					},
				},
				methods: []astAPIMethod{
					astAPIMethod{
						astNode:    astNode{idl.Location{Line: 1, Column: 51, Offset: 51}},
						identifier: *newTokenLineSpan(1, 29, 28, 3, idl.TokenTypeIdentifier, "baz"),
						methodInput: astAPIMethodInput{
							astNode: astNode{idl.Location{Line: 1, Column: 35, Offset: 35}},
							typeSpecifier: astTypeSpecifier{
								astNode: astNode{idl.Location{Line: 1, Column: 34, Offset: 33}},
								typeName: astTypeName{
									astNode:    astNode{idl.Location{Line: 1, Column: 34, Offset: 33}},
									identifier: *newTokenLineSpan(1, 34, 33, 3, idl.TokenTypeIdentifier, "int"),
								},
							},
						},
						methodReturns: astAPIMethodReturns{
							astNode: astNode{idl.Location{Line: 1, Column: 51, Offset: 51}},
							typeSpecifier: astTypeSpecifier{
								astNode: astNode{idl.Location{Line: 1, Column: 50, Offset: 49}},
								typeName: astTypeName{
									astNode:    astNode{idl.Location{Line: 1, Column: 50, Offset: 49}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 39, Offset: 39}},
				typeName: astTypeName{
					astNode:    astNode{idl.Location{Line: 1, Column: 7, Offset: 6}},
					identifier: *newTokenLineSpan(1, 7, 6, 3, idl.TokenTypeIdentifier, "foo"),
				},
				methods: []astSDKMethod{
					astSDKMethod{
						astNode:    astNode{idl.Location{Line: 1, Column: 37, Offset: 37}},
						identifier: *newTokenLineSpan(1, 13, 12, 3, idl.TokenTypeIdentifier, "baz"),
						methodInput: astSDKMethodInput{
							astNode: astNode{idl.Location{Line: 1, Column: 21, Offset: 21}},
							parameters: []astSDKMethodParameter{
								astSDKMethodParameter{
									astNode:    astNode{idl.Location{Line: 1, Column: 20, Offset: 19}},
									identifier: *newTokenLineSpan(1, 15, 14, 1, idl.TokenTypeIdentifier, "x"),
									typeSpecifier: astTypeSpecifier{
										astNode: astNode{idl.Location{Line: 1, Column: 20, Offset: 19}},
										typeName: astTypeName{
											astNode:    astNode{idl.Location{Line: 1, Column: 20, Offset: 19}},
											identifier: *newTokenLineSpan(1, 20, 19, 3, idl.TokenTypeIdentifier, "int"),
										},
									},
								},
							},
						},
						methodReturns: &astSDKMethodReturns{
							astNode: astNode{idl.Location{Line: 1, Column: 37, Offset: 37}},
							typeSpecifier: astTypeSpecifier{
								astNode: astNode{idl.Location{Line: 1, Column: 36, Offset: 35}},
								typeName: astTypeName{
									astNode:    astNode{idl.Location{Line: 1, Column: 36, Offset: 35}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 85, Offset: 85}},
				typeName: astTypeName{
					astNode:    astNode{idl.Location{Line: 1, Column: 8, Offset: 7}},
					identifier: *newTokenLineSpan(1, 8, 7, 3, idl.TokenTypeIdentifier, "foo"),
				},
				as: astImplAs{
					astNode: astNode{idl.Location{Line: 1, Column: 18, Offset: 18}},
					types: []astTypeSpecifier{
						astTypeSpecifier{
							astNode: astNode{idl.Location{Line: 1, Column: 16, Offset: 15}},
							typeName: astTypeName{
								astNode:    astNode{idl.Location{Line: 1, Column: 16, Offset: 15}},
								identifier: *newTokenLineSpan(1, 16, 15, 3, idl.TokenTypeIdentifier, "bar"),
							},
						},
					},
				},
				requires: &astImplRequires{
					astNode: astNode{idl.Location{Line: 1, Column: 38, Offset: 38}},
					requirements: []astImplRequirement{
						astImplRequirement{
							astNode:    astNode{idl.Location{Line: 1, Column: 36, Offset: 35}},
							identifier: *newTokenLineSpan(1, 33, 32, 1, idl.TokenTypeIdentifier, "x"),
							typeSpecifier: astTypeSpecifier{
								astNode: astNode{idl.Location{Line: 1, Column: 36, Offset: 35}},
								typeName: astTypeName{
									astNode:    astNode{idl.Location{Line: 1, Column: 36, Offset: 35}},
									identifier: *newTokenLineSpan(1, 36, 35, 1, idl.TokenTypeIdentifier, "y"),
								},
							},
						},
					},
				},
				methods: []implmethod{
					astImplSDKMethod{
						astNode:    astNode{idl.Location{Line: 1, Column: 53, Offset: 53}},
						identifier: *newTokenLineSpan(1, 42, 41, 3, idl.TokenTypeIdentifier, "baz"),
						methodInput: astSDKMethodInput{
							astNode: astNode{idl.Location{Line: 1, Column: 50, Offset: 50}},
							parameters: []astSDKMethodParameter{
								astSDKMethodParameter{
									astNode:    astNode{idl.Location{Line: 1, Column: 49, Offset: 48}},
									identifier: *newTokenLineSpan(1, 44, 43, 1, idl.TokenTypeIdentifier, "x"),
									typeSpecifier: astTypeSpecifier{
										astNode: astNode{idl.Location{Line: 1, Column: 49, Offset: 48}},
										typeName: astTypeName{
											astNode:    astNode{idl.Location{Line: 1, Column: 49, Offset: 48}},
											identifier: *newTokenLineSpan(1, 49, 48, 3, idl.TokenTypeIdentifier, "int"),
										},
									},
								},
							},
						},
						block: astImplBlock{
							astNode: astNode{idl.Location{Line: 1, Column: 53, Offset: 53}},
						},
					},
					astImplAPIMethod{
						astNode:    astNode{idl.Location{Line: 1, Column: 84, Offset: 84}},
						identifier: *newTokenLineSpan(1, 60, 59, 6, idl.TokenTypeIdentifier, "barney"),
						methodInput: astAPIMethodInput{
							astNode: astNode{idl.Location{Line: 1, Column: 66, Offset: 66}},
							typeSpecifier: astTypeSpecifier{
								astNode: astNode{idl.Location{Line: 1, Column: 65, Offset: 64}},
								typeName: astTypeName{
									astNode:    astNode{idl.Location{Line: 1, Column: 65, Offset: 64}},
									identifier: *newTokenLineSpan(1, 65, 64, 3, idl.TokenTypeIdentifier, "int"),
								},
							},
						},
						methodReturns: astAPIMethodReturns{
							astNode: astNode{idl.Location{Line: 1, Column: 81, Offset: 81}},
							typeSpecifier: astTypeSpecifier{
								astNode: astNode{idl.Location{Line: 1, Column: 80, Offset: 79}},
								typeName: astTypeName{
									astNode:    astNode{idl.Location{Line: 1, Column: 80, Offset: 79}},
									identifier: *newTokenLineSpan(1, 80, 79, 3, idl.TokenTypeIdentifier, "int"),
								},
							},
						},
						block: astImplBlock{
							astNode: astNode{idl.Location{Line: 1, Column: 84, Offset: 84}},
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
				astNode:             astNode{idl.Location{Line: 1, Column: 6, Offset: 6}},
				namespaceIdentifier: nil,
				identifier:          *newTokenLineSpan(1, 3, 2, 3, idl.TokenTypeIdentifier, "foo"),
				value: astValue{astValueLiteralInt{
					astNode: astNode{idl.Location{Line: 1, Column: 5, Offset: 4}},
					token:   *newTokenLineSpan(1, 5, 4, 1, idl.TokenTypeIntegerDecimal, "1"),
					val:     1,
				}},
			},
		},
		{
			name:   "namespaced annotation instance",
			input:  "foo.bar(1)",
			parser: func(p *parserMicroglotTokens) node { return p.parseAnnotationInstance() },
			expected: &astAnnotationInstance{
				astNode:             astNode{idl.Location{Line: 1, Column: 10, Offset: 10}},
				namespaceIdentifier: newTokenLineSpan(1, 3, 2, 3, idl.TokenTypeIdentifier, "foo"),
				identifier:          *newTokenLineSpan(1, 7, 6, 3, idl.TokenTypeIdentifier, "bar"),
				value: astValue{astValueLiteralInt{
					astNode: astNode{idl.Location{Line: 1, Column: 9, Offset: 8}},
					token:   *newTokenLineSpan(1, 9, 8, 1, idl.TokenTypeIntegerDecimal, "1"),
					val:     1,
				}},
			},
		},
		{
			name:   "unary operator",
			input:  "-x",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueUnary() },
			expected: &astValueUnary{
				astNode:  astNode{idl.Location{Line: 1, Column: 2, Offset: 1}},
				operator: *newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeMinus, "-"),
				operand: astValue{astValueIdentifier{
					astNode: astNode{idl.Location{Line: 1, Column: 2, Offset: 1}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 5, Offset: 5}},
				leftOperand: astValue{astValueIdentifier{
					astNode: astNode{idl.Location{Line: 1, Column: 2, Offset: 1}},
					components: []idl.Token{
						*newTokenLineSpan(1, 2, 1, 1, idl.TokenTypeIdentifier, "x"),
					},
				}},
				operator: *newTokenLineSpan(1, 3, 3, 1, idl.TokenTypeStar, "*"),
				rightOperand: astValue{astValueIdentifier{
					astNode: astNode{idl.Location{Line: 1, Column: 4, Offset: 3}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 2, Offset: 2}},
				vals:    []astValue{},
			},
		},
		{
			name:   "literal list (non-empty)",
			input:  "[x]",
			parser: func(p *parserMicroglotTokens) node { return p.parseValueLiteralList() },
			expected: &astValueLiteralList{
				astNode: astNode{idl.Location{Line: 1, Column: 3, Offset: 3}},
				vals: []astValue{
					astValue{astValueIdentifier{
						astNode: astNode{idl.Location{Line: 1, Column: 2, Offset: 1}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 4, Offset: 4}},
				vals: []astValue{
					astValue{astValueIdentifier{
						astNode: astNode{idl.Location{Line: 1, Column: 2, Offset: 1}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 5, Offset: 4}},
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
				astNode: astNode{idl.Location{Line: 1, Column: 9, Offset: 9}},
				vals: []astLiteralStructPair{
					astLiteralStructPair{
						astNode:    astNode{idl.Location{Line: 1, Column: 6, Offset: 5}},
						identifier: *newTokenLineSpan(1, 3, 2, 1, idl.TokenTypeIdentifier, "a"),
						value: astValue{astValueLiteralInt{
							astNode: astNode{idl.Location{Line: 1, Column: 6, Offset: 5}},
							token:   *newTokenLineSpan(1, 6, 5, 1, idl.TokenTypeIntegerDecimal, "2"),
							val:     2,
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
				astNode:   astNode{idl.Location{Line: 1, Column: 8, Offset: 7}},
				qualifier: newTokenLineSpan(1, 4, 3, 3, idl.TokenTypeIdentifier, "foo"),
				typeName: astTypeName{
					astNode:    astNode{idl.Location{Line: 1, Column: 8, Offset: 7}},
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
				astNode:   astNode{idl.Location{Line: 1, Column: 4, Offset: 3}},
				qualifier: nil,
				typeName: astTypeName{
					astNode:    astNode{idl.Location{Line: 1, Column: 4, Offset: 3}},
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
