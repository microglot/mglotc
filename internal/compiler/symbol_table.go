package compiler

import (
	"errors"
	"fmt"
	"sync"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

type globalSymbolTable struct {
	lock       sync.RWMutex
	modules    map[string]uint64
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
		s.modules = make(map[string]uint64)
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

	for moduleURI, moduleUID := range s.modules {
		if moduleUID == parsed.UID {
			// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
			_ = r.Report(exc.New(exc.Location{
				URI: parsed.URI,
				// TODO 2023.09.14: getting Location here would be nice!
			}, exc.CodeUnknownFatal, fmt.Sprintf("module UID '%d' is already in-use by '%s'", parsed.UID, moduleURI)))
			return errors.New("collect error")
		}
	}
	s.modules[parsed.URI] = parsed.UID

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
	} else {
		s.types[moduleURI][name] = *typeReference
	}
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
