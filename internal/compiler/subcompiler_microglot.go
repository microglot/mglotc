// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package compiler

import (
	"context"
	"errors"
	"fmt"

	"gopkg.microglot.org/mglotc/internal/compiler/microglot"
	"gopkg.microglot.org/mglotc/internal/exc"
	"gopkg.microglot.org/mglotc/internal/idl"
	"gopkg.microglot.org/mglotc/internal/proto"
)

type SubCompilerMicroglot struct{}

func (self *SubCompilerMicroglot) CompileFile(ctx context.Context, r exc.Reporter, file idl.File, dumpTokens bool, dumpTree bool) (*proto.Module, error) {
	lexer := microglot.NewLexerMicroglot(r)
	parser := microglot.NewParserMicroglot(r)
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
	p, err := parser.PrepareParse(ctx, lf)
	if err != nil {
		return nil, err
	}
	ast := p.ParseModule()
	if ast == nil {
		return nil, errors.New("parse failure")
	}
	if dumpTree {
		fmt.Println(ast)
	}

	module, err := microglot.FromModule(ast)
	if err != nil {
		return nil, err
	}

	return module, nil
}
