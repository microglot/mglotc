package compiler

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

type moduleMeta struct {
	uid             uint64
	protobufPackage string
}

type globalSymbolTable struct {
	lock       sync.RWMutex
	modules    map[string]moduleMeta
	types      map[string]map[string]proto.TypeReference
	attributes map[string]map[string]map[string]proto.AttributeReference
	inputs     map[string]map[string]map[string]map[string]proto.SDKInputReference
}

// globalSymbolTable.collect() populates a symbol table with the symbols in a given descriptor
// reports: name collisions, UID collisions
func (s *globalSymbolTable) collect(parsed proto.Module, r exc.Reporter) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.modules == nil {
		s.modules = make(map[string]moduleMeta)
	}
	if s.types == nil {
		s.types = make(map[string]map[string]proto.TypeReference)
	}
	if s.attributes == nil {
		s.attributes = make(map[string]map[string]map[string]proto.AttributeReference)
	}
	if s.inputs == nil {
		s.inputs = make(map[string]map[string]map[string]map[string]proto.SDKInputReference)
	}

	if s.types[parsed.URI] == nil {
		s.types[parsed.URI] = make(map[string]proto.TypeReference)
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: parsed.URI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("types for '%s' have already been collected (????)", parsed.URI)))
		return errors.New("collect error")
	}
	if s.attributes[parsed.URI] == nil {
		s.attributes[parsed.URI] = make(map[string]map[string]proto.AttributeReference)
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: parsed.URI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("attributes for '%s' have already been collected (????)", parsed.URI)))
		return errors.New("collect error")
	}
	if s.inputs[parsed.URI] == nil {
		s.inputs[parsed.URI] = make(map[string]map[string]map[string]proto.SDKInputReference)
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: parsed.URI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("inputs for '%s' have already been collected (????)", parsed.URI)))
		return errors.New("collect error")
	}

	for moduleURI, moduleMeta := range s.modules {
		if moduleMeta.uid == parsed.UID {
			// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
			_ = r.Report(exc.New(exc.Location{
				URI: parsed.URI,
				// TODO 2023.09.14: getting Location here would be nice!
			}, exc.CodeUnknownFatal, fmt.Sprintf("module UID '%d' is already in-use by '%s'", parsed.UID, moduleURI)))
			return errors.New("collect error")
		}
	}
	s.modules[parsed.URI] = moduleMeta{
		uid:             parsed.UID,
		protobufPackage: parsed.ProtobufPackage,
	}

	typeUIDs := make(map[uint64]string)

	for _, struct_ := range parsed.Structs {
		s.addType(r, parsed.URI, struct_.Name.Name, struct_.Reference, typeUIDs)

		attributeUIDs := make(map[uint64]string)
		for _, field := range struct_.Fields {
			s.addAttribute(r, parsed.URI, struct_.Name.Name, field.Name, field.Reference, attributeUIDs)
		}
		for _, union := range struct_.Unions {
			s.addAttribute(r, parsed.URI, struct_.Name.Name, union.Name, union.Reference, attributeUIDs)
		}
	}
	for _, enum := range parsed.Enums {
		s.addType(r, parsed.URI, enum.Name, enum.Reference, typeUIDs)
		attributeUIDs := make(map[uint64]string)
		for _, enumerant := range enum.Enumerants {
			s.addAttribute(r, parsed.URI, enum.Name, enumerant.Name, enumerant.Reference, attributeUIDs)
		}
	}
	for _, api := range parsed.APIs {
		s.addType(r, parsed.URI, api.Name.Name, api.Reference, typeUIDs)
		attributeUIDs := make(map[uint64]string)
		for _, apiMethod := range api.Methods {
			s.addAttribute(r, parsed.URI, api.Name.Name, apiMethod.Name, apiMethod.Reference, attributeUIDs)
		}
	}
	for _, sdk := range parsed.SDKs {
		s.addType(r, parsed.URI, sdk.Name.Name, sdk.Reference, typeUIDs)
		attributeUIDs := make(map[uint64]string)
		for _, sdkMethod := range sdk.Methods {
			s.addAttribute(r, parsed.URI, sdk.Name.Name, sdkMethod.Name, sdkMethod.Reference, attributeUIDs)
			sdkMethodInputUIDs := make(map[uint64]string)
			for _, sdkMethodInput := range sdkMethod.Input {
				s.addSDKMethodInput(r, parsed.URI, sdk.Name.Name, sdkMethod.Name, sdkMethodInput.Name, sdkMethodInput.Reference, sdkMethodInputUIDs)
			}
		}
	}
	for _, annotation := range parsed.Annotations {
		s.addType(r, parsed.URI, annotation.Name, annotation.Reference, typeUIDs)
	}
	for _, constant := range parsed.Constants {
		s.addType(r, parsed.URI, constant.Name, constant.Reference, typeUIDs)
	}

	if len(r.Reported()) > 0 {
		return errors.New("collect error")
	}
	return nil
}

