package compiler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/fs"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
	"gopkg.microglot.org/compiler.go/internal/target"
)

type Option func(c *compiler) error

func OptionWithFS(fs idl.FileSystem) Option {
	return func(c *compiler) error {
		c.FS = fs
		return nil
	}
}

func OptionWithLookupEnv(lookupEnv func(string) (string, bool)) Option {
	return func(c *compiler) error {
		c.LookupENV = lookupEnv
		return nil
	}
}

func OptionWithExcReporter(reporter exc.Reporter) Option {
	return func(c *compiler) error {
		c.Reporter = reporter
		return nil
	}
}

func New(opts ...Option) (idl.Compiler, error) {
	c := &compiler{}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.LookupENV == nil {
		c.LookupENV = os.LookupEnv
	}
	if c.FS == nil {
		dfs, err := NewDefaultFS(c.LookupENV)
		if err != nil {
			return nil, err
		}
		c.FS = dfs
	}
	if c.MaxConcurrency == 0 {
		max := runtime.GOMAXPROCS(-1)
		cpus := runtime.NumCPU()
		if max > cpus {
			max = cpus
		}
		c.MaxConcurrency = max
	}
	if c.Semaphore == nil {
		c.Semaphore = newSemaphore(c.MaxConcurrency)
	}
	if c.Reporter == nil {
		c.Reporter = exc.NewReporter(nil)
	}
	if c.SubCompilers == nil {
		c.SubCompilers = DefaultSubCompilers()
	}
	return c, nil
}

type compiler struct {
	LookupENV      func(string) (string, bool)
	FS             idl.FileSystem
	MaxConcurrency int
	Semaphore      *semaphore
	Reporter       exc.Reporter
	SubCompilers   map[idl.FileKind]SubCompiler
}

func (self *compiler) Compile(ctx context.Context, req *idl.CompileRequest) (*idl.CompileResponse, error) {
	targets := make([]string, 0, len(req.Files))
	for _, f := range req.Files {
		targets = append(targets, target.Normalize(f))
	}
	files := make([]idl.File, 0, len(targets))
	for _, target := range targets {
		in, err := self.FS.Open(ctx, target)
		if err != nil {
			panic(err.Error())
		}
		for _, inf := range in {
			if inf.Kind(ctx) == idl.FileKindNone {
				continue
			}
			files = append(files, inf)
		}
	}
	modules := make([]*proto.Module, 0, len(files))
	loaded := &sync.Map{}
	results := make(chan fileResult)
	expectedResults := len(files) + 1
	symbols := globalSymbolTable{}

	go func() {
		image, err := self.compileFile(ctx, fs.NewFileString("/protobuf.mgdl", idl.PROTOBUF_IDL, idl.FileKindMicroglot), loaded, &symbols, req.DumpTokens, req.DumpTree)
		results <- fileResult{image, err}
	}()

	for _, file := range files {
		go func(file idl.File) {
			image, err := self.compileFile(ctx, file, loaded, &symbols, req.DumpTokens, req.DumpTree)
			results <- fileResult{image, err}
		}(file)
	}

	// TODO 2024.01.08: the racing here means that reported errors can vary from run to run,
	// which can be surprising/off-putting. Waiting for all of the raced goroutines before
	// returning errors might be better?
	for x := 0; x < expectedResults; x = x + 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-results:
			if result.err != nil {
				caught := self.Reporter.Reported()
				if len(caught) > 0 {
					return nil, MultiException(caught)
				} else {
					return nil, result.err
				}
			}
			if result.module != nil {
				modules = append(modules, result.module)
				for _, import_ := range result.module.Imports {
					uri := target.Normalize(import_.ImportedURI)
					in, err := self.FS.Open(ctx, uri)
					if err != nil {
						return nil, err
					}
					for _, inf := range in {
						if inf.Kind(ctx) == idl.FileKindNone {
							continue
						}

						go func(file idl.File) {
							image, err := self.compileFile(ctx, file, loaded, &symbols, req.DumpTokens, req.DumpTree)
							results <- fileResult{image, err}
						}(inf)
						expectedResults += 1
					}
				}
			}
		}
	}

	linked_modules := make([]*proto.Module, 0, len(modules))
	linkResults := make(chan fileResult)
	expectedLinkResults := len(modules)

	for _, mod := range modules {
		go func(mod proto.Module) {
			linked_module, err := link(mod, &symbols, self.Reporter)
			linkResults <- fileResult{linked_module, err}
		}(*mod)
	}

	// TODO 2024.01.08: the racing here means that reported errors can vary from run to run,
	// which can be surprising/off-putting. Waiting for all of the raced goroutines before
	// returning errors might be better?
	for x := 0; x < expectedLinkResults; x = x + 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-linkResults:
			if result.err != nil {
				caught := self.Reporter.Reported()
				if len(caught) > 0 {
					return nil, MultiException(caught)
				} else {
					return nil, result.err
				}
			}
			linked_modules = append(linked_modules, result.module)
		}
	}

	final := &idl.Image{}
	included := make(map[string]bool)
	for _, mod := range linked_modules {
		if _, ok := included[mod.URI]; ok {
			continue
		}
		included[mod.URI] = true
		final.Modules = append(final.Modules, mod)
	}

	optimize(final)
	check(final, self.Reporter)

	caught := self.Reporter.Reported()
	if len(caught) > 0 {
		return &idl.CompileResponse{
			Image: final,
		}, MultiException(caught)
	}
	return &idl.CompileResponse{
		Image: final,
	}, nil
}

