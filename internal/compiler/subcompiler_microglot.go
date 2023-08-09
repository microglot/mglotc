package compiler

import (
	"context"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
)

type SubCompilerMicroglot struct{}

func (self *SubCompilerMicroglot) CompileFile(ctx context.Context, r exc.Reporter, file idl.File) (*idl.Module, error) {
	lexer := NewLexerMicroglot(r)
	parser := NewParserMicroglot(r)
	lf, err := lexer.Lex(ctx, file)
	if err != nil {
		return nil, err
	}
	mod, err := parser.Parse(ctx, lf)
	if err != nil {
		return nil, err
	}
	return mod, nil
}
