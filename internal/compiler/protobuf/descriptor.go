package protobuf

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	"google.golang.org/protobuf/types/descriptorpb"

	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
	"gopkg.microglot.org/compiler.go/internal/target"
)

func mapFrom[F any, T any](p *idl.PathState, in []*F, f func(*F) (T, error)) ([]T, error) {
	if in != nil {
		out := make([]T, 0, len(in))

		for _, element := range in {
			outElement, err := f(element)
			if err != nil {
				return nil, err
			}
			out = append(out, outElement)
			p.IncrementIndex()
		}
		return out, nil
	}
	return nil, nil
}

func appendProtobufAnnotation(as []*proto.AnnotationApplication, name string, value *proto.Value) []*proto.AnnotationApplication {
	return append(as, &proto.AnnotationApplication{
		Annotation: &proto.TypeSpecifier{
			Reference: &proto.TypeSpecifier_Resolved{
				Resolved: &proto.ResolvedReference{
					Reference: &proto.TypeReference{
						// moduleUID 2 is for Protobuf annotations
						ModuleUID: 2,
						TypeUID:   idl.PROTOBUF_TYPE_UIDS[name],
					},
				},
			},
		},
		Value: value,
	})
}

func appendProtobufAnnotationString(as []*proto.AnnotationApplication, name string, value string) []*proto.AnnotationApplication {
	return appendProtobufAnnotation(as, name, &proto.Value{
		Kind: &proto.Value_Text{
			Text: &proto.ValueText{
				Value: value,
			},
		},
	})
}

func appendProtobufAnnotationBoolean(as []*proto.AnnotationApplication, name string, value bool) []*proto.AnnotationApplication {
	return appendProtobufAnnotation(as, name, &proto.Value{
		Kind: &proto.Value_Bool{
			Bool: &proto.ValueBool{
				Value: value,
			},
		},
	})
}

// $(Protobuf.NestedTypeInfo()) is encoded as a Protobuf.NestedTypes struct
func computeNestedTypeInfo(promoted map[string]string) *proto.Value {
	elements := make([]*proto.Value, 0)
	for key, value := range promoted {
		elements = append(elements, &proto.Value{
			Kind: &proto.Value_Struct{
				Struct: &proto.ValueStruct{
					Fields: []*proto.ValueStructField{
						&proto.ValueStructField{
							Name: "From",
							Value: &proto.Value{
								Kind: &proto.Value_Text{
									Text: &proto.ValueText{
										Value:  key,
										Source: key,
									},
								},
							},
						},
						&proto.ValueStructField{
							Name: "To",
							Value: &proto.Value{
								Kind: &proto.Value_Text{
									Text: &proto.ValueText{
										Value:  value,
										Source: value,
									},
								},
							},
						},
					},
				},
			},
		})
	}
	return &proto.Value{
		Kind: &proto.Value_Struct{
			Struct: &proto.ValueStruct{
				Fields: []*proto.ValueStructField{
					&proto.ValueStructField{
						Name: "NestedTypes",
						Value: &proto.Value{
							Kind: &proto.Value_List{
								List: &proto.ValueList{
									Elements: elements,
								},
							},
						},
					},
				},
			},
		},
	}
}

func nameCollides(name string, structs *[]*proto.Struct, enums *[]*proto.Enum) bool {
	if structs != nil {
		for _, struct_ := range *structs {
			if name == struct_.Name.Name {
				return true
			}
		}
	}
	if enums != nil {
		for _, enum := range *enums {
			if name == enum.Name {
				return true
			}
		}
	}
	return false
}

