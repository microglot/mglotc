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
	moduleUIDs map[uint64]string
	types      map[string]map[string]proto.TypeReference
}

// globalSymbolTable.collect() populates a symbol table with the symbols in a given descriptor
// as a side-effect, it generates any missing TypeUID values in the descriptor!
// reports: name collisions, UID collisions
func (s *globalSymbolTable) collect(parsed proto.Module, r exc.Reporter) (*proto.Module, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.types == nil {
		s.types = make(map[string]map[string]proto.TypeReference)
	}
	if s.moduleUIDs == nil {
		s.moduleUIDs = make(map[uint64]string)
	}
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
	if _, ok := s.moduleUIDs[parsed.UID]; ok {
		// TODO 2023.09.14: replace CodeUnknownFatal with something more meaningful
		r.Report(exc.New(exc.Location{
			URI: parsed.URI,
			// TODO 2023.09.14: getting Location here would be nice!
		}, exc.CodeUnknownFatal, fmt.Sprintf("module UID '%d' is already in-use by '%s'", parsed.UID, s.moduleUIDs[parsed.UID])))
		return nil, errors.New("collect error")
	}
	s.moduleUIDs[parsed.UID] = parsed.URI

	typeUIDs := make(map[uint64]string)

	// TODO 2023.09.11: add AttributeReference and SDKInputReference
	for _, struct_ := range parsed.Structs {
		s.addType(r, parsed.UID, parsed.URI, struct_.Name.Name, struct_.Reference, typeUIDs)
	}
	for _, enum := range parsed.Enums {
		s.addType(r, parsed.UID, parsed.URI, enum.Name, enum.Reference, typeUIDs)
	}
	for _, api := range parsed.APIs {
		s.addType(r, parsed.UID, parsed.URI, api.Name.Name, api.Reference, typeUIDs)
	}
	for _, sdk := range parsed.SDKs {
		s.addType(r, parsed.UID, parsed.URI, sdk.Name.Name, sdk.Reference, typeUIDs)
	}
	for _, annotation := range parsed.Annotations {
		s.addType(r, parsed.UID, parsed.URI, annotation.Name, annotation.Reference, typeUIDs)
	}
	for _, constant := range parsed.Constants {
		s.addType(r, parsed.UID, parsed.URI, constant.Name, constant.Reference, typeUIDs)
	}

	if len(r.Reported()) > 0 {
		return nil, errors.New("collect error")
	}
	return &parsed, nil
}

func newTypeUID(moduleUID uint64, name string) uint64 {
	hasher := sha256.New()
	err := binary.Write(hasher, binary.LittleEndian, moduleUID)
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

func (s *globalSymbolTable) addType(r exc.Reporter, moduleUID uint64, moduleURI string, name string, typeReference *proto.TypeReference, typeUIDs map[uint64]string) {
	// Assumes we're already holding s.lock!

	if typeReference.ModuleUID == 0 {
		typeReference.ModuleUID = moduleUID
	}
	if typeReference.TypeUID == 0 {
		typeReference.TypeUID = newTypeUID(moduleUID, name)
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