func (s *globalSymbolTable) addType(r exc.Reporter, moduleURI string, name string, typeReference *proto.TypeReference, typeUIDs map[uint64]string) {
	// Assumes we're already holding s.lock!

	if _, ok := typeUIDs[typeReference.TypeUID]; ok {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration with the uid '%d' in '%s'", typeReference.TypeUID, typeUIDs[typeReference.TypeUID])))
	}
	typeUIDs[typeReference.TypeUID] = moduleURI

	if _, ok := s.types[moduleURI][name]; ok {
		// TODO 2023.09.12: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.12: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration of '%s' in '%s'", name, moduleURI)))
	}

	// We consider it an error to have the more than one declaration of the same typename in a given
	// *protobuf* package.
	//  * this is *required* for .proto compilation and linking
	//  * it is *assumed* by protobuf plugins (even if we're passing them descriptors compiled from mgdl!)
	//  * it is hopefully rare to trigger accidentally, given how we assign default protobuf package names
	for uri, meta := range s.modules {
		if meta.protobufPackage == s.modules[moduleURI].protobufPackage {
			if _, ok := s.types[uri][name]; ok {
				// TODO 2023.11.01: replace CodeUnknownFatal with something more meaningful
				_ = r.Report(exc.New(exc.Location{
					URI: moduleURI,
					// TODO 2023.11.01: getting Location here would be nice!
				}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration of '%s.%s' in '%s'", meta.protobufPackage, name, uri)))
			}
		}
	}

	s.types[moduleURI][name] = *typeReference
}

func (s *globalSymbolTable) addAttribute(r exc.Reporter, moduleURI string, typeName string, name string, attributeReference *proto.AttributeReference, attributeUIDs map[uint64]string) {
	// Assumes we're already holding s.lock!

	if _, ok := attributeUIDs[attributeReference.AttributeUID]; ok {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration with the uid '%d' in '%s'", attributeReference.AttributeUID, attributeUIDs[attributeReference.AttributeUID])))
	}
	attributeUIDs[attributeReference.AttributeUID] = typeName

	if s.attributes[moduleURI][typeName] == nil {
		s.attributes[moduleURI][typeName] = make(map[string]proto.AttributeReference)
	}

	if _, ok := s.attributes[moduleURI][typeName][name]; ok {
		// TODO 2023.09.12: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.12: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration of '%s' in '%s'", name, typeName)))
	} else {
		s.attributes[moduleURI][typeName][name] = *attributeReference
	}
}

func (s *globalSymbolTable) addSDKMethodInput(r exc.Reporter, moduleURI string, typeName string, sdkMethodName string, name string, sdkInputReference *proto.SDKInputReference, sdkMethodInputUIDs map[uint64]string) {
	// Assumes we're already holding s.lock!

	if _, ok := sdkMethodInputUIDs[sdkInputReference.AttributeUID]; ok {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration with the uid '%d' in '%s'", sdkInputReference.AttributeUID, sdkMethodInputUIDs[sdkInputReference.AttributeUID])))
	}
	sdkMethodInputUIDs[sdkInputReference.AttributeUID] = typeName

	if s.inputs[moduleURI][typeName] == nil {
		s.inputs[moduleURI][typeName] = make(map[string]map[string]proto.SDKInputReference)
	}
	if s.inputs[moduleURI][typeName][sdkMethodName] == nil {
		s.inputs[moduleURI][typeName][sdkMethodName] = make(map[string]proto.SDKInputReference)
	}

	if _, ok := s.inputs[moduleURI][typeName][sdkMethodName][name]; ok {
		// TODO 2023.09.12: replace CodeUnknownFatal with something more meaningful
		_ = r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.12: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration of '%s' in '%s'", name, typeName)))
	} else {
		s.inputs[moduleURI][typeName][sdkMethodName][name] = *sdkInputReference
	}
}

func (s *globalSymbolTable) packageSearch(pkg string, name string) (proto.TypeReference, bool) {
	// build segmentPackages, which is the list of packages to search, i.e. if the current package is
	// "outer.inner", it will be ["", "outer", "outer.inner"]
	segmentPackage := ""
	segmentPackages := []string{segmentPackage}
	for _, segment := range strings.Split(pkg, ".") {
		if segmentPackage == "" {
			segmentPackage = segment
		} else {
			segmentPackage = segmentPackage + "." + segment
		}
		segmentPackages = append(segmentPackages, segmentPackage)
	}

	// if the name starts with '.', remove it. Otherwise, reverse the search order.
	if !strings.HasPrefix(name, ".") {
		slices.Reverse(segmentPackages)
	} else {
		name = name[1:]
	}

	// now divide the name into qualifier and base
	nameSegments := strings.Split(name, ".")
	qualifier := ""
	base := nameSegments[len(nameSegments)-1]
	if len(nameSegments) > 1 {
		qualifier = strings.Join(nameSegments[0:len(nameSegments)-1], ".")
	}

	// and search, in segmentPackages order!
	for _, segmentPackage := range segmentPackages {
		// fullPackage is the segmentPackage plus the qualifier part of the name
		fullPackage := segmentPackage
		if qualifier != "" {
			if fullPackage != "" {
				fullPackage = fullPackage + "." + qualifier
			} else {
				fullPackage = qualifier
			}
		}

		// look for exact package matches
		for uri, meta := range s.modules {
			if meta.protobufPackage == fullPackage {
				// we're in a matching package. Is 'base' in the type symbol table?
				sym, ok := s.types[uri][base]
				if ok {
					return sym, true
				}
			}
		}
	}
	return proto.TypeReference{}, false
}