func (c *fileDescriptorConverter) promoteNested(structs *[]*proto.Struct, enums *[]*proto.Enum, prefix string, descriptor *descriptorpb.DescriptorProto) (map[string]string, error) {
	var promotions map[string]string
	for _, descriptorProto := range descriptor.NestedType {
		// recur
		promoted, err := c.promoteNested(structs, enums, prefix+*descriptorProto.Name+"_", descriptorProto)
		if err != nil {
			return nil, err
		}

		struct_, err := c.fromDescriptorProto(descriptorProto)
		if err != nil {
			return nil, err
		}
		suffix := ""
		for nameCollides(prefix+struct_.Name.Name+suffix, structs, enums) {
			suffix = suffix + "X"
		}
		struct_.Name.Name = prefix + struct_.Name.Name + suffix
		if suffix != "" {
			// TODO 2023.10.31: emit a warning?
		}

		if promoted != nil {
			struct_.AnnotationApplications = appendProtobufAnnotation(struct_.AnnotationApplications, "NestedTypeInfo", computeNestedTypeInfo(promoted))
		}
		*structs = append(*structs, struct_)

		if promotions == nil {
			promotions = make(map[string]string)
		}
		promotions[*descriptorProto.Name] = struct_.Name.Name
	}
	for _, enumDescriptorProto := range descriptor.EnumType {
		enum, err := c.fromEnumDescriptorProto(enumDescriptorProto)
		if err != nil {
			return nil, err
		}
		suffix := ""
		for nameCollides(prefix+enum.Name+suffix, structs, enums) {
			suffix = suffix + "X"
		}
		enum.Name = prefix + enum.Name + suffix
		if suffix != "" {
			// TODO 2023.10.31: emit a warning?
		}
		*enums = append(*enums, enum)

		if promotions == nil {
			promotions = make(map[string]string)
		}
		promotions[*enumDescriptorProto.Name] = enum.Name
	}
	return promotions, nil
}

type fileDescriptorConverter struct {
	fileDescriptor *descriptorpb.FileDescriptorProto

	p *idl.PathState
}

func FromFileDescriptorProto(fileDescriptor *descriptorpb.FileDescriptorProto) (*proto.Module, error) {
	converter := fileDescriptorConverter{
		fileDescriptor: fileDescriptor,
	}
	return converter.convert()
}

