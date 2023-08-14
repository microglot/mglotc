package compiler

import (
	"context"
	"errors"
	"fmt"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
)

type SubCompilerMicroglot struct{}

func (self *SubCompilerMicroglot) CompileFile(ctx context.Context, r exc.Reporter, file idl.File, dumpTokens bool) (*idl.Module, error) {
	lexer := NewLexerMicroglot(r)
	parser := NewParserMicroglot(r)
	lf, err := lexer.Lex(ctx, file)
	if err != nil {
		return nil, err
	}
	if dumpTokens {
		// TODO 2023.08.14: make token dumping a side-effect of consuming the stream during parsing
		stream, err := lf.Tokens(ctx)
		if err != nil {
			return nil, err
		}
		for tok := stream.Next(ctx); tok.IsPresent(); tok = stream.Next(ctx) {
			token := tok.Value()
			fmt.Printf("%-24s", token.Type)
			if token.Type != idl.TokenTypeNewline {
				fmt.Printf("'%s'", token.Value)
			}
			fmt.Println()
		}
		return nil, errors.New("aborting after dumping tokens, since the parser doesn't consume the token stream yet")
	}
	mod, err := parser.Parse(ctx, lf)
	if err != nil {
		return nil, err
	}
	return mod, nil
}
