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
)

func mapFrom[F any, T any](in []*F, f func(*F) (T, error)) ([]T, error) {
	if in != nil {
		out := make([]T, 0, len(in))

		for _, element := range in {
			outElement, err := f(element)
			if err != nil {
				return nil, err
			}
			out = append(out, outElement)
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
						// moduleUID 1 is for Protobuf annotations
						ModuleUID: 1,
						TypeUID:   idl.PROTOBUF_TYPE_UIDS[name],
					},
				},
			},
		},
		Value: value,
	})
}

// $(Protobuf.NestedTypeInfo()) is encoded as a flattened list of key
func computeNestedTypeInfo(promoted map[string]string) *proto.Value {
	elements := make([]*proto.Value, 0)
	for key, value := range promoted {
		elements = append(elements, &proto.Value{
			Kind: &proto.Value_Text{
				Text: &proto.ValueText{
					Value:  key,
					Source: key,
				},
			},
		})
		elements = append(elements, &proto.Value{
			Kind: &proto.Value_Text{
				Text: &proto.ValueText{
					Value:  value,
					Source: value,
				},
			},
		})
	}
	return &proto.Value{
		Kind: &proto.Value_List{
			List: &proto.ValueList{
				Elements: elements,
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

func promoteNested(structs *[]*proto.Struct, enums *[]*proto.Enum, prefix string, descriptor *descriptorpb.DescriptorProto) (map[string]string, error) {
	var promotions map[string]string
	for _, descriptorProto := range descriptor.NestedType {
		// recur
		promoted, err := promoteNested(structs, enums, prefix+*descriptorProto.Name+"_", descriptorProto)
		if err != nil {
			return nil, err
		}

		struct_, err := fromDescriptorProto(descriptorProto)
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
		enum, err := fromEnumDescriptorProto(enumDescriptorProto)
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

func FromFileDescriptorProto(fileDescriptor *descriptorpb.FileDescriptorProto) (*proto.Module, error) {
	var imports []*proto.Import
	for _, import_ := range fileDescriptor.Dependency {
		imports = append(imports, &proto.Import{
			// ModuleUID:
			// ImportedUID:
			// IsDot:

			ImportedURI: import_,
			// Alias:

			// CommentBlock:
		})
	}

	var structs []*proto.Struct
	enums, err := mapFrom(fileDescriptor.EnumType, fromEnumDescriptorProto)
	if err != nil {
		return nil, err
	}
	for _, descriptorProto := range fileDescriptor.MessageType {
		promoted, err := promoteNested(&structs, &enums, *descriptorProto.Name+"_", descriptorProto)
		if err != nil {
			return nil, err
		}
		struct_, err := fromDescriptorProto(descriptorProto)
		if err != nil {
			return nil, err
		}

		if promoted != nil {
			struct_.AnnotationApplications = appendProtobufAnnotation(struct_.AnnotationApplications, "NestedTypeInfo", computeNestedTypeInfo(promoted))
		}
		structs = append(structs, struct_)
	}

	// compute moduleUID
	var moduleUID uint64
	hasher := sha256.New()
	if fileDescriptor.Package != nil {
		hasher.Write([]byte(*fileDescriptor.Package))
	}
	hasher.Write([]byte(*fileDescriptor.Name))
	err = binary.Read(bytes.NewReader(hasher.Sum(nil)), binary.LittleEndian, &moduleUID)
	if err != nil {
		return nil, err
	}

	// compute protobufPackage
	var protobufPackage string
	if fileDescriptor.Package != nil {
		protobufPackage = *fileDescriptor.Package
	}

	apis, err := mapFrom(fileDescriptor.Service, fromServiceDescriptorProto)
	if err != nil {
		return nil, err
	}

	// TODO 2023.10.06: annotate with $(Protobuf.Package("")) if there's a package name

	// TODO 2023.10.10: convert Options

	return &proto.Module{
		URI:             *fileDescriptor.Name,
		UID:             moduleUID,
		ProtobufPackage: protobufPackage,
		// AnnotationApplications:
		Imports: imports,
		Structs: structs,
		Enums:   enums,
		APIs:    apis,
		// SDKs
		// Constants
		// Annotations
		// DotImports
	}, nil
}

func fromDescriptorProto(descriptor *descriptorpb.DescriptorProto) (*proto.Struct, error) {
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

	fields, err := mapFrom(descriptor.Field, fromFieldDescriptorProto)
	if err != nil {
		return nil, err
	}

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
		// CommentBlock:
		// AnnotationsApplications:
	}, nil
}

func fromFieldDescriptorProto(fieldDescriptor *descriptorpb.FieldDescriptorProto) (*proto.Field, error) {
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

		// CommentBlock:
	}, nil
}

func fromEnumDescriptorProto(enumDescriptor *descriptorpb.EnumDescriptorProto) (*proto.Enum, error) {
	enumerants, err := mapFrom(enumDescriptor.Value, fromEnumValueDescriptorProto)
	if err != nil {
		return nil, err
	}

	// TODO 2023.10.10: convert Options

	return &proto.Enum{
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
	}, nil
}

func fromEnumValueDescriptorProto(enumValueDescriptor *descriptorpb.EnumValueDescriptorProto) (*proto.Enumerant, error) {
	// TODO 2023.10.10: convert Options

	return &proto.Enumerant{
		Reference: &proto.AttributeReference{
			ModuleUID:    idl.Incomplete,
			TypeUID:      idl.Incomplete,
			AttributeUID: uint64(*enumValueDescriptor.Number),
		},
		Name: *enumValueDescriptor.Name,
		// CommentBlock:
		// AnnotationApplications:
	}, nil
}

func fromServiceDescriptorProto(serviceDescriptor *descriptorpb.ServiceDescriptorProto) (*proto.API, error) {
	methods, err := mapFrom(serviceDescriptor.Method, fromMethodDescriptorProto)
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

func fromMethodDescriptorProto(methodDescriptor *descriptorpb.MethodDescriptorProto) (*proto.APIMethod, error) {
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
