package compiler

import (
	"errors"
	"fmt"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

// link() takes a parsed Module descriptor + global symbol table, and outputs a linked Module descriptor.
// reports: unresolved imports and unknown types
func link(parsed proto.Module, gsymbols *globalSymbolTable, r exc.Reporter) (*proto.Module, error) {
	symbols := newLocalSymbols(gsymbols, parsed.URI)
	if symbols == nil {
		return nil, errors.New("unable to initialize local symbol table (???)")
	}

	// alias all of the dependencies' symbols into the local symbol table
	for _, import_ := range parsed.Imports {
		if !symbols.alias(gsymbols, import_.ImportedURI, import_.Alias, import_.IsDot) {
			// TODO 2023.09.12: replace CodeUnknownFatal with something more meaningful
			r.Report(exc.New(exc.Location{
				URI: parsed.URI,
				// TODO 2023.09.12: getting Location here would sure be nice!
			}, exc.CodeUnknownFatal, fmt.Sprintf("unknown import %s", import_.ImportedURI)))
		}
	}

	// populate all the TypeSpecifiers
	walkTypeSpecifiers(&parsed, func(typeSpecifier *proto.TypeSpecifier) {
		sym, ok := symbols.types[localSymbolName{
			qualifier: typeSpecifier.Qualifier,
			name:      typeSpecifier.Name.Name,
		}]
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

	if len(r.Reported()) > 0 {
		return nil, errors.New("linking failed")
	}
	return &parsed, nil
}

type localSymbolName struct {
	// magic value "" means "no qualifier" (same as proto.TypeSpecifier)
	qualifier string
	name      string
}

type localSymbolTable struct {
	types map[localSymbolName]proto.TypeReference
}

func newLocalSymbols(gsymbols *globalSymbolTable, URI string) *localSymbolTable {
	symbols := localSymbolTable{}
	symbols.types = make(map[localSymbolName]proto.TypeReference)

	// TODO 2023.09.14: bring in attributes and inputs!

	for _, internalTypeName := range []string{
		"Bool",
		"Text",
		"Data",
		"Int8",
		"Int16",
		"Int32",
		"Int64",
		"UInt8",
		"UInt16",
		"UInt32",
		"UInt64",
		"Float32",
		"Float64",
		"List",
		"Map",
		"Empty",
		"Presence",
		"AsyncTask",
	} {
		symbols.types[localSymbolName{
			qualifier: "",
			name:      internalTypeName,
		}] = proto.TypeReference{
			// moduleUID 0 is for built-in types
			ModuleUID: 0,
			// TODO 2023.09.12: just a shim to allow linking; this will need to be fleshed out
			TypeUID: 0,
		}
	}

	ok := symbols.alias(gsymbols, URI, "", false)
	if !ok {
		return nil
	}
	return &symbols
}

func (s *localSymbolTable) alias(gsymbols *globalSymbolTable, URI string, alias string, isDot bool) bool {
	gsymbols.lock.Lock()
	defer gsymbols.lock.Unlock()

	if isDot {
		alias = ""
	}

	names, ok := gsymbols.types[URI]
	if !ok {
		return false
	}

	for name, ref := range names {
		s.types[localSymbolName{
			qualifier: alias,
			name:      name,
		}] = ref
	}
	return true
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
