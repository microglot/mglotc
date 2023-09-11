package compiler

import (
	"fmt"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

// link() takes a parsed Module descriptor + optional input symbol table, and outputs a linked Module
// descriptor and the exported symbol table from this module.
func link(parsed *proto.Module, r exc.Reporter) (*proto.Module, *symbolTable) {
	// TODO 2023.09.11: all imported modules need to be compiled first, and their (linked) descriptors
	// and exported symbol tables passed in

	symbols := symbolTable{}
	symbols.types = make(map[string]proto.TypeReference)

	// TODO 2023.09.11: figure out how to handle built-in types
	// TODO 2023.09.11: "alias" all of the dependencies' symbols into the local symbol table

	// populate the local symbol table, and populate all declaration references in the descriptor
	// TODO 2023.09.11: add AttributeReference and SDKInputReference
	// TODO 2023.09.11: are APIs, SDKs and Annotation "types"? Or some other kind of symbol?
	// TODO 2023.09.11: Constants don't seem like "types", so store them in a different table?
	var type_uid uint64 = 0
	for _, struct_ := range parsed.Structs {
		type_uid++
		struct_.Reference = &proto.TypeReference{
			ModuleUID: parsed.UID,
			TypeUID:   type_uid,
		}

		symbols.types[struct_.Name.Name] = *struct_.Reference
	}
	for _, enum := range parsed.Enums {
		type_uid++
		enum.Reference = &proto.TypeReference{
			ModuleUID: parsed.UID,
			TypeUID:   type_uid,
		}

		symbols.types[enum.Name] = *enum.Reference
	}

	// populate all the TypeSpecifiers
	walkTypeSpecifiers(parsed, func(typeSpecifier *proto.TypeSpecifier) {
		sym, ok := symbols.types[typeSpecifier.Name.Name]
		if !ok {
			// TODO 2023.09.11: replace CodeUnknownFatal with something more meaningful
			r.Report(exc.New(exc.Location{
				URI: parsed.URI,
				// TODO 2023.09.11: getting Location here would sure be nice!
			}, exc.CodeUnknownFatal, fmt.Sprintf("unknown type %s", typeSpecifier.Name.Name)))
		} else {
			typeSpecifier.Reference = &sym
		}
	})

	// TODO 2023.09.11: we probably need to differentiate between the internal symbol table (used while
	// linking *this* module) and the external one, because of import-aliasing.

	return parsed, &symbols
}

type symbolTable struct {
	types map[string]proto.TypeReference
}

// TODO 2023.09.11: will almost certainly need a more general walk() fn, but this is okay for now
func walkTypeSpecifiers(parsed *proto.Module, f func(*proto.TypeSpecifier)) {
	// TODO 2023.09.11: Currently we are *NOT* walking TypeName.Parameters (generics)
	for _, struct_ := range parsed.Structs {
		for _, field := range struct_.Fields {
			f(field.Type)
			for _, annotation := range field.AnnotationApplications {
				f(annotation.Annotation)
			}
		}
		for _, union := range struct_.Unions {
			for _, annotation := range union.AnnotationApplications {
				f(annotation.Annotation)
			}
		}
		for _, annotation := range struct_.AnnotationApplications {
			f(annotation.Annotation)
		}
	}
	for _, enum := range parsed.Enums {
		for _, enumerant := range enum.Enumerants {
			for _, annotation := range enumerant.AnnotationApplications {
				f(annotation.Annotation)
			}
		}
		for _, annotation := range enum.AnnotationApplications {
			f(annotation.Annotation)
		}
	}
	for _, api := range parsed.APIs {
		for _, extends := range api.Extends {
			f(extends)
		}
		for _, method := range api.Methods {
			f(method.Input)
			f(method.Output)
			for _, annotation := range method.AnnotationApplications {
				f(annotation.Annotation)
			}
		}
		for _, annotation := range api.AnnotationApplications {
			f(annotation.Annotation)
		}
	}
	for _, sdk := range parsed.SDKs {
		for _, extends := range sdk.Extends {
			f(extends)
		}
		for _, method := range sdk.Methods {
			for _, input := range method.Input {
				f(input.Type)
			}
			f(method.Output)
			for _, annotation := range method.AnnotationApplications {
				f(annotation.Annotation)
			}
		}
		for _, annotation := range sdk.AnnotationApplications {
			f(annotation.Annotation)
		}
	}
	for _, constant := range parsed.Constants {
		f(constant.Type)
		for _, annotation := range constant.AnnotationApplications {
			f(annotation.Annotation)
		}
	}
}
