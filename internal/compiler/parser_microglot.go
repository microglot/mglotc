package compiler

import (
	"context"
	"fmt"
	"strconv"

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

type parserMicroglotTokens struct {
	reporter exc.Reporter
	ctx      context.Context
	uri      string
	// this is the .Span.End of the last successfully parsed token; we keep track of it
	// so that we can give a meaningful location to "unexpected EOF" errors.
	loc    idl.Location
	tokens idl.Lookahead[*idl.Token]
}

func (p *parserMicroglotTokens) advance() {
	maybe_token := p.tokens.Lookahead(p.ctx, 0)
	if maybe_token.IsPresent() {
		p.loc = *maybe_token.Value().Span.End
	}
	_ = p.tokens.Next(p.ctx)
}

func (p *parserMicroglotTokens) peek() *idl.Token {
	maybe_token := p.tokens.Lookahead(p.ctx, 0)
	if !maybe_token.IsPresent() {
		return nil
	}
	return maybe_token.Value()
}

// reports an error if there is no current token, or the current token isn't of the expected type
// advances on success
func (p *parserMicroglotTokens) expect(expectedType idl.TokenType) *string {
	maybe_token := p.peek()
	if maybe_token == nil {
		p.reporter.Report(exc.New(exc.Location{
			URI:      p.uri,
			Location: p.loc,
		}, exc.CodeUnexpectedEOF, fmt.Sprintf("unexpected EOF (expecting %s)", expectedType)))
		return nil
	}
	if maybe_token.Type != expectedType {
		p.reporter.Report(exc.New(exc.Location{
			URI:      p.uri,
			Location: *maybe_token.Span.Start,
		}, exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting %s)", maybe_token.Value, expectedType)))
		return nil
	}
	p.advance()
	return &maybe_token.Value
}

// reports an error if current token isn't one of the expected types.
// Does NOT advance under any circumstance.
func (p *parserMicroglotTokens) expectOneOf(expectedTypes []idl.TokenType) *idl.Token {
	maybe_token := p.peek()
	if maybe_token == nil {
		return nil
	}
	for _, expectedType := range expectedTypes {
		if maybe_token.Type == expectedType {
			return maybe_token
		}
	}
	p.reporter.Report(exc.New(exc.Location{
		URI:      p.uri,
		Location: *maybe_token.Span.Start,
	}, exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting one of %v)", maybe_token.Value, expectedTypes)))
	return nil
}

// microglot = [CommentBlock] { Statement }
func (p *parserMicroglotTokens) parse() *ast {
	newAst := ast{}

	newAst.comments = p.parseCommentBlock()

	for {
		maybe_token := p.expectOneOf([]idl.TokenType{
			idl.TokenTypeKeywordSyntax,
			idl.TokenTypeKeywordModule,
		})
		if maybe_token == nil {
			break
		}

		switch maybe_token.Type {
		case idl.TokenTypeKeywordSyntax:
			maybe_statement := p.parseStatementSyntax()
			if maybe_statement == nil {
				return nil
			}
			newAst.statements = append(newAst.statements, maybe_statement)
		case idl.TokenTypeKeywordModule:
			maybe_statement := p.parseStatementModuleMeta()
			if maybe_statement == nil {
				return nil
			}
			newAst.statements = append(newAst.statements, maybe_statement)
		default:
			panic("can't happen")
		}
	}
	return &newAst
}

// StatementSyntax = "syntax" "=" text_lit
func (p *parserMicroglotTokens) parseStatementSyntax() *astStatementSyntax {
	if p.expect(idl.TokenTypeKeywordSyntax) == nil {
		return nil
	}
	if p.expect(idl.TokenTypeEqual) == nil {
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

// StatementModuleMeta = "module" "=" UID [AnnotationApplication] [CommentBlock]
func (p *parserMicroglotTokens) parseStatementModuleMeta() *astStatementModuleMeta {
	this := astStatementModuleMeta{}
	if p.expect(idl.TokenTypeKeywordModule) == nil {
		return nil
	}
	if p.expect(idl.TokenTypeEqual) == nil {
		return nil
	}
	uidNode := p.parseUID()
	if uidNode == nil {
		return nil
	}
	this.uid = *uidNode

	maybe_token := p.peek()
	if maybe_token != nil && maybe_token.Type == idl.TokenTypeDollar {
		annotationApplicationNode := p.parseAnnotationApplication()
		if annotationApplicationNode == nil {
			return nil
		}

		this.annotationApplication = *annotationApplicationNode
	}

	this.comments = p.parseCommentBlock()

	return &this
}

func (p *parserMicroglotTokens) parseCommentBlock() astCommentBlock {
	comments := []astComment{}
	for {
		maybe_token := p.peek()
		if maybe_token == nil || maybe_token.Type != idl.TokenTypeComment {
			break
		}
		comments = append(comments, *p.parseComment())
	}
	return astCommentBlock{
		comments,
	}
}

// AnnotationApplication = dollar paren_open [AnnotationInstance] { comma AnnotationInstance } [comma] paren_close
func (p *parserMicroglotTokens) parseAnnotationApplication() *astAnnotationApplication {
	if p.expect(idl.TokenTypeDollar) == nil {
		return nil
	}
	if p.expect(idl.TokenTypeParenOpen) == nil {
		return nil
	}

	// TODO 2023.08.16: incomplete

	if p.expect(idl.TokenTypeParenClose) == nil {
		return nil
	}

	return &astAnnotationApplication{
		// TODO 2023.08.16: incomplete
	}
}

// UID = at int_lit
func (p *parserMicroglotTokens) parseUID() *astIntLit {
	if p.expect(idl.TokenTypeAt) == nil {
		return nil
	}
	return p.parseIntLit()
}

// int_lit = decimal_lit | binary_lit | octal_lit | hex_lit
func (p *parserMicroglotTokens) parseIntLit() *astIntLit {
	maybe_token := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeIntegerDecimal,
		idl.TokenTypeIntegerHex,
		idl.TokenTypeIntegerOctal,
		idl.TokenTypeIntegerBinary,
	})
	if maybe_token == nil {
		return nil
	}

	p.advance()
	i, err := strconv.ParseUint(maybe_token.Value, 0, 64)
	if err != nil {
		p.reporter.Report(exc.New(exc.Location{
			URI:      p.uri,
			Location: *maybe_token.Span.Start,
		}, exc.CodeUnknownFatal, fmt.Sprintf("invalid integer literal %s", maybe_token.Value)))
		return nil
	}

	return &astIntLit{
		strValue: maybe_token.Value,
		value:    i,
	}
}

func (p *parserMicroglotTokens) parseTextLit() *astTextLit {
	t := p.expect(idl.TokenTypeText)
	if t == nil {
		return nil
	}
	return &astTextLit{
		value: *t,
	}
}

func (p *parserMicroglotTokens) parseComment() *astComment {
	t := p.expect(idl.TokenTypeComment)
	if t == nil {
		return nil
	}
	return &astComment{
		value: *t,
	}
}

func (self *ParserMicroglot) Parse(ctx context.Context, f idl.LexerFile) (*ast, error) {
	ft, err := f.Tokens(ctx)
	if err != nil {
		return nil, err
	}
	defer ft.Close(ctx)

	// as of right now, newlines and semicolons are ignored by the parser, but we're not 100% sure
	// this will be true forever. If it stops being true, this will need to be removed. If it
	// becomes certain, we should consider ignoring them in the lexer, instead.
	filtered_tokens := iter.NewIteratorFilter(ft, idl.Filter[*idl.Token](iter.FilterFunc[*idl.Token](func(ctx context.Context, t *idl.Token) bool {
		switch t.Type {
		case idl.TokenTypeNewline, idl.TokenTypeSemicolon:
			return false
		default:
			return true
		}
	})))

	tokens := iter.NewLookahead(filtered_tokens, 8)

	parser := parserMicroglotTokens{
		reporter: self.reporter,
		ctx:      ctx,
		tokens:   tokens,
		uri:      f.Path(ctx),
	}

	// TODO 2023.08.15: blatant hack to just parse a syntax statement
	return parser.parse(), nil
}
