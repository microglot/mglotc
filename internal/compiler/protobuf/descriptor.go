package protobuf

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"

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

        // TODO 2023.10.06: annotate with $(Protobuf.Package("")) if there's a package name

	return &proto.Module{
		URI: *fileDescriptor.Name,
		UID: moduleUID,
		// Imports
		Structs: structs,
		Enums:   enums,
		// APIs
		// SDKs
		// Constants
		// Annotations
		// DotImports
	}, nil
}

func fromDescriptorProto(descriptor *descriptorpb.DescriptorProto) (*proto.Struct, error) {
	return &proto.Struct{
		Reference: &proto.TypeReference{},
		Name: &proto.TypeName{
			Name: *descriptor.Name,
			// Parameters:
		},
		// Fields:
		// Unions:
		// Reserved:
		// CommentBlock:
		// AnnotationsApplications:
		// IsSynthetic:
	}, nil
}

func fromEnumDescriptorProto(enumDescriptor *descriptorpb.EnumDescriptorProto) (*proto.Enum, error) {
	return &proto.Enum{
		Reference: &proto.TypeReference{},
		Name:      *enumDescriptor.Name,
		// Enumerants:
		// Reserved:
		// ReservedNames:
		// CommentBlock:
		// AnnotationApplications:
	}, nil
}
