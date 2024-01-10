package idl

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"

	"gopkg.microglot.org/compiler.go/internal/proto"
)

func (image *Image) ToFileDescriptorSet() (*descriptorpb.FileDescriptorSet, error) {
	converter := imageConverter{
		image: image,
	}
	return converter.convert()
}

type imageConverter struct {
	image *Image
}

func (c *imageConverter) convert() (*descriptorpb.FileDescriptorSet, error) {
	files := make([]*descriptorpb.FileDescriptorProto, 0, len(c.image.Modules))
	for _, module := range c.image.Modules {
		file, err := c.fromModule(module)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return &descriptorpb.FileDescriptorSet{
		File: files,
	}, nil
}

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

func GetProtobufAnnotation(as []*proto.AnnotationApplication, name string) *proto.Value {
	for _, annotation := range as {
		resolvedReference, ok := annotation.Annotation.Reference.(*proto.TypeSpecifier_Resolved)
		if !ok {
			continue
		}
		typeReference := *(resolvedReference.Resolved.Reference)
		if typeReference.ModuleUID == 2 && typeReference.TypeUID == PROTOBUF_TYPE_UIDS[name] {
			return annotation.Value
		}
	}
	return nil
}

func getProtobufAnnotationString(as []*proto.AnnotationApplication, name string) *string {
	value := GetProtobufAnnotation(as, name)
	if value == nil {
		return nil
	}
	return &value.Kind.(*proto.Value_Text).Text.Value
}

func GetPromotedSymbolTable(as []*proto.AnnotationApplication) map[string]string {
	promotedSymbolTable := make(map[string]string)
	nestedTypeInfo := GetProtobufAnnotation(as, "NestedTypeInfo")
	if nestedTypeInfo != nil {
		elements := nestedTypeInfo.Kind.(*proto.Value_Struct).Struct.Fields[0].Value.Kind.(*proto.Value_List).List.Elements
		for _, element := range elements {
			from := element.Kind.(*proto.Value_Struct).Struct.Fields[0].Value.Kind.(*proto.Value_Text).Text.Value
			to := element.Kind.(*proto.Value_Struct).Struct.Fields[1].Value.Kind.(*proto.Value_Text).Text.Value
			promotedSymbolTable[from] = to
		}
	}
	return promotedSymbolTable
}

func (c *imageConverter) lookupStruct(moduleUID uint64, structName string) *proto.Struct {
	for _, module := range c.image.Modules {
		if module.UID == moduleUID {
			for _, struct_ := range module.Structs {
				if struct_.Name.Name == structName {
					return struct_
				}
			}
		}
	}
	return nil
}

func (c *imageConverter) lookupEnum(moduleUID uint64, enumName string) *proto.Enum {
	for _, module := range c.image.Modules {
		if module.UID == moduleUID {
			for _, enum := range module.Enums {
				if enum.Name == enumName {
					return enum
				}
			}
		}
	}
	return nil
}

func (c *imageConverter) getNestedName(moduleUID uint64, name string) string {
	for _, module := range c.image.Modules {
		if module.UID == moduleUID {
			for _, struct_ := range module.Structs {
				for nestedName, promotedName := range GetPromotedSymbolTable(struct_.AnnotationApplications) {
					if promotedName == name {
						return nestedName
					}
				}
			}
			for _, enum := range module.Enums {
				for nestedName, promotedName := range GetPromotedSymbolTable(enum.AnnotationApplications) {
					if promotedName == name {
						return nestedName
					}
				}
			}
		}
	}
	return name
}

func (c *imageConverter) isPromotedType(moduleUID uint64, name string) bool {
	return c.getNestedName(moduleUID, name) != name
}

func (c *imageConverter) getQualifiedName(protobufPackage string, moduleUID uint64, name string) string {
	nestedName := c.getNestedName(moduleUID, name)
	if nestedName != name {
		return nestedName
	}
	return fmt.Sprintf("%s.%s", protobufPackage, name)
}

func (c *imageConverter) fromModule(module *proto.Module) (*descriptorpb.FileDescriptorProto, error) {
	// TODO 2023.11.03: is it a fatal error to attempt to convert a microglot Module that contains
	// stuff that cannot be represented in protobuf, e.g. SDKs? Or is this conversion allowed to be
	// lossy?

	var dependencies []string
	for _, import_ := range module.Imports {
		dependencies = append(dependencies, import_.ImportedURI)
	}

	var messageTypes []*descriptorpb.DescriptorProto
	for _, struct_ := range module.Structs {
		if !c.isPromotedType(module.UID, struct_.Name.Name) {
			messageType, err := c.fromStruct(struct_)
			if err != nil {
				return nil, err
			}
			messageTypes = append(messageTypes, messageType)
		}
	}

	var enumTypes []*descriptorpb.EnumDescriptorProto
	for _, enum := range module.Enums {
		if !c.isPromotedType(module.UID, enum.Name) {
			enumType, err := c.fromEnum(enum)
			if err != nil {
				return nil, err
			}
			enumTypes = append(enumTypes, enumType)
		}
	}

	var services []*descriptorpb.ServiceDescriptorProto
	for _, api := range module.APIs {
		service, err := c.fromAPI(api)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	syntax := "proto3"

	// TODO 2023.11.20: this is a little bit suspicious, and very possibly wrong.
	name := strings.TrimLeft(module.URI, "/")

	return &descriptorpb.FileDescriptorProto{
		Name:       &name,
		Package:    &module.ProtobufPackage,
		Dependency: dependencies,
		// PublicDependency
		// WeakDependency
		MessageType: messageTypes,
		EnumType:    enumTypes,
		Service:     services,
		// Extension

		Options: &descriptorpb.FileOptions{
			GoPackage: getProtobufAnnotationString(module.AnnotationApplications, "FileOptionsGoPackage"),
			// TODO 2023.12.30: remaining options
		},

		// SourceCodeInfo
		Syntax: &syntax,
		// Edition
	}, nil
}

func (c *imageConverter) fromStruct(struct_ *proto.Struct) (*descriptorpb.DescriptorProto, error) {
	fields, err := mapFrom(struct_.Fields, c.fromField)
	if err != nil {
		return nil, err
	}

	oneofs, err := mapFrom(struct_.Unions, c.fromUnion)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		if field.Proto3Optional != nil && *field.Proto3Optional {
			// TODO 2023.11.12: there's presumably a naming convention for these synthetic oneofs.
			name := "synthetic"
			oneofs = append(oneofs, &descriptorpb.OneofDescriptorProto{
				Name: &name,
			})
			oneofIndex := (int32)(len(oneofs) - 1)
			field.OneofIndex = &oneofIndex
		}
	}

	var options *descriptorpb.MessageOptions
	if struct_.IsSynthetic {
		options = new(descriptorpb.MessageOptions)
		options.MapEntry = new(bool)
		*(options.MapEntry) = true
	}

	var nestedType []*descriptorpb.DescriptorProto
	var enumType []*descriptorpb.EnumDescriptorProto
	for nestedName, promotedName := range GetPromotedSymbolTable(struct_.AnnotationApplications) {
		maybeStruct := c.lookupStruct(struct_.Reference.ModuleUID, promotedName)
		if maybeStruct == nil {
			maybeEnum := c.lookupEnum(struct_.Reference.ModuleUID, promotedName)
			if maybeEnum == nil {
				return nil, fmt.Errorf("unexpectedly missing promoted type named '%s'", promotedName)
			}
			maybeEnumType, err := c.fromEnum(maybeEnum)
			if err != nil {
				return nil, err
			}
			*maybeEnumType.Name = nestedName
			enumType = append(enumType, maybeEnumType)
		} else {
			maybeNestedType, err := c.fromStruct(maybeStruct)
			if err != nil {
				return nil, err
			}
			*maybeNestedType.Name = nestedName
			nestedType = append(nestedType, maybeNestedType)
		}
	}

	return &descriptorpb.DescriptorProto{
		Name:       &struct_.Name.Name,
		Field:      fields,
		OneofDecl:  oneofs,
		Options:    options,
		NestedType: nestedType,
		EnumType:   enumType,
		// Extension
		// ExtensionRange
		// ReservedRange
		// ReservedName
	}, nil
}

func (c *imageConverter) fromUnion(union *proto.Union) (*descriptorpb.OneofDescriptorProto, error) {
	return &descriptorpb.OneofDescriptorProto{
		Name: &union.Name,
		// Options
	}, nil
}

func (c *imageConverter) fromField(field *proto.Field) (*descriptorpb.FieldDescriptorProto, error) {
	number := (int32)(field.Reference.AttributeUID)

	label, type_, typeName, err := c.fromTypeSpecifier(field.Type)
	if err != nil {
		return nil, err
	}

	var oneofIndex *int32
	if field.UnionIndex != nil {
		oneofIndex = new(int32)
		*oneofIndex = (int32)(*field.UnionIndex)
	}

	var proto3Optional *bool
	if label != nil && *label == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
		proto3Optional = new(bool)
		*proto3Optional = true
	}

	return &descriptorpb.FieldDescriptorProto{
		Name:     &field.Name,
		Number:   &number,
		Label:    label,
		Type:     type_,
		TypeName: typeName,
		// Extendee
		// DefaultValue
		OneofIndex: oneofIndex,
		// JsonName
		// Options

		Proto3Optional: proto3Optional,
	}, nil
}