func (self *compiler) compileFile(ctx context.Context, file idl.File, loaded *sync.Map, symbols *globalSymbolTable, dumpTokens bool, dumpTree bool) (*proto.Module, error) {
	self.Semaphore.Lock()
	defer self.Semaphore.Unlock()
	if _, ok := loaded.Load(file.Path(ctx)); ok {
		return nil, nil
	}
	loaded.Store(file.Path(ctx), true)
	sc := self.SubCompilers[file.Kind(ctx)]
	if sc == nil {
		e := exc.New(exc.Location{URI: file.Path(ctx)}, exc.CodeUnsupportedFileFormat, "Unsupported file format")
		return nil, self.Reporter.Report(e)
	}
	module, err := sc.CompileFile(ctx, self.Reporter, file, dumpTokens, dumpTree)
	if err != nil {
		return nil, err
	}
	completed := completeUIDs(*module)

	err = symbols.collect(*completed, self.Reporter)
	if err != nil {
		return nil, err
	}

	return completed, nil
}

func (self *compiler) targetURI(ctx context.Context, target string) string {
	// The compiler allows targets to be any valid URI or file path. When
	// the target is a file path or a file URI then we convert the paths to
	// an absolute form in order to work with the local implementation of
	// the FileSystem interface. All non-file URIs are left as-is with the
	// expectation that they will be handled by some other implementation.
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "" && u.Scheme != "file") {
		return target
	}
	if u.Scheme == "file" {
		target = u.Path
	}
	if !filepath.IsAbs(target) {
		return filepath.Join("/", target)
	}
	return target
}

type fileResult struct {
	module *proto.Module
	err    error
}

type MultiException []exc.Exception

