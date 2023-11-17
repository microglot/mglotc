package compiler

import (
	"gopkg.microglot.org/compiler.go/internal/proto"
)

func walkModule(module *proto.Module, f func(interface{})) {
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
	f(module)
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
	if field.DefaultValue != nil {
		walkValue(field.DefaultValue, f)
	}
	for _, annotation := range field.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	walkTypeSpecifier(field.Type, f)
	f(field)
}

func walkUnion(union *proto.Union, f func(interface{})) {
	for _, annotation := range union.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	f(union)
}

func walkAnnotationApplication(annotationApplication *proto.AnnotationApplication, f func(interface{})) {
	walkTypeSpecifier(annotationApplication.Annotation, f)
	walkValue(annotationApplication.Value, f)
	f(annotationApplication)
}

func walkEnum(enum *proto.Enum, f func(interface{})) {
	for _, enumerant := range enum.Enumerants {
		walkEnumerant(enumerant, f)
	}
	for _, annotation := range enum.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	f(enum)
}

func walkEnumerant(enumerant *proto.Enumerant, f func(interface{})) {
	for _, annotation := range enumerant.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	f(enumerant)
}

func walkAPI(api *proto.API, f func(interface{})) {
	for _, method := range api.Methods {
		walkAPIMethod(method, f)
	}
	for _, extends := range api.Extends {
		walkTypeSpecifier(extends, f)
	}
	for _, annotation := range api.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	f(api)
}

func walkAPIMethod(method *proto.APIMethod, f func(interface{})) {
	walkTypeSpecifier(method.Input, f)
	walkTypeSpecifier(method.Output, f)
	for _, annotation := range method.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	f(method)
}

func walkSDK(sdk *proto.SDK, f func(interface{})) {
	for _, method := range sdk.Methods {
		walkSDKMethod(method, f)
	}
	for _, extends := range sdk.Extends {
		walkTypeSpecifier(extends, f)
	}
	for _, annotation := range sdk.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	f(sdk)
}

func walkSDKMethod(method *proto.SDKMethod, f func(interface{})) {
	for _, input := range method.Input {
		walkTypeSpecifier(input.Type, f)
	}
	walkTypeSpecifier(method.Output, f)
	for _, annotation := range method.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	f(method)
}

func walkConstant(constant *proto.Constant, f func(interface{})) {
	walkTypeSpecifier(constant.Type, f)
	walkValue(constant.Value, f)
	for _, annotation := range constant.AnnotationApplications {
		walkAnnotationApplication(annotation, f)
	}
	f(constant)
}

func walkAnnotation(annotation *proto.Annotation, f func(interface{})) {
	for _, scope := range annotation.Scopes {
		f(scope)
	}
	walkTypeSpecifier(annotation.Type, f)
	f(annotation)
}

func walkTypeSpecifier(typeSpecifier *proto.TypeSpecifier, f func(interface{})) {
	switch r := typeSpecifier.Reference.(type) {
	case *proto.TypeSpecifier_Forward:
		switch kind := r.Forward.Reference.(type) {
		case *proto.ForwardReference_Microglot:
			for _, parameter := range kind.Microglot.Name.Parameters {
				walkTypeSpecifier(parameter, f)
			}
		}
	}
	f(typeSpecifier)
}

func walkValue(value *proto.Value, f func(interface{})) {
	switch v := value.Kind.(type) {
	case *proto.Value_Identifier:
		f(v.Identifier)
	}
}
