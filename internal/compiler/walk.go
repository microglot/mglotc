package compiler

import (
	"gopkg.microglot.org/compiler.go/internal/proto"
)

func walkModule(module *proto.Module, f func(interface{})) {
	f(module)
	for _, struct_ := range module.Structs {
		walkStruct(struct_, f)
	}
	for _, enum := range module.Enums {
		walkEnum(enum, f)
	}
	for _, api := range module.APIs {
		walkAPI(api, f)
	}
	for _, sdk := range module.SDKs {
		walkSDK(sdk, f)
	}
	for _, constant := range module.Constants {
		walkConstant(constant, f)
	}
	for _, annotation := range module.Annotations {
		walkAnnotation(annotation, f)
	}
}

func walkStruct(struct_ *proto.Struct, f func(interface{})) {
	f(struct_)
	for _, field := range struct_.Fields {
		walkField(field, f)
	}
	for _, union := range struct_.Unions {
		walkUnion(union, f)
	}
	for _, annotation := range struct_.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkField(field *proto.Field, f func(interface{})) {
	f(field)
	f(field.Type)
	if field.DefaultValue != nil {
		walkValue(field.DefaultValue, f)
	}
	for _, annotation := range field.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkUnion(union *proto.Union, f func(interface{})) {
	f(union)
	for _, annotation := range union.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkAnnotationApplication(annotationApplication *proto.AnnotationApplication, f func(interface{})) {
	f(annotationApplication)
	walkTypeSpecifier(annotationApplication.Annotation, f)
	walkValue(annotationApplication.Value, f)
}

func walkEnum(enum *proto.Enum, f func(interface{})) {
	f(enum)
	for _, enumerant := range enum.Enumerants {
		walkEnumerant(enumerant, f)
	}
	for _, annotation := range enum.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkEnumerant(enumerant *proto.Enumerant, f func(interface{})) {
	f(enumerant)
	for _, annotation := range enumerant.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkAPI(api *proto.API, f func(interface{})) {
	f(api)
	for _, method := range api.Methods {
		walkAPIMethod(method, f)
	}
	for _, extends := range api.Extends {
		walkTypeSpecifier(extends, f)
	}
	for _, annotation := range api.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkAPIMethod(method *proto.APIMethod, f func(interface{})) {
	f(method)
	walkTypeSpecifier(method.Input, f)
	walkTypeSpecifier(method.Output, f)
	for _, annotation := range method.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkSDK(sdk *proto.SDK, f func(interface{})) {
	f(sdk)
	for _, method := range sdk.Methods {
		walkSDKMethod(method, f)
	}
	for _, extends := range sdk.Extends {
		walkTypeSpecifier(extends, f)
	}
	for _, annotation := range sdk.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkSDKMethod(method *proto.SDKMethod, f func(interface{})) {
	f(method)
	for _, input := range method.Input {
		walkTypeSpecifier(input.Type, f)
	}
	walkTypeSpecifier(method.Output, f)
	for _, annotation := range method.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkConstant(constant *proto.Constant, f func(interface{})) {
	f(constant)
	walkTypeSpecifier(constant.Type, f)
	walkValue(constant.Value, f)
	for _, annotation := range constant.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
}

func walkAnnotation(annotation *proto.Annotation, f func(interface{})) {
	f(annotation)
	for _, scope := range annotation.Scopes {
		f(scope)
	}
	walkTypeSpecifier(annotation.Type, f)
}

func walkTypeSpecifier(typeSpecifier *proto.TypeSpecifier, f func(interface{})) {
	f(typeSpecifier)
	switch r := typeSpecifier.Reference.(type) {
	case *proto.TypeSpecifier_Forward:
		switch kind := r.Forward.Reference.(type) {
		case *proto.ForwardReference_Microglot:
			for _, parameter := range kind.Microglot.Name.Parameters {
				walkTypeSpecifier(parameter, f)
			}
		}
	}
}

func walkValue(value *proto.Value, f func(interface{})) {
	switch v := value.Kind.(type) {
	case *proto.Value_Identifier:
		f(v.Identifier)
	}
}