func (c *imageConverter) fromTypeSpecifier(typeSpecifier *proto.TypeSpecifier) (*descriptorpb.FieldDescriptorProto_Label, *descriptorpb.FieldDescriptorProto_Type, *string, error) {
	resolved, ok := typeSpecifier.Reference.(*proto.TypeSpecifier_Resolved)
	if !ok {
		return nil, nil, nil, errors.New("unexpected forward reference while converting descriptor to protobuf!")
	}

	label, type_, typeName, err := c.fromResolvedReference(resolved.Resolved)
	if err != nil {
		return nil, nil, nil, err
	}
	return label, type_, typeName, nil
}

func (c *imageConverter) fromResolvedReference(resolvedReference *proto.ResolvedReference) (*descriptorpb.FieldDescriptorProto_Label, *descriptorpb.FieldDescriptorProto_Type, *string, error) {

	// moduleUID 0 is for built-in types
	if resolvedReference.Reference.ModuleUID == 0 {
		builtinTypeName, ok := GetBuiltinTypeNameFromUID(resolvedReference.Reference.TypeUID)
		if !ok {
			return nil, nil, nil, fmt.Errorf("unknown built-in type UID: %d\n", resolvedReference.Reference.TypeUID)
		}

		// TODO 2023.11.09: respect $(Protobuf.FieldType())

		switch builtinTypeName.Name {
		case "Bool":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_BOOL
			return nil, &type_, nil, nil
		case "Text":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_STRING
			return nil, &type_, nil, nil
		case "Data":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_BYTES
			return nil, &type_, nil, nil
		case "Int8":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_INT32
			return nil, &type_, nil, nil
		case "Int16":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_INT32
			return nil, &type_, nil, nil
		case "Int32":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_INT32
			return nil, &type_, nil, nil
		case "Int64":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_INT64
			return nil, &type_, nil, nil
		case "UInt8":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_UINT32
			return nil, &type_, nil, nil
		case "UInt16":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_UINT32
			return nil, &type_, nil, nil
		case "UInt32":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_UINT32
			return nil, &type_, nil, nil
		case "UInt64":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_UINT64
			return nil, &type_, nil, nil
		case "Float32":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_FLOAT
			return nil, &type_, nil, nil
		case "Float64":
			type_ := descriptorpb.FieldDescriptorProto_TYPE_DOUBLE
			return nil, &type_, nil, nil
		case "List":
			_, type_, typeName, err := c.fromTypeSpecifier(resolvedReference.Parameters[0])
			if err != nil {
				return nil, nil, nil, err
			}
			label := descriptorpb.FieldDescriptorProto_LABEL_REPEATED
			return &label, type_, typeName, nil
		case "Presence":
			_, type_, typeName, err := c.fromTypeSpecifier(resolvedReference.Parameters[0])
			if err != nil {
				return nil, nil, nil, err
			}
			label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
			return &label, type_, typeName, nil
		default:
			return nil, nil, nil, fmt.Errorf("built-in type %v doesn't convert to protobuf", builtinTypeName)
		}
	}

	for _, module := range c.image.Modules {
		if module.UID == resolvedReference.Reference.ModuleUID {
			for _, struct_ := range module.Structs {
				if struct_.Reference.TypeUID == resolvedReference.Reference.TypeUID {
					type_ := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
					typeName := c.getQualifiedName(module.ProtobufPackage, struct_.Reference.ModuleUID, struct_.Name.Name)
					return nil, &type_, &typeName, nil
				}
			}
			for _, enum := range module.Enums {
				if enum.Reference.TypeUID == resolvedReference.Reference.TypeUID {
					type_ := descriptorpb.FieldDescriptorProto_TYPE_ENUM
					typeName := c.getQualifiedName(module.ProtobufPackage, enum.Reference.ModuleUID, enum.Name)
					return nil, &type_, &typeName, nil
				}
			}
			for _, api := range module.APIs {
				if api.Reference.TypeUID == resolvedReference.Reference.TypeUID {
					return nil, nil, nil, fmt.Errorf("can't use an API (%s) as a protobuf type", api.Name)
				}
			}
			for _, sdk := range module.SDKs {
				if sdk.Reference.TypeUID == resolvedReference.Reference.TypeUID {
					return nil, nil, nil, fmt.Errorf("can't use an SDK (%s) as a protobuf type", sdk.Name)
				}
			}
			for _, annotation := range module.Annotations {
				if annotation.Reference.TypeUID == resolvedReference.Reference.TypeUID {
					return nil, nil, nil, fmt.Errorf("can't use an Annotation (%s) as a protobuf type", annotation.Name)
				}
			}
			for _, constant := range module.Constants {
				if constant.Reference.TypeUID == resolvedReference.Reference.TypeUID {
					return nil, nil, nil, fmt.Errorf("can't use a Constant (%s) as a protobuf type", constant.Name)
				}
			}
		}
	}

	return nil, nil, nil, fmt.Errorf("linked type with moduleUID=%d and typeUID=%d wasn't found in the image!", resolvedReference.Reference.ModuleUID, resolvedReference.Reference.TypeUID)
}

