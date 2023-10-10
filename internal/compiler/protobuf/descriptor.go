package protobuf

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"

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
			return out, nil
		}
	}
	return nil, nil
}

func promoteNested(structs *[]*proto.Struct, enums *[]*proto.Enum, prefix string, descriptor *descriptorpb.DescriptorProto) error {
	for _, descriptorProto := range descriptor.NestedType {
		struct_, err := fromDescriptorProto(descriptorProto)
		if err != nil {
			return err
		}
		struct_.Name.Name = prefix + struct_.Name.Name
		// TODO 2023.10.06: append "X" and warn if promoted name collides
		// TODO 2023.10.06: annotate with $(Protobuf.NestedTypeInfo({}))
		*structs = append(*structs, struct_)
	}
	for _, enumDescriptorProto := range descriptor.EnumType {
		enum, err := fromEnumDescriptorProto(enumDescriptorProto)
		if err != nil {
			return err
		}
		enum.Name = prefix + enum.Name
		// TODO 2023.10.06: append "X" and warn if promoted name collides
		// TODO 2023.10.06: annotate with $(Protobuf.NestedTypeInfo({}))
		*enums = append(*enums, enum)
	}
	// recur
	for _, descriptorProto := range descriptor.NestedType {
		err := promoteNested(structs, enums, prefix+*descriptorProto.Name+"_", descriptorProto)
		if err != nil {
			return err
		}
	}
	return nil
}

func FromFileDescriptorProto(fileDescriptor *descriptorpb.FileDescriptorProto) (*proto.Module, error) {
	var imports []*proto.Import
	for _, import_ := range fileDescriptor.Dependency {
		imports = append(imports, &proto.Import{
			// ModuleUID:
			// ImportedUID:
			// IsDot:

			// TODO 2023.10.10: this works okay as long as none of the imports
			// have "package" statements. If they do, though, they will fail to
			// link (for now, until we work out the desired semantics.)
			ImportedURI: import_,
			// Alias:

			// CommentBlock:
		})
	}

	structs, err := mapFrom(fileDescriptor.MessageType, fromDescriptorProto)
	if err != nil {
		return nil, err
	}
	enums, err := mapFrom(fileDescriptor.EnumType, fromEnumDescriptorProto)
	if err != nil {
		return nil, err
	}

	// promote nested structs and enums
	for _, descriptorProto := range fileDescriptor.MessageType {
		err = promoteNested(&structs, &enums, *descriptorProto.Name+"_", descriptorProto)
		if err != nil {
			return nil, err
		}
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

	apis, err := mapFrom(fileDescriptor.Service, fromServiceDescriptorProto)
	if err != nil {
		return nil, err
	}

	// TODO 2023.10.06: annotate with $(Protobuf.Package("")) if there's a package name

	// TODO 2023.10.10: convert Options

	return &proto.Module{
		URI:     *fileDescriptor.Name,
		UID:     moduleUID,
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
	fields, err := mapFrom(descriptor.Field, fromFieldDescriptorProto)
	if err != nil {
		return nil, err
	}

	// TODO 2023.10.10: convert Options

	return &proto.Struct{
		Reference: &proto.TypeReference{},
		Name: &proto.TypeName{
			Name:       *descriptor.Name,
			Parameters: nil,
		},
		Fields: fields,
		// Unions:
		// Reserved:
		// CommentBlock:
		// AnnotationsApplications:
		// IsSynthetic:
	}, nil
}

func fromFieldDescriptorProto(fieldDescriptor *descriptorpb.FieldDescriptorProto) (*proto.Field, error) {
	qualifier := ""
	typeName := ""
	if fieldDescriptor.Type == nil || *fieldDescriptor.Type == descriptorpb.FieldDescriptorProto_TYPE_GROUP || *fieldDescriptor.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE || *fieldDescriptor.Type == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		qualifier, typeName = fromTypeString(*fieldDescriptor.TypeName)
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

	// TODO 2023.10.10: convert Options

	return &proto.Field{
		Reference: &proto.AttributeReference{
			// ModuleUID:
			// TypeUID:
			AttributeUID: (uint64)(*fieldDescriptor.Number),
		},
		Name: *fieldDescriptor.Name,
		Type: &proto.TypeSpecifier{
			Reference: nil,
			Qualifier: qualifier,
			Name: &proto.TypeName{
				Name:       typeName,
				Parameters: nil,
			},
			// IsList:
			// IsMap:
			// HasPresence:
		},

		// TODO 2023.10.10: fieldDescriptor.DefaultValue is a string, whereas
		// proto.Field.DefaultValue is a proto.Value. Can we safely call
		// the microglot *parser* from here??? Should we do something else???
		// DefaultValue:

		// UnionUID:
		// CommentBlock:
		// AnnotationApplications:
	}, nil
}

func fromEnumDescriptorProto(enumDescriptor *descriptorpb.EnumDescriptorProto) (*proto.Enum, error) {
	enumerants, err := mapFrom(enumDescriptor.Value, fromEnumValueDescriptorProto)
	if err != nil {
		return nil, err
	}

	// TODO 2023.10.10: convert Options

	return &proto.Enum{
		Reference:  &proto.TypeReference{},
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
			// ModuleUID:
			// TypeUID:
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
		Reference: &proto.TypeReference{},
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

	inputQualifier, inputTypeName := fromTypeString(*methodDescriptor.InputType)
	outputQualifier, outputTypeName := fromTypeString(*methodDescriptor.OutputType)

	// TODO 2023.10.10: convert Options

	return &proto.APIMethod{
		Reference: &proto.AttributeReference{},
		Name:      *methodDescriptor.Name,
		Input: &proto.TypeSpecifier{
			Reference: nil,
			Qualifier: inputQualifier,
			Name: &proto.TypeName{
				Name:       inputTypeName,
				Parameters: nil,
			},
			// IsList:
			// IsMap:
			// HasPresence:
		},
		Output: &proto.TypeSpecifier{
			Reference: nil,
			Qualifier: outputQualifier,
			Name: &proto.TypeName{
				Name:       outputTypeName,
				Parameters: nil,
			},
			// IsList:
			// IsMap:
			// HasPresence:
		},
		// CommentBlock
		// AnnotationApplication
	}, nil
}

func fromTypeString(typeString string) (string, string) {
	qualifier := ""
	typeName := ""
	segments := strings.Split(typeString, ".")
	if len(segments) > 1 {
		qualifier = strings.Join(segments[:len(segments)-1], ".")
	}
	typeName = segments[len(segments)-1]
	return qualifier, typeName
}
