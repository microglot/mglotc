package protobuf

import (
	"strings"
	"testing"

	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/stretchr/testify/require"

	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

var zero uint64 = 0

func TestDescriptor(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected *proto.Module
	}{
		{
			name:  "bare minimum",
			input: "syntax = \"proto3\";",
			expected: &proto.Module{
				UID: 1449310910991872227,
			},
		},
		{
			name:  "message -> struct",
			input: "syntax = \"proto3\";\nmessage Foo { string X = 1; }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						Fields: []*proto.Field{
							&proto.Field{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 1,
								},
								Name: "X",
								Type: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Protobuf{
												Protobuf: "Text",
											},
										},
									},
								},

								AnnotationApplications: []*proto.AnnotationApplication{
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["JsonName"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Text{
												Text: &proto.ValueText{
													Value:  "X",
													Source: "",
												},
											},
										},
									},
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["Proto3Optional"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Bool{
												Bool: &proto.ValueBool{
													Value: false,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "nested message -> struct",
			input: "syntax = \"proto3\";\nmessage Foo { message Bar { message Baz { pkg.Barney X = 1; } } }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo_Bar_Baz",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						Fields: []*proto.Field{
							&proto.Field{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 1,
								},
								Name: "X",
								Type: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Protobuf{
												Protobuf: "pkg.Barney",
											},
										},
									},
								},

								AnnotationApplications: []*proto.AnnotationApplication{
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["JsonName"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Text{
												Text: &proto.ValueText{
													Value:  "X",
													Source: "",
												},
											},
										},
									},
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["Proto3Optional"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Bool{
												Bool: &proto.ValueBool{
													Value: false,
												},
											},
										},
									},
								},
							},
						},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo_Bar",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						AnnotationApplications: []*proto.AnnotationApplication{
							&proto.AnnotationApplication{
								Annotation: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Resolved{
										Resolved: &proto.ResolvedReference{
											Reference: &proto.TypeReference{
												ModuleUID: 2,
												TypeUID:   idl.PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
											},
										},
									},
								},
								Value: &proto.Value{
									Kind: &proto.Value_Struct{
										Struct: &proto.ValueStruct{
											Fields: []*proto.ValueStructField{
												&proto.ValueStructField{
													Name: "NestedTypes",
													Value: &proto.Value{
														Kind: &proto.Value_List{
															List: &proto.ValueList{
																Elements: []*proto.Value{
																	&proto.Value{
																		Kind: &proto.Value_Struct{
																			Struct: &proto.ValueStruct{
																				Fields: []*proto.ValueStructField{
																					&proto.ValueStructField{
																						Name: "From",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Baz",
																									Source: "Baz",
																								},
																							},
																						},
																					},
																					&proto.ValueStructField{
																						Name: "To",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Foo_Bar_Baz",
																									Source: "Foo_Bar_Baz",
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						AnnotationApplications: []*proto.AnnotationApplication{
							&proto.AnnotationApplication{
								Annotation: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Resolved{
										Resolved: &proto.ResolvedReference{
											Reference: &proto.TypeReference{
												ModuleUID: 2,
												TypeUID:   idl.PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
											},
										},
									},
								},
								Value: &proto.Value{
									Kind: &proto.Value_Struct{
										Struct: &proto.ValueStruct{
											Fields: []*proto.ValueStructField{
												&proto.ValueStructField{
													Name: "NestedTypes",
													Value: &proto.Value{
														Kind: &proto.Value_List{
															List: &proto.ValueList{
																Elements: []*proto.Value{
																	&proto.Value{
																		Kind: &proto.Value_Struct{
																			Struct: &proto.ValueStruct{
																				Fields: []*proto.ValueStructField{
																					&proto.ValueStructField{
																						Name: "From",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Bar",
																									Source: "Bar",
																								},
																							},
																						},
																					},
																					&proto.ValueStructField{
																						Name: "To",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Foo_Bar",
																									Source: "Foo_Bar",
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "nested message -> uniquely named struct",
			input: "syntax = \"proto3\";\nmessage Foo_Bar { }\nmessage Foo { message Bar { } }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo_Bar",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo_BarX",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						AnnotationApplications: []*proto.AnnotationApplication{
							&proto.AnnotationApplication{
								Annotation: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Resolved{
										Resolved: &proto.ResolvedReference{
											Reference: &proto.TypeReference{
												ModuleUID: 2,
												TypeUID:   idl.PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
											},
										},
									},
								},
								Value: &proto.Value{
									Kind: &proto.Value_Struct{
										Struct: &proto.ValueStruct{
											Fields: []*proto.ValueStructField{
												&proto.ValueStructField{
													Name: "NestedTypes",
													Value: &proto.Value{
														Kind: &proto.Value_List{
															List: &proto.ValueList{
																Elements: []*proto.Value{
																	&proto.Value{
																		Kind: &proto.Value_Struct{
																			Struct: &proto.ValueStruct{
																				Fields: []*proto.ValueStructField{
																					&proto.ValueStructField{
																						Name: "From",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Bar",
																									Source: "Bar",
																								},
																							},
																						},
																					},
																					&proto.ValueStructField{
																						Name: "To",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Foo_BarX",
																									Source: "Foo_BarX",
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "oneof -> union",
			input: "syntax = \"proto3\";\nmessage Foo { oneof Bar { string Baz = 1; string Barney = 2; } }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						Unions: []*proto.Union{
							&proto.Union{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: idl.Incomplete,
								},
								Name: "Bar",
							},
						},
						Fields: []*proto.Field{
							&proto.Field{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 1,
								},
								Name:       "Baz",
								UnionIndex: &zero,
								Type: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Protobuf{
												Protobuf: "Text",
											},
										},
									},
								},
								AnnotationApplications: []*proto.AnnotationApplication{
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["JsonName"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Text{
												Text: &proto.ValueText{
													Value:  "Baz",
													Source: "",
												},
											},
										},
									},
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["Proto3Optional"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Bool{
												Bool: &proto.ValueBool{
													Value: false,
												},
											},
										},
									},
								},
							},
							&proto.Field{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 2,
								},
								Name:       "Barney",
								UnionIndex: &zero,
								Type: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Protobuf{
												Protobuf: "Text",
											},
										},
									},
								},
								AnnotationApplications: []*proto.AnnotationApplication{
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["JsonName"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Text{
												Text: &proto.ValueText{
													Value:  "Barney",
													Source: "",
												},
											},
										},
									},
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["Proto3Optional"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Bool{
												Bool: &proto.ValueBool{
													Value: false,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "enum -> enum",
			input: "syntax = \"proto3\";\nenum Foo { X = 0; }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Enums: []*proto.Enum{
					&proto.Enum{
						Name: "Foo",
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						Enumerants: []*proto.Enumerant{
							&proto.Enumerant{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 0,
								},
								Name: "X",
							},
						},
						AnnotationApplications: []*proto.AnnotationApplication{
							&proto.AnnotationApplication{
								Annotation: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Resolved{
										Resolved: &proto.ResolvedReference{
											Reference: &proto.TypeReference{
												ModuleUID: 2,
												TypeUID:   idl.PROTOBUF_TYPE_UIDS["EnumFromProto"],
											},
										},
									},
								},
								Value: &proto.Value{Kind: &proto.Value_Bool{Bool: &proto.ValueBool{Value: true}}},
							},
						},
					},
				},
			},
		},
		{
			name:  "nested enum -> enum",
			input: "syntax = \"proto3\";\nmessage Foo { message Bar { enum Baz { X = 0; } } }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo_Bar",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						AnnotationApplications: []*proto.AnnotationApplication{
							&proto.AnnotationApplication{
								Annotation: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Resolved{
										Resolved: &proto.ResolvedReference{
											Reference: &proto.TypeReference{
												ModuleUID: 2,
												TypeUID:   idl.PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
											},
										},
									},
								},
								Value: &proto.Value{
									Kind: &proto.Value_Struct{
										Struct: &proto.ValueStruct{
											Fields: []*proto.ValueStructField{
												&proto.ValueStructField{
													Name: "NestedTypes",
													Value: &proto.Value{
														Kind: &proto.Value_List{
															List: &proto.ValueList{
																Elements: []*proto.Value{
																	&proto.Value{
																		Kind: &proto.Value_Struct{
																			Struct: &proto.ValueStruct{
																				Fields: []*proto.ValueStructField{
																					&proto.ValueStructField{
																						Name: "From",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Baz",
																									Source: "Baz",
																								},
																							},
																						},
																					},
																					&proto.ValueStructField{
																						Name: "To",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Foo_Bar_Baz",
																									Source: "Foo_Bar_Baz",
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						AnnotationApplications: []*proto.AnnotationApplication{
							&proto.AnnotationApplication{
								Annotation: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Resolved{
										Resolved: &proto.ResolvedReference{
											Reference: &proto.TypeReference{
												ModuleUID: 2,
												TypeUID:   idl.PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
											},
										},
									},
								},
								Value: &proto.Value{
									Kind: &proto.Value_Struct{
										Struct: &proto.ValueStruct{
											Fields: []*proto.ValueStructField{
												&proto.ValueStructField{
													Name: "NestedTypes",
													Value: &proto.Value{
														Kind: &proto.Value_List{
															List: &proto.ValueList{
																Elements: []*proto.Value{
																	&proto.Value{
																		Kind: &proto.Value_Struct{
																			Struct: &proto.ValueStruct{
																				Fields: []*proto.ValueStructField{
																					&proto.ValueStructField{
																						Name: "From",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Bar",
																									Source: "Bar",
																								},
																							},
																						},
																					},
																					&proto.ValueStructField{
																						Name: "To",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Foo_Bar",
																									Source: "Foo_Bar",
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Enums: []*proto.Enum{
					&proto.Enum{
						Name: "Foo_Bar_Baz",
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						Enumerants: []*proto.Enumerant{
							&proto.Enumerant{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 0,
								},
								Name: "X",
							},
						},
						AnnotationApplications: []*proto.AnnotationApplication{
							&proto.AnnotationApplication{
								Annotation: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Resolved{
										Resolved: &proto.ResolvedReference{
											Reference: &proto.TypeReference{
												ModuleUID: 2,
												TypeUID:   idl.PROTOBUF_TYPE_UIDS["EnumFromProto"],
											},
										},
									},
								},
								Value: &proto.Value{Kind: &proto.Value_Bool{Bool: &proto.ValueBool{Value: true}}},
							},
						},
					},
				},
			},
		},
		{
			name:  "service -> api",
			input: "syntax = \"proto3\";service Foo { rpc Bar(Baz) returns (Barney); }",
			expected: &proto.Module{
				UID: 1449310910991872227,
				APIs: []*proto.API{
					&proto.API{
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Methods: []*proto.APIMethod{
							&proto.APIMethod{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: idl.Incomplete,
								},
								Name: "Bar",
								Input: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Protobuf{
												Protobuf: "Baz",
											},
										},
									},
								},
								Output: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Protobuf{
												Protobuf: "Barney",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "map<string,string> field -> list of synthetic structs",
			input: "syntax = \"proto3\";message Foo { map<string,string> Bar = 1; }",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						IsSynthetic: true,
						Name: &proto.TypeName{
							Name: "Foo_BarEntry",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},
						Fields: []*proto.Field{
							&proto.Field{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 1,
								},
								Name: "key",
								Type: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Protobuf{
												Protobuf: "Text",
											},
										},
									},
								},

								AnnotationApplications: []*proto.AnnotationApplication{
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["JsonName"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Text{
												Text: &proto.ValueText{
													Value:  "key",
													Source: "",
												},
											},
										},
									},
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["Proto3Optional"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Bool{
												Bool: &proto.ValueBool{
													Value: false,
												},
											},
										},
									},
								},
							},
							&proto.Field{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 2,
								},
								Name: "value",
								Type: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Protobuf{
												Protobuf: "Text",
											},
										},
									},
								},

								AnnotationApplications: []*proto.AnnotationApplication{
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["JsonName"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Text{
												Text: &proto.ValueText{
													Value:  "value",
													Source: "",
												},
											},
										},
									},
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["Proto3Optional"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Bool{
												Bool: &proto.ValueBool{
													Value: false,
												},
											},
										},
									},
								},
							},
						},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{
							ModuleUID: idl.Incomplete,
							TypeUID:   idl.Incomplete,
						},

						AnnotationApplications: []*proto.AnnotationApplication{
							&proto.AnnotationApplication{
								Annotation: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Resolved{
										Resolved: &proto.ResolvedReference{
											Reference: &proto.TypeReference{
												ModuleUID: 2,
												TypeUID:   idl.PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
											},
										},
									},
								},
								Value: &proto.Value{
									Kind: &proto.Value_Struct{
										Struct: &proto.ValueStruct{
											Fields: []*proto.ValueStructField{
												&proto.ValueStructField{
													Name: "NestedTypes",
													Value: &proto.Value{
														Kind: &proto.Value_List{
															List: &proto.ValueList{
																Elements: []*proto.Value{
																	&proto.Value{
																		Kind: &proto.Value_Struct{
																			Struct: &proto.ValueStruct{
																				Fields: []*proto.ValueStructField{
																					&proto.ValueStructField{
																						Name: "From",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "BarEntry",
																									Source: "BarEntry",
																								},
																							},
																						},
																					},
																					&proto.ValueStructField{
																						Name: "To",
																						Value: &proto.Value{
																							Kind: &proto.Value_Text{
																								Text: &proto.ValueText{
																									Value:  "Foo_BarEntry",
																									Source: "Foo_BarEntry",
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},

						Fields: []*proto.Field{
							&proto.Field{
								Reference: &proto.AttributeReference{
									ModuleUID:    idl.Incomplete,
									TypeUID:      idl.Incomplete,
									AttributeUID: 1,
								},
								Name: "Bar",
								Type: &proto.TypeSpecifier{
									Reference: &proto.TypeSpecifier_Forward{
										Forward: &proto.ForwardReference{
											Reference: &proto.ForwardReference_Microglot{
												Microglot: &proto.MicroglotForwardReference{
													Name: &proto.TypeName{
														Name: "List",
														Parameters: []*proto.TypeSpecifier{
															&proto.TypeSpecifier{
																Reference: &proto.TypeSpecifier_Forward{
																	Forward: &proto.ForwardReference{
																		Reference: &proto.ForwardReference_Protobuf{
																			Protobuf: "BarEntry",
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},

								AnnotationApplications: []*proto.AnnotationApplication{
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["JsonName"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Text{
												Text: &proto.ValueText{
													Value:  "Bar",
													Source: "",
												},
											},
										},
									},
									&proto.AnnotationApplication{
										Annotation: &proto.TypeSpecifier{
											Reference: &proto.TypeSpecifier_Resolved{
												Resolved: &proto.ResolvedReference{
													Reference: &proto.TypeReference{
														ModuleUID: 2,
														TypeUID:   idl.PROTOBUF_TYPE_UIDS["Proto3Optional"],
													},
												},
											},
										},
										Value: &proto.Value{
											Kind: &proto.Value_Bool{
												Bool: &proto.ValueBool{
													Value: false,
												},
											},
										},
									},
								},
							},
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
			h := reporter.NewHandler(reporter.NewReporter(
				func(err reporter.ErrorWithPos) error { return nil },
				func(err reporter.ErrorWithPos) {},
			))
			ast, err := parser.Parse("", strings.NewReader(testCase.input), h)
			require.Nil(t, err)

			result, err := parser.ResultFromAST(ast, true, h)
			require.Nil(t, err)

			module, err := FromFileDescriptorProto(result.FileDescriptorProto())
			require.Nil(t, err)
			require.Equal(t, testCase.expected, module)
		})
	}
}
