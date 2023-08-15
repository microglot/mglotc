package compiler

import (
	"context"
	"fmt"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/iter"
)

type ParserMicroglot struct {
	reporter exc.Reporter
}

type astStatementSyntax struct {
	syntax astTextLit
}

type astSyntax struct {
}

type astEqual struct {
}

type astTextLit struct {
	text string
}

func NewParserMicroglot(reporter exc.Reporter) *ParserMicroglot {
	return &ParserMicroglot{reporter: reporter}
}

type parserMicroglotTokens struct {
	reporter exc.Reporter
	ctx      context.Context
	uri      string
	// this is the .Span.End of the last successfully parsed token; we keep track of it
	// so that we can give a meaningful location to "unexpected EOF" errors.
	loc    idl.Location
	tokens idl.Lookahead[*idl.Token]
}

func (p *parserMicroglotTokens) expect(expectedType idl.TokenType) *string {
	maybe_token := p.tokens.Lookahead(p.ctx, 0)
	if !maybe_token.IsPresent() {
		p.reporter.Report(exc.New(exc.Location{
			URI:      p.uri,
			Location: p.loc,
		}, exc.CodeUnexpectedEOF, fmt.Sprintf("unexpected EOF (expecting %s)", expectedType)))
		return nil
	}
	if maybe_token.Value().Type != expectedType {
		p.reporter.Report(exc.New(exc.Location{
			URI:      p.uri,
			Location: *maybe_token.Value().Span.Start,
		}, exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting %s)", maybe_token.Value().Value, expectedType)))
		return nil
	}
	p.loc = *maybe_token.Value().Span.End
	_ = p.tokens.Next(p.ctx)
	return &maybe_token.Value().Value
}

func (p *parserMicroglotTokens) parseStatementSyntax() *astStatementSyntax {
	syntaxNode := p.parseSyntax()
	if syntaxNode == nil {
		return nil
	}
	equalNode := p.parseEqual()
	if equalNode == nil {
		return nil
	}
	textNode := p.parseTextLit()
	if textNode == nil {
		return nil
	}
	return &astStatementSyntax{
		syntax: *textNode,
	}
}

func (p *parserMicroglotTokens) parseSyntax() *astSyntax {
	if p.expect(idl.TokenTypeKeywordSyntax) == nil {
		return nil
	}
	return &astSyntax{}
}

func (p *parserMicroglotTokens) parseEqual() *astEqual {
	if p.expect(idl.TokenTypeEqual) == nil {
		return nil
	}
	return &astEqual{}
}

func (p *parserMicroglotTokens) parseTextLit() *astTextLit {
	t := p.expect(idl.TokenTypeText)
	if t == nil {
		return nil
	}
	return &astTextLit{
		text: *t,
	}
}

func (self *ParserMicroglot) Parse(ctx context.Context, f idl.LexerFile) (*astStatementSyntax, error) {
	ft, err := f.Tokens(ctx)
	if err != nil {
		return nil, err
	}
	defer ft.Close(ctx)

	tokens := iter.NewLookahead(ft, 8)

	parser := parserMicroglotTokens{
		reporter: self.reporter,
		ctx:      ctx,
		tokens:   tokens,
		uri:      f.Path(ctx),
	}

	// TODO 2023.08.15: blatant hack to just parse a syntax statement
	return parser.parseStatementSyntax(), nil
}
