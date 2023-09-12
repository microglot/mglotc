package compiler

import (
	"context"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/iter"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

// SubCompilerIDL is an adaptive sub-compiler for all IDL formats that switches
// into the appropriate sub-compiler based on the IDL format.
type SubCompilerIDL struct {
	Microglot SubCompiler
	Protobuf  SubCompiler
}

func (self *SubCompilerIDL) CompileFile(ctx context.Context, r exc.Reporter, file idl.File, dumpTokens bool, dumpTree bool) (*proto.Module, error) {
	lex := NewLexerIDL(r)
	lexf, err := lex.Lex(ctx, file)
	if err != nil {
		return nil, err
	}
	tokens, err := lexf.Tokens(ctx)
	if err != nil {
		return nil, err
	}
	defer tokens.Close(ctx)
	tokenLookahead := iter.NewLookahead(tokens, 2)
	syntax := ""

READLOOP:
	for tok := tokenLookahead.Next(ctx); tok.IsPresent(); tok = tokenLookahead.Next(ctx) {
		t := tok.Value()
		switch t.Type {
		case idl.TokenTypeKeywordSyntax:
			nt := tokenLookahead.Lookahead(ctx, 1)
			if !nt.IsPresent() || nt.Value().Type != idl.TokenTypeEqual {
				return nil, r.Report(exc.New(exc.Location{URI: file.Path(ctx), Location: *t.Span.Start}, exc.CodeUnsupportedFileFormat, "missing or invalid syntax statement"))
			}
			nt = tokenLookahead.Lookahead(ctx, 2)
			if !nt.IsPresent() || nt.Value().Type != idl.TokenTypeText {
				return nil, r.Report(exc.New(exc.Location{URI: file.Path(ctx), Location: *t.Span.Start}, exc.CodeUnsupportedFileFormat, "missing or invalid syntax statement"))
			}
			syntax = nt.Value().Value
			break READLOOP
		}
	}

	_ = tokenLookahead.Close(ctx)
	switch syntax {
	case "proto2", "proto3":
		return self.Protobuf.CompileFile(ctx, r, file, dumpTokens, dumpTree)
	case "microglot0", "microglot1":
		return self.Microglot.CompileFile(ctx, r, file, dumpTokens, dumpTree)
	default:
		return nil, r.Report(exc.New(exc.Location{URI: file.Path(ctx)}, exc.CodeUnsupportedFileFormat, "missing or invalid syntax statement"))
	}
}