func (c *fileDescriptorConverter) convert() (*proto.Module, error) {
	var imports []*proto.Import
	for _, import_ := range c.fileDescriptor.Dependency {
		imports = append(imports, &proto.Import{
			// ModuleUID:
			// ImportedUID:
			// IsDot:

			ImportedURI: target.Normalize(import_),
			// Alias:

			// CommentBlock:
		})
	}

	c.p = &idl.PathState{}

	var structs []*proto.Struct
	enums, err := mapFrom(c.p, c.fileDescriptor.EnumType, c.fromEnumDescriptorProto)
	if err != nil {
		return nil, err
	}
	c.p.PushFieldNumber( /* MessageType */ 4)
	c.p.PushIndex()
	for _, descriptorProto := range c.fileDescriptor.MessageType {
		promoted, err := c.promoteNested(&structs, &enums, *descriptorProto.Name+"_", descriptorProto)
		if err != nil {
			return nil, err
		}
		struct_, err := c.fromDescriptorProto(descriptorProto)
		if err != nil {
			return nil, err
		}

		if promoted != nil {
			struct_.AnnotationApplications = appendProtobufAnnotation(struct_.AnnotationApplications, "NestedTypeInfo", computeNestedTypeInfo(promoted))
		}
		structs = append(structs, struct_)
		c.p.IncrementIndex()
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	// compute moduleUID
	var moduleUID uint64
	hasher := sha256.New()
	if c.fileDescriptor.Package != nil {
		hasher.Write([]byte(*c.fileDescriptor.Package))
	}
	hasher.Write([]byte(*c.fileDescriptor.Name))
	err = binary.Read(bytes.NewReader(hasher.Sum(nil)), binary.LittleEndian, &moduleUID)
	if err != nil {
		return nil, err
	}

	var annotationApplications []*proto.AnnotationApplication

	// compute protobufPackage
	var protobufPackage string
	if c.fileDescriptor.Package != nil {
		protobufPackage = *c.fileDescriptor.Package
		annotationApplications = appendProtobufAnnotationString(annotationApplications, "Package", protobufPackage)
	}

	apis, err := mapFrom(c.p, c.fileDescriptor.Service, c.fromServiceDescriptorProto)
	if err != nil {
		return nil, err
	}

	if c.fileDescriptor.Options != nil {
		if c.fileDescriptor.Options.GoPackage != nil {
			annotationApplications = appendProtobufAnnotationString(annotationApplications, "FileOptionsGoPackage", *c.fileDescriptor.Options.GoPackage)
		}
		// TODO 2023.12.30: ... convert remaining Options
	}

	return &proto.Module{
		URI:                    *c.fileDescriptor.Name,
		UID:                    moduleUID,
		ProtobufPackage:        protobufPackage,
		AnnotationApplications: annotationApplications,
		Imports:                imports,
		Structs:                structs,
		Enums:                  enums,
		APIs:                   apis,
		// SDKs
		// Constants
		// Annotations
		// DotImports
	}, nil
}

func (c *fileDescriptorConverter) fromDescriptorProto(descriptor *descriptorpb.DescriptorProto) (*proto.Struct, error) {
	var unions []*proto.Union
	for index, oneofDescriptor := range descriptor.OneofDecl {
		isSynthetic := false
		for _, fieldDescriptor := range descriptor.Field {
			if fieldDescriptor.Proto3Optional != nil && *fieldDescriptor.Proto3Optional {
				if *fieldDescriptor.OneofIndex == (int32)(index) {
					isSynthetic = true
				}
			}
		}
		if !isSynthetic {
			unions = append(unions, &proto.Union{
				Reference: &proto.AttributeReference{
					ModuleUID:    idl.Incomplete,
					TypeUID:      idl.Incomplete,
					AttributeUID: idl.Incomplete,
				},
				Name: *oneofDescriptor.Name,
				// CommentBlock:
				// AnnotationApplications:
			})
		}
	}

	c.p.PushFieldNumber( /* Field */ 2)
	c.p.PushIndex()
	fields, err := mapFrom(c.p, descriptor.Field, c.fromFieldDescriptorProto)
	if err != nil {
		return nil, err
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	isSynthetic := false
	if descriptor.Options != nil && descriptor.Options.MapEntry != nil && *descriptor.Options.MapEntry {
		isSynthetic = true
	}

	// TODO 2023.10.10: convert other Options

	return &proto.Struct{
		Reference: &proto.TypeReference{
			ModuleUID: idl.Incomplete,
			TypeUID:   idl.Incomplete,
		},
		Name: &proto.TypeName{
			Name:       *descriptor.Name,
			Parameters: nil,
		},
		Fields:      fields,
		Unions:      unions,
		IsSynthetic: isSynthetic,
		// Reserved:
		CommentBlock: c.fromSourceCodeInfo(),
		// AnnotationsApplications:
	}, nil
}

func (c *fileDescriptorConverter) fromFieldDescriptorProto(fieldDescriptor *descriptorpb.FieldDescriptorProto) (*proto.Field, error) {
	typeName := ""
	if fieldDescriptor.Type == nil || *fieldDescriptor.Type == descriptorpb.FieldDescriptorProto_TYPE_GROUP || *fieldDescriptor.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE || *fieldDescriptor.Type == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		typeName = *fieldDescriptor.TypeName
	} else {
		switch *fieldDescriptor.Type {
		case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
			typeName = "Float64"
		case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
			typeName = "Float32"
		case descriptorpb.FieldDescriptorProto_TYPE_INT64:
			typeName = "Int64"
		case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
			typeName = "UInt64"
		case descriptorpb.FieldDescriptorProto_TYPE_INT32:
			typeName = "Int32"
		case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
			// TODO 2023.10.10: annotate with $(Protobuf.FieldType())
			typeName = "UInt64"
		case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
			// TODO 2023.10.10: annotate with $(Protobuf.FieldType())
			typeName = "UInt32"
		case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
			typeName = "Bool"
		case descriptorpb.FieldDescriptorProto_TYPE_STRING:
			typeName = "Text"
		case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
			typeName = "Data"
		case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
			typeName = "UInt32"
		case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
			// TODO 2023.10.10: annotate with $(Protobuf.FieldType())
			typeName = "Int32"
		case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
			// TODO 2023.10.10: annotate with $(Protobuf.FieldType())
			typeName = "Int64"
		case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
			// TODO 2023.10.10: annotate with $(Protobuf.FieldType())
			typeName = "Int32"
		case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
			// TODO 2023.10.10: annotate with $(Protobuf.FieldType())
			typeName = "Int64"
		}
	}

	// Default values are a proto2 feature.
	// In the fieldDescriptor, they are *string. It's not really clear if/where/when
	// default values are supposed to be type-checked.
	// For microglot's descriptor, we need a typed proto.Value.
	// We can't "just" use microglot's parser, because apparently we're supposed to
	// coerce these into the fieldDescriptor.Type, i.e. a default of "10" can be
	// *either* a string or a number, depending on the field type!
	// As a result, we're doing something vaguely type-checker-like here, even
	// though in theory we're not yet at the point of type-checking.
	// In particular, this currently *fails* if the value can't be parsed as the
	// expected type (we could punt these cases down to the type-checker by
	// emitting a ValueText instead, on failure; maybe that'd be better?)
	var defaultValue *proto.Value = nil
	if fieldDescriptor.DefaultValue != nil {
		switch typeName {
		case "Bool":
			v, err := strconv.ParseBool(*fieldDescriptor.DefaultValue)
			if err != nil {
				return nil, err
			}
			defaultValue = &proto.Value{
				Kind: &proto.Value_Bool{
					Bool: &proto.ValueBool{
						Value:  v,
						Source: *fieldDescriptor.DefaultValue,
					},
				},
			}
		case "Float64":
			v, err := strconv.ParseFloat(*fieldDescriptor.DefaultValue, 64)
			if err != nil {
				return nil, err
			}
			defaultValue = &proto.Value{
				Kind: &proto.Value_Float64{
					Float64: &proto.ValueFloat64{
						Value:  v,
						Source: *fieldDescriptor.DefaultValue,
					},
				},
			}
		case "Float32":
			v, err := strconv.ParseFloat(*fieldDescriptor.DefaultValue, 32)
			if err != nil {
				return nil, err
			}
			defaultValue = &proto.Value{
				Kind: &proto.Value_Float32{
					Float32: &proto.ValueFloat32{
						Value:  float32(v),
						Source: *fieldDescriptor.DefaultValue,
					},
				},
			}
		case "Int64":
			v, err := strconv.ParseInt(*fieldDescriptor.DefaultValue, 10, 64)
			if err != nil {
				return nil, err
			}
			defaultValue = &proto.Value{
				Kind: &proto.Value_Int64{
					Int64: &proto.ValueInt64{
						Value:  v,
						Source: *fieldDescriptor.DefaultValue,
					},
				},
			}
		case "Int32":
			v, err := strconv.ParseInt(*fieldDescriptor.DefaultValue, 10, 32)
			if err != nil {
				return nil, err
			}
			defaultValue = &proto.Value{
				Kind: &proto.Value_Int32{
					Int32: &proto.ValueInt32{
						Value:  int32(v),
						Source: *fieldDescriptor.DefaultValue,
					},
				},
			}
		case "UInt64":
			v, err := strconv.ParseUint(*fieldDescriptor.DefaultValue, 10, 64)
			if err != nil {
				return nil, err
			}
			defaultValue = &proto.Value{
				Kind: &proto.Value_UInt64{
					UInt64: &proto.ValueUInt64{
						Value:  v,
						Source: *fieldDescriptor.DefaultValue,
					},
				},
			}
		case "UInt32":
			v, err := strconv.ParseUint(*fieldDescriptor.DefaultValue, 10, 32)
			if err != nil {
				return nil, err
			}
			defaultValue = &proto.Value{
				Kind: &proto.Value_UInt32{
					UInt32: &proto.ValueUInt32{
						Value:  uint32(v),
						Source: *fieldDescriptor.DefaultValue,
					},
				},
			}
		default:
			defaultValue = &proto.Value{
				Kind: &proto.Value_Text{
					Text: &proto.ValueText{
						Value:  *fieldDescriptor.DefaultValue,
						Source: *fieldDescriptor.DefaultValue,
					},
				},
			}
		}
	}

	// TODO 2023.10.10: convert Options

	forwardTypeSpecifier := proto.TypeSpecifier{
		Reference: &proto.TypeSpecifier_Forward{
			Forward: &proto.ForwardReference{
				Reference: &proto.ForwardReference_Protobuf{
					Protobuf: typeName,
				},
			},
		},
	}

	typeSpecifier := forwardTypeSpecifier
	if fieldDescriptor.Label != nil {
		switch *fieldDescriptor.Label {
		case descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL:
			if fieldDescriptor.Proto3Optional != nil && *fieldDescriptor.Proto3Optional {
				typeSpecifier = proto.TypeSpecifier{
					Reference: &proto.TypeSpecifier_Forward{
						Forward: &proto.ForwardReference{
							Reference: &proto.ForwardReference_Microglot{
								Microglot: &proto.MicroglotForwardReference{
									Qualifier: "",
									Name: &proto.TypeName{
										Name: "Presence",
										Parameters: []*proto.TypeSpecifier{
											&forwardTypeSpecifier,
										},
									},
								},
							},
						},
					},
				}
			}
		case descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
			typeSpecifier = proto.TypeSpecifier{
				Reference: &proto.TypeSpecifier_Forward{
					Forward: &proto.ForwardReference{
						Reference: &proto.ForwardReference_Microglot{
							Microglot: &proto.MicroglotForwardReference{
								Qualifier: "",
								Name: &proto.TypeName{
									Name: "List",
									Parameters: []*proto.TypeSpecifier{
										&forwardTypeSpecifier,
									},
								},
							},
						},
					},
				},
			}
		default:
			return nil, fmt.Errorf("unimplemented protobuf label %s", *fieldDescriptor.Label)
		}
	}

	// TODO 2023.11.09: how are protobuf maps represented in the descriptor?

	var unionIndex *uint64
	if fieldDescriptor.OneofIndex != nil {
		unionIndex = new(uint64)
		*unionIndex = (uint64)(*fieldDescriptor.OneofIndex)
	}

	var annotationApplications []*proto.AnnotationApplication
	if fieldDescriptor.JsonName != nil {
		annotationApplications = appendProtobufAnnotationString(annotationApplications, "JsonName", *fieldDescriptor.JsonName)
	}
	annotationApplications = appendProtobufAnnotationBoolean(annotationApplications, "Proto3Optional", fieldDescriptor.Proto3Optional != nil && *fieldDescriptor.Proto3Optional)

	return &proto.Field{
		Reference: &proto.AttributeReference{
			ModuleUID:    idl.Incomplete,
			TypeUID:      idl.Incomplete,
			AttributeUID: (uint64)(*fieldDescriptor.Number),
		},
		Name:                   *fieldDescriptor.Name,
		Type:                   &typeSpecifier,
		DefaultValue:           defaultValue,
		UnionIndex:             unionIndex,
		AnnotationApplications: annotationApplications,

		CommentBlock: c.fromSourceCodeInfo(),
	}, nil
}

func getUnregisteredOption(name string, options []*descriptorpb.UninterpretedOption) (string, bool) {
	for _, option := range options {
		if option.Name[0].NamePart != nil && *option.Name[0].NamePart == name {
			if option.AggregateValue != nil {
				return *option.AggregateValue, true
			}
			if option.StringValue != nil {
				return string(option.StringValue), true
			}
			if option.DoubleValue != nil {
				return strconv.FormatFloat(*option.DoubleValue, 'f', -1, 64), true
			}
			if option.PositiveIntValue != nil {
				return strconv.FormatUint(*option.PositiveIntValue, 10), true
			}
			if option.NegativeIntValue != nil {
				return strconv.FormatInt(*option.NegativeIntValue, 10), true
			}
			if option.IdentifierValue != nil {
				return *option.IdentifierValue, true
			}
			return "", false
		}
	}
	return "", false
}

func (c *fileDescriptorConverter) fromEnumDescriptorProto(enumDescriptor *descriptorpb.EnumDescriptorProto) (*proto.Enum, error) {
	enumerants, err := mapFrom(c.p, enumDescriptor.Value, c.fromEnumValueDescriptorProto)
	if err != nil {
		return nil, err
	}

	result := &proto.Enum{
		Reference: &proto.TypeReference{
			ModuleUID: idl.Incomplete,
			TypeUID:   idl.Incomplete,
		},
		Name:       *enumDescriptor.Name,
		Enumerants: enumerants,
		// Reserved:
		// ReservedNames:
		// CommentBlock:
		// AnnotationApplications:
	}
	// TODO 2023.10.10: convert official Options
	result.AnnotationApplications = appendProtobufAnnotationBoolean(result.AnnotationApplications, "EnumFromProto", true)
	return result, nil
}

func (c *fileDescriptorConverter) fromEnumValueDescriptorProto(enumValueDescriptor *descriptorpb.EnumValueDescriptorProto) (*proto.Enumerant, error) {
	// TODO 2023.10.10: convert Options
	name := *enumValueDescriptor.Name
	trueName, ok := getUnregisteredOption("MicroglotName", enumValueDescriptor.GetOptions().GetUninterpretedOption())
	if ok {
		name = trueName
	}
	return &proto.Enumerant{
		Reference: &proto.AttributeReference{
			ModuleUID:    idl.Incomplete,
			TypeUID:      idl.Incomplete,
			AttributeUID: uint64(*enumValueDescriptor.Number),
		},
		Name: name,
		// CommentBlock:
		// AnnotationApplications:
	}, nil
}

func (c *fileDescriptorConverter) fromServiceDescriptorProto(serviceDescriptor *descriptorpb.ServiceDescriptorProto) (*proto.API, error) {
	methods, err := mapFrom(c.p, serviceDescriptor.Method, c.fromMethodDescriptorProto)
	if err != nil {
		return nil, err
	}

	// TODO 2023.10.10: convert Options

	return &proto.API{
		Reference: &proto.TypeReference{
			ModuleUID: idl.Incomplete,
			TypeUID:   idl.Incomplete,
		},
		Name: &proto.TypeName{
			Name:       *serviceDescriptor.Name,
			Parameters: nil,
		},
		Methods: methods,
		// Reserved:
		// ReservedNames:
		// CommentBlock:
		// AnnotationApplications:
	}, nil
}

func (c *fileDescriptorConverter) fromMethodDescriptorProto(methodDescriptor *descriptorpb.MethodDescriptorProto) (*proto.APIMethod, error) {
	if methodDescriptor.ClientStreaming != nil && *methodDescriptor.ClientStreaming {
		return nil, errors.New("client streaming protobufs have no microglot equivalent")
	}
	if methodDescriptor.ServerStreaming != nil && *methodDescriptor.ServerStreaming {
		return nil, errors.New("server streaming protobufs have no microglot equivalent")
	}

	// TODO 2023.10.10: convert Options

	return &proto.APIMethod{
		Reference: &proto.AttributeReference{
			ModuleUID:    idl.Incomplete,
			TypeUID:      idl.Incomplete,
			AttributeUID: idl.Incomplete,
		},
		Name: *methodDescriptor.Name,
		Input: &proto.TypeSpecifier{
			Reference: &proto.TypeSpecifier_Forward{
				Forward: &proto.ForwardReference{
					Reference: &proto.ForwardReference_Protobuf{
						Protobuf: *methodDescriptor.InputType,
					},
				},
			},
		},
		Output: &proto.TypeSpecifier{
			Reference: &proto.TypeSpecifier_Forward{
				Forward: &proto.ForwardReference{
					Reference: &proto.ForwardReference_Protobuf{
						Protobuf: *methodDescriptor.OutputType,
					},
				},
			},
		},
		// CommentBlock
		// AnnotationApplication
	}, nil
}

func (c *fileDescriptorConverter) fromSourceCodeInfo() *proto.CommentBlock {
	currentPath := c.p.CopyPath()
	if c.fileDescriptor.SourceCodeInfo != nil {
		for _, location := range c.fileDescriptor.SourceCodeInfo.Location {
			if len(location.Path) == len(currentPath) {
				match := true
				for i := 0; i < len(location.Path); i++ {
					if location.Path[i] != currentPath[i] {
						match = false
						break
					}
				}
				if match {
					commentBlock := proto.CommentBlock{}

					if location.LeadingComments != nil {
						commentBlock.Lines = append(commentBlock.Lines, *location.LeadingComments)
					}
					if location.TrailingComments != nil {
						commentBlock.Lines = append(commentBlock.Lines, *location.TrailingComments)
					}
					for _, detached := range location.LeadingDetachedComments {
						commentBlock.Lines = append(commentBlock.Lines, detached)
					}

					if len(commentBlock.Lines) > 0 {
						return &commentBlock
					}
				}
			}
		}
	}
	return nil
}
