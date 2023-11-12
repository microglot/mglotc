package compiler

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
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
			_ = r.Report(exc.New(exc.Location{
				URI: parsed.URI,
				// TODO 2023.09.12: getting Location here would sure be nice!
			}, exc.CodeUnknownFatal, fmt.Sprintf("unknown import %s", import_.ImportedURI)))
		}
	}

	// populate all the TypeSpecifiers
	walkModule(&parsed, func(node interface{}) {
		switch n := node.(type) {
		case *proto.TypeSpecifier:
			switch kind := n.Reference.(type) {
			case *proto.TypeSpecifier_Forward:
				var sym proto.TypeReference
				var parameters []*proto.TypeSpecifier
				var ok bool
				var fullName string
				switch reference := kind.Forward.Reference.(type) {
				case *proto.ForwardReference_Microglot:
					fullName = fmt.Sprintf("%s.%s", reference.Microglot.Qualifier, reference.Microglot.Name.Name)
					sym, ok = symbols.types[localSymbolName{
						qualifier: reference.Microglot.Qualifier,
						name:      reference.Microglot.Name.Name,
					}]

					parameters = reference.Microglot.Name.Parameters
				case *proto.ForwardReference_Protobuf:
					fullName = reference.Protobuf
					sym, ok = gsymbols.packageSearch(parsed.ProtobufPackage, reference.Protobuf)

					// this is how we deal with built-in types in protobuf, for now,
					// but it definitely feels a little bit off.
					if (!ok) && (!strings.Contains(reference.Protobuf, ".")) {
						sym, ok = symbols.types[localSymbolName{
							qualifier: "",
							name:      reference.Protobuf,
						}]
					}
				}
				if !ok {
					// TODO 2023.11.01: replace CodeUnknownFatal with something more meaningful
					_ = r.Report(exc.New(exc.Location{
						URI: parsed.URI,
						// TODO 2023.11.01: getting Location here would sure be nice!
					}, exc.CodeUnknownFatal, fmt.Sprintf("unknown type %s", fullName)))
				} else {
					n.Reference = &proto.TypeSpecifier_Resolved{
						Resolved: &proto.ResolvedReference{
							Reference:  &sym,
							Parameters: parameters,
						},
					}
				}
			}
		case *proto.ValueIdentifier:
			// TODO 2023.09.23: the ambiguity of whether the first component of the ValueIdentifier
			// is a qualifier or a type seems... off?
			possibleSymbolNames := []localSymbolName{
				localSymbolName{
					qualifier: "",
					name:      strings.Join(n.Names, "."),
				},
			}
			if len(n.Names) > 1 {
				possibleSymbolNames = append(possibleSymbolNames, localSymbolName{
					qualifier: n.Names[0],
					name:      strings.Join(n.Names[1:], "."),
				})
			}

			for _, symbolName := range possibleSymbolNames {
				type_, ok := symbols.types[symbolName]
				if ok {
					n.Reference = &proto.ValueIdentifier_Type{
						Type: &type_,
					}
					return
				}
				attribute, ok := symbols.attributes[symbolName]
				if ok {
					n.Reference = &proto.ValueIdentifier_Attribute{
						Attribute: &attribute,
					}
					return
				}
			}

			// TODO 2023.09.23: replace CodeUnknownFatal with something more meaningful
			_ = r.Report(exc.New(exc.Location{
				URI: parsed.URI,
				// TODO 2023.09.23: getting Location here would sure be nice!
			}, exc.CodeUnknownFatal, fmt.Sprintf("unknown identifier: %s", strings.Join(n.Names, "."))))
		}
	})

	// TODO 2023.09.23: populate all the ValueIdentifier.Reference

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
	types      map[localSymbolName]proto.TypeReference
	attributes map[localSymbolName]proto.AttributeReference
	inputs     map[localSymbolName]proto.SDKInputReference
}

func newLocalSymbols(gsymbols *globalSymbolTable, URI string) *localSymbolTable {
	symbols := localSymbolTable{}
	symbols.types = make(map[localSymbolName]proto.TypeReference)
	symbols.attributes = make(map[localSymbolName]proto.AttributeReference)
	symbols.inputs = make(map[localSymbolName]proto.SDKInputReference)

	for builtinTypeName, builtinTypeUID := range idl.BUILTIN_TYPE_UIDS {
		symbols.types[localSymbolName{
			qualifier: "",
			name:      builtinTypeName,
		}] = proto.TypeReference{
			// moduleUID 0 is for built-in types
			ModuleUID: 0,
			TypeUID:   builtinTypeUID,
		}
	}

	for protobufTypeName, protobufTypeUID := range idl.PROTOBUF_TYPE_UIDS {
		symbols.types[localSymbolName{
			qualifier: "Protobuf",
			name:      protobufTypeName,
		}] = proto.TypeReference{
			// moduleUID 1 is for Protobuf annotations
			ModuleUID: 1,
			TypeUID:   protobufTypeUID,
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

	attributeTypes, ok := gsymbols.attributes[URI]
	if !ok {
		return false
	}

	for typeName, attributes := range attributeTypes {
		for name, ref := range attributes {
			s.attributes[localSymbolName{
				qualifier: alias,
				name:      fmt.Sprintf("%s.%s", typeName, name),
			}] = ref
		}
	}

	inputTypes, ok := gsymbols.inputs[URI]
	if !ok {
		return false
	}

	for typeName, attributes := range inputTypes {
		for attributeName, inputs := range attributes {
			for name, ref := range inputs {
				s.inputs[localSymbolName{
					qualifier: alias,
					name:      fmt.Sprintf("%s.%s.%s", typeName, attributeName, name),
				}] = ref
			}
		}
	}

	return true
}