func (self MultiException) Error() string {
	var b strings.Builder
	for _, err := range self[:len(self)-1] {
		b.WriteString(err.Error())
		b.WriteString("; ")
	}
	b.WriteString(self[len(self)-1].Error())
	return b.String()
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
func newAttributeUID(parent uint64, name string) uint64 {
	// Protubuf field numbers are limited to 2^29 - 1 because the first three
	// bytes are used to encode the field type. For compatibility we must remove
	// the leading three bits.
	//
	// TODO 2024.02.01: Attribute UID values must also not be in the range of
	//                  19000 - 19999. Add a check that validates all field UID
	//                  values are in the supported range.
	uid := newUID(parent, name)
	return uid & 0x1FFFFFFF
}

func completeTypeReference(moduleUID uint64, name string, typeReference *proto.TypeReference) {
	if typeReference.ModuleUID == idl.Incomplete {
		typeReference.ModuleUID = moduleUID
	} else {
		// TODO 2023.09.23: this may become a non-panic in future, but right now means a logic error
		panic(fmt.Errorf("the module UID for %s is already set, which shouldn't happen", name))
	}
	if typeReference.TypeUID == idl.Incomplete {
		typeReference.TypeUID = newUID(moduleUID, name)
	}
}

func completeAttributeReference(moduleUID uint64, typeUID uint64, name string, attributeReference *proto.AttributeReference) {
	if attributeReference.ModuleUID == idl.Incomplete {
		attributeReference.ModuleUID = moduleUID
	} else {
		// TODO 2023.09.23: this may become a non-panic in future, but right now means a logic error
		panic(fmt.Errorf("the module UID for %s is already set, which shouldn't happen", name))
	}

	if attributeReference.TypeUID == idl.Incomplete {
		attributeReference.TypeUID = typeUID
	} else {
		// TODO 2023.09.23: this may become a non-panic in future, but right now means a logic error
		panic(fmt.Errorf("the type UID for %s is already set, which shouldn't happen", name))
	}

	if attributeReference.AttributeUID == idl.Incomplete {
		attributeReference.AttributeUID = newAttributeUID(typeUID, name)
	}

}

func completeSDKInputReference(moduleUID uint64, typeUID uint64, attributeUID uint64, name string, sdkInputReference *proto.SDKInputReference) {
	if sdkInputReference.ModuleUID == idl.Incomplete {
		sdkInputReference.ModuleUID = moduleUID
	} else {
		// TODO 2023.09.23: this may become a non-panic in future, but right now means a logic error
		panic(fmt.Errorf("the module UID for %s is already set, which shouldn't happen", name))
	}

	if sdkInputReference.TypeUID == idl.Incomplete {
		sdkInputReference.TypeUID = typeUID
	} else {
		// TODO 2023.09.23: this may become a non-panic in future, but right now means a logic error
		panic(fmt.Errorf("the type UID for %s is already set, which shouldn't happen", name))
	}

	if sdkInputReference.AttributeUID == idl.Incomplete {
		sdkInputReference.AttributeUID = attributeUID
	} else {
		// TODO 2023.09.23: this may become a non-panic in future, but right now means a logic error
		panic(fmt.Errorf("the attribute UID for %s is already set, which shouldn't happen", name))
	}

	if sdkInputReference.InputUID == idl.Incomplete {
		sdkInputReference.InputUID = newUID(attributeUID, name)
	}
}

// this completes the pre-linked descriptor by generating any missing TypeUID values in the descriptor!
func completeUIDs(parsed proto.Module) *proto.Module {
	for _, struct_ := range parsed.Structs {
		completeTypeReference(parsed.UID, struct_.Name.Name, struct_.Reference)
		for _, field := range struct_.Fields {
			completeAttributeReference(parsed.UID, struct_.Reference.TypeUID, field.Name, field.Reference)
		}
		for _, union := range struct_.Unions {
			completeAttributeReference(parsed.UID, struct_.Reference.TypeUID, union.Name, union.Reference)
		}
	}
	for _, enum := range parsed.Enums {
		completeTypeReference(parsed.UID, enum.Name, enum.Reference)
		for _, enumerant := range enum.Enumerants {
			completeAttributeReference(parsed.UID, enum.Reference.TypeUID, enumerant.Name, enumerant.Reference)
		}
	}
	for _, api := range parsed.APIs {
		completeTypeReference(parsed.UID, api.Name.Name, api.Reference)
		for _, apiMethod := range api.Methods {
			completeAttributeReference(parsed.UID, api.Reference.TypeUID, apiMethod.Name, apiMethod.Reference)
		}
	}
	for _, sdk := range parsed.SDKs {
		completeTypeReference(parsed.UID, sdk.Name.Name, sdk.Reference)
		for _, sdkMethod := range sdk.Methods {
			completeAttributeReference(parsed.UID, sdk.Reference.TypeUID, sdkMethod.Name, sdkMethod.Reference)
			for _, sdkMethodInput := range sdkMethod.Input {
				completeSDKInputReference(parsed.UID, sdk.Reference.TypeUID, sdkMethod.Reference.AttributeUID, sdkMethodInput.Name, sdkMethodInput.Reference)
			}
		}
	}
	for _, annotation := range parsed.Annotations {
		completeTypeReference(parsed.UID, annotation.Name, annotation.Reference)
	}
	for _, constant := range parsed.Constants {
		completeTypeReference(parsed.UID, constant.Name, constant.Reference)
	}

	return &parsed
}