func (c *imageConverter) fromEnum(enum *proto.Enum) (*descriptorpb.EnumDescriptorProto, error) {
	values, err := mapFrom(enum.Enumerants, c.fromEnumerant)
	if err != nil {
		return nil, err
	}

	return &descriptorpb.EnumDescriptorProto{
		Name:  &enum.Name,
		Value: values,
		// Options
		// ReservedRange
		// ReservedName
	}, nil
}

func (c *imageConverter) fromEnumerant(enumerant *proto.Enumerant) (*descriptorpb.EnumValueDescriptorProto, error) {
	number := (int32)(enumerant.Reference.AttributeUID)
	return &descriptorpb.EnumValueDescriptorProto{
		Name:   &enumerant.Name,
		Number: &number,
		// Options
	}, nil
}

func (c *imageConverter) fromAPI(api *proto.API) (*descriptorpb.ServiceDescriptorProto, error) {
	methods, err := mapFrom(api.Methods, c.fromAPIMethod)
	if err != nil {
		return nil, err
	}

	return &descriptorpb.ServiceDescriptorProto{
		Name:   &api.Name.Name,
		Method: methods,
		// Options
	}, nil
}

func (c *imageConverter) fromAPIMethod(apiMethod *proto.APIMethod) (*descriptorpb.MethodDescriptorProto, error) {
	_, _, inputTypeName, err := c.fromTypeSpecifier(apiMethod.Input)
	if err != nil {
		return nil, err
	}
	_, _, outputTypeName, err := c.fromTypeSpecifier(apiMethod.Output)
	if err != nil {
		return nil, err
	}

	return &descriptorpb.MethodDescriptorProto{
		Name:       &apiMethod.Name,
		InputType:  inputTypeName,
		OutputType: outputTypeName,
		// Options:
		// ClientStreaming:
		// ServerStreaming:
	}, nil
}
