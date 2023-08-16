package compiler

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
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
		targets = append(targets, self.targetURI(ctx, f))
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
	modules := make([]*idl.Module, 0, len(files))
	loaded := &sync.Map{}
	results := make(chan fileResult)
	expectedResults := len(files)

	for _, file := range files {
		go func(file idl.File) {
			image, err := self.compileFile(ctx, file, loaded, req.DumpTokens, req.DumpTree)
			results <- fileResult{image, err}
		}(file)
	}

	for x := 0; x < expectedResults; x = x + 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-results:
			if result.err != nil {
				return nil, result.err
			}
			if result.module != nil {
				modules = append(modules, result.module)
				// for _, mod := range result.module.Elements {
				// 	_ = mod
				// 	// TODO: iterate module elements and add any imports to the
				// 	// set of targets to compile.
				// }
			}
		}
	}

	final := &idl.Image{}
	included := make(map[string]bool)
	for _, mod := range modules {
		if _, ok := included[mod.URI]; ok {
			continue
		}
		included[mod.URI] = true
		final.Modules = append(final.Modules, mod)
	}
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

func (self *compiler) compileFile(ctx context.Context, file idl.File, loaded *sync.Map, dumpTokens bool, dumpTree bool) (*idl.Module, error) {
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
	return sc.CompileFile(ctx, self.Reporter, file, dumpTokens, dumpTree)
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
	module *idl.Module
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
