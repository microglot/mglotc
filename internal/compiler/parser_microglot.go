package compiler

import (
	"context"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/iter"
)

type ParserMicroglot struct {
	reporter exc.Reporter
}

func NewParserMicroglot(reporter exc.Reporter) *ParserMicroglot {
	return &ParserMicroglot{reporter: reporter}
}

func (self *ParserMicroglot) Parse(ctx context.Context, f idl.LexerFile) (*idl.Module, error) {
	ft, err := f.Tokens(ctx)
	if err != nil {
		return nil, err
	}
	defer ft.Close(ctx)

	tokens := iter.NewLookahead(ft, 8)
	for tok := tokens.Next(ctx); tok.IsPresent(); tok = tokens.Next(ctx) {

	}
	return nil, nil
}
