package compiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
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
// as a side-effect, it generates any missing TypeUID values in the descriptor!
// reports: name collisions, UID collisions
func (s *globalSymbolTable) collect(parsed proto.Module, r exc.Reporter) (*proto.Module, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.modules == nil {
		s.modules = make(map[string]uint64)
	}
	if s.types == nil {
		s.types = make(map[string]map[string]proto.TypeReference)
	}

	// TODO 2023.09.14: allocate attributes, inputs!

	if s.types[parsed.URI] == nil {
		s.types[parsed.URI] = make(map[string]proto.TypeReference)
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: parsed.URI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("symbols for '%s' have already been collected (????)", parsed.URI)))
		return nil, errors.New("collect error")
	}
	for moduleURI, moduleUID := range s.modules {
		if moduleUID == parsed.UID {
			// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
			r.Report(exc.New(exc.Location{
				URI: parsed.URI,
				// TODO 2023.09.14: getting Location here would be nice!
			}, exc.CodeUnknownFatal, fmt.Sprintf("module UID '%d' is already in-use by '%s'", parsed.UID, moduleURI)))
			return nil, errors.New("collect error")
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
		return nil, errors.New("collect error")
	}
	return &parsed, nil
}

func newUID(parentUID uint64, name string) uint64 {
	hasher := sha256.New()
	err := binary.Write(hasher, binary.LittleEndian, parentUID)
	if err != nil {
		// IIUC this can only happen if something is *really* wrong (OOM?)
		panic(err)
	}
	hasher.Write([]byte(name))
	var typeUID uint64
	err = binary.Read(bytes.NewReader(hasher.Sum(nil)), binary.LittleEndian, &typeUID)
	if err != nil {
		// IIUC this can only happen if something is *really* wrong (OOM?)
		panic(err)
	}
	return typeUID
}

func (s *globalSymbolTable) addType(r exc.Reporter, moduleURI string, name string, typeReference *proto.TypeReference, typeUIDs map[uint64]string) {
	// Assumes we're already holding s.lock!

	moduleUID := s.modules[moduleURI]

	if typeReference.ModuleUID == 0 {
		typeReference.ModuleUID = moduleUID
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("the module UID for %s is already set, which shouldn't happen", name)))
	}
	if typeReference.TypeUID == 0 {
		typeReference.TypeUID = newUID(moduleUID, name)
	}

	if _, ok := typeUIDs[typeReference.TypeUID]; ok {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration with the uid '%d' in '%s'", typeReference.TypeUID, typeUIDs[typeReference.TypeUID])))
	}
	typeUIDs[typeReference.TypeUID] = moduleURI

	if _, ok := s.types[moduleURI][name]; ok {
		// TODO 2023.09.12: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.12: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration of '%s' in '%s'", name, moduleURI)))
	} else {
		s.types[moduleURI][name] = *typeReference
	}
}

func (s *globalSymbolTable) addAttribute(r exc.Reporter, moduleURI string, typeName string, name string, attributeReference *proto.AttributeReference, attributeUIDs map[uint64]string) {
	// Assumes we're already holding s.lock!

	moduleUID := s.modules[moduleURI]
	typeUID := s.types[moduleURI][typeName].TypeUID

	if attributeReference.ModuleUID == 0 {
		attributeReference.ModuleUID = moduleUID
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("the module UID for %s is already set, which shouldn't happen", name)))
	}

	if attributeReference.TypeUID == 0 {
		attributeReference.TypeUID = typeUID
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("the type UID for %s is already set, which shouldn't happen", name)))
	}

	if attributeReference.AttributeUID == 0 {
		attributeReference.AttributeUID = newUID(typeUID, name)
	}

	if _, ok := attributeUIDs[attributeReference.AttributeUID]; ok {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration with the uid '%d' in '%s'", attributeReference.AttributeUID, attributeUIDs[attributeReference.AttributeUID])))
	}
	attributeUIDs[attributeReference.AttributeUID] = typeName

	if _, ok := s.attributes[moduleURI][typeName][name]; ok {
		// TODO 2023.09.12: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.12: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration of '%s' in '%s'", name, typeName)))
	} else {
		s.attributes[moduleURI][typeName][name] = *attributeReference
	}
}

func (s *globalSymbolTable) addSDKMethodInput(r exc.Reporter, moduleURI string, typeName string, sdkMethodName string, name string, sdkInputReference *proto.SDKInputReference, sdkMethodInputUIDs map[uint64]string) {
	// Assumes we're already holding s.lock!

	moduleUID := s.modules[moduleURI]
	typeUID := s.types[moduleURI][typeName].TypeUID
	attributeUID := s.attributes[moduleURI][typeName][sdkMethodName].AttributeUID

	if sdkInputReference.ModuleUID == 0 {
		sdkInputReference.ModuleUID = moduleUID
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("the module UID for %s is already set, which shouldn't happen", name)))
	}

	if sdkInputReference.TypeUID == 0 {
		sdkInputReference.TypeUID = typeUID
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("the type UID for %s is already set, which shouldn't happen", name)))
	}

	if sdkInputReference.AttributeUID == 0 {
		sdkInputReference.AttributeUID = attributeUID
	} else {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("the attribute UID for %s is already set, which shouldn't happen", name)))
	}

	if sdkInputReference.AttributeUID == 0 {
		sdkInputReference.InputUID = newUID(attributeUID, name)
	}

	if _, ok := sdkMethodInputUIDs[sdkInputReference.AttributeUID]; ok {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration with the uid '%d' in '%s'", sdkInputReference.AttributeUID, sdkMethodInputUIDs[sdkInputReference.AttributeUID])))
	}
	sdkMethodInputUIDs[sdkInputReference.AttributeUID] = typeName

	if _, ok := s.attributes[moduleURI][typeName][name]; ok {
		// TODO 2023.09.12: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: moduleURI,
			// TODO 2023.09.12: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("there is already a declaration of '%s' in '%s'", name, typeName)))
	} else {
		s.inputs[moduleURI][typeName][sdkMethodName][name] = *sdkInputReference
	}
}
