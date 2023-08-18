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

func (self *ParserMicroglot) PrepareParse(ctx context.Context, f idl.LexerFile) (*parserMicroglotTokens, error) {
	ft, err := f.Tokens(ctx)
	if err != nil {
		return nil, err
	}

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

	return &parserMicroglotTokens{
		reporter: self.reporter,
		ctx:      ctx,
		tokens:   tokens,
		uri:      f.Path(ctx),
	}, nil
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

func (p *parserMicroglotTokens) report(code string, message string) {
	p.reporter.Report(exc.New(exc.Location{
		URI:      p.uri,
		Location: p.loc,
	}, code, message))
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
func (p *parserMicroglotTokens) expectOne(expectedType idl.TokenType) *idl.Token {
	return p.expectOneOf([]idl.TokenType{expectedType})
}

// just like expectOne(), except it then advances over (and returns) any subsequent tokens of the same type.
func (p *parserMicroglotTokens) expectSeveral(expectedType idl.TokenType) []idl.Token {
	first := p.expectOne(expectedType)
	if first == nil {
		return nil
	}

	values := []idl.Token{*first}
	for {
		maybe_token := p.peek()
		if maybe_token == nil || maybe_token.Type != expectedType {
			break
		}
		p.advance()
		values = append(values, *maybe_token)
	}
	return values
}

// reports an error if current token isn't one of the given expected types.
// advances on success
func (p *parserMicroglotTokens) expectOneOf(expectedTypes []idl.TokenType) *idl.Token {
	maybe_token := p.peek()
	if maybe_token == nil {
		p.report(exc.CodeUnexpectedEOF, fmt.Sprintf("unexpected EOF (expecting %v)", expectedTypes))
		return nil
	}
	for _, expectedType := range expectedTypes {
		if maybe_token.Type == expectedType {
			p.advance()
			return maybe_token
		}
	}
	p.report(exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting %v)", maybe_token.Value, expectedTypes))
	return nil
}

// microglot = [CommentBlock] StatementSyntax { Statement }
func (p *parserMicroglotTokens) parse() *ast {
	newAst := ast{}

	maybe_token := p.peek()
	if maybe_token != nil && maybe_token.Type == idl.TokenTypeComment {
		newAst.comments = *p.parseCommentBlock()
	}

	syntax := p.parseStatementSyntax()
	if syntax == nil {
		return nil
	}
	newAst.syntax = *syntax

	for {
		maybe_token := p.peek()
		if maybe_token == nil {
			break
		}

		switch maybe_token.Type {
		case idl.TokenTypeKeywordModule:
			maybe_statement := p.parseStatementModuleMeta()
			if maybe_statement == nil {
				return nil
			}
			newAst.statements = append(newAst.statements, maybe_statement)
		default:
			p.report(exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting a statement)", maybe_token.Value))
			return nil
		}
	}

	return &newAst
}

// StatementSyntax = "syntax" "=" text_lit
func (p *parserMicroglotTokens) parseStatementSyntax() *astStatementSyntax {
	if p.expectOne(idl.TokenTypeKeywordSyntax) == nil {
		return nil
	}
	if p.expectOne(idl.TokenTypeEqual) == nil {
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
	if p.expectOne(idl.TokenTypeKeywordModule) == nil {
		return nil
	}
	if p.expectOne(idl.TokenTypeEqual) == nil {
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

	maybe_token = p.peek()
	if maybe_token != nil && maybe_token.Type == idl.TokenTypeComment {
		this.comments = *p.parseCommentBlock()
	}

	return &this
}

func (p *parserMicroglotTokens) parseCommentBlock() *astCommentBlock {
	comments := p.expectSeveral(idl.TokenTypeComment)
	if comments == nil {
		return nil
	}
	return &astCommentBlock{
		comments,
	}
}

// AnnotationApplication = dollar paren_open [AnnotationInstance] { comma AnnotationInstance } [comma] paren_close
func (p *parserMicroglotTokens) parseAnnotationApplication() *astAnnotationApplication {
	if p.expectOne(idl.TokenTypeDollar) == nil {
		return nil
	}
	if p.expectOne(idl.TokenTypeParenOpen) == nil {
		return nil
	}

	for {
		maybe_token := p.peek()
		if maybe_token == nil || maybe_token.Type != idl.TokenTypeIdentifier {
			break
		}
		annotationInstance := p.parseAnnotationInstance()
		if annotationInstance == nil {
			return nil
		}

		maybe_token = p.peek()
		if maybe_token == nil || maybe_token.Type != idl.TokenTypeComma {
			break
		}
	}

	if p.expectOne(idl.TokenTypeParenClose) == nil {
		return nil
	}

	return &astAnnotationApplication{
		// TODO 2023.08.16: incomplete
	}
}

// AnnotationInstance = identifier [dot identifier] paren_open Value paren_close
func (p *parserMicroglotTokens) parseAnnotationInstance() *astAnnotationInstance {
	// TODO 2023.08.16: is "namespace" the right nomenclature?
	var namespace_identifier *idl.Token = nil
	identifier := p.expectOne(idl.TokenTypeIdentifier)
	if identifier == nil {
		return nil
	}

	maybe_token := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeDot,
		idl.TokenTypeParenOpen,
	})
	if maybe_token == nil {
		return nil
	}

	if maybe_token.Type == idl.TokenTypeDot {
		namespace_identifier = identifier
		identifier = p.expectOne(idl.TokenTypeIdentifier)
		if identifier == nil {
			return nil
		}

		maybe_token = p.expectOne(idl.TokenTypeParenOpen)
		if maybe_token == nil {
			return nil
		}
	}

	value := p.parseValue()
	if value == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeParenClose) == nil {
		return nil
	}

	return &astAnnotationInstance{
		namespace_identifier: namespace_identifier,
		identifier:           *identifier,
		value:                *value,
	}
}

// UID = at int_lit
func (p *parserMicroglotTokens) parseUID() *astIntLit {
	if p.expectOne(idl.TokenTypeAt) == nil {
		return nil
	}
	return p.parseIntLit()
}

// Value = ValueUnary | ValueBinary | ValueLiteral | ValueIdentifier
func (p *parserMicroglotTokens) parseValue() *astValue {
	maybe_token := p.peek()
	if maybe_token == nil {
		p.report(exc.CodeUnexpectedEOF, fmt.Sprint(exc.CodeUnexpectedEOF, "unexpected EOF (expecting a value)"))
		return nil
	}

	switch maybe_token.Type {
	case idl.TokenTypePlus, idl.TokenTypeMinus, idl.TokenTypeExclamation:
		_ = p.parseValueUnary()
	case idl.TokenTypeParenOpen:
		_ = p.parseValueBinary()
	case idl.TokenTypeKeywordTrue, idl.TokenTypeKeywordFalse, idl.TokenTypeIntegerDecimal, idl.TokenTypeIntegerHex, idl.TokenTypeIntegerOctal, idl.TokenTypeIntegerBinary, idl.TokenTypeFloatDecimal, idl.TokenTypeFloatHex, idl.TokenTypeData, idl.TokenTypeSquareOpen, idl.TokenTypeCurlyOpen:
		_ = p.parseValueLiteral()
	case idl.TokenTypeIdentifier:
		_ = p.parseValueIdentifier()
	default:
		p.report(exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting a value)", maybe_token.Value))
	}

	// TODO 2023.08.17: incomplete
	p.advance()
	return &astValue{}
}

// ValueUnary = (plus | minus | bang ) Value
func (p *parserMicroglotTokens) parseValueUnary() *astValueUnary {
	maybe_token := p.expectOneOf([]idl.TokenType{
		idl.TokenTypePlus,
		idl.TokenTypeMinus,
		idl.TokenTypeExclamation,
	})
	if maybe_token == nil {
		return nil
	}
	//TODO
	return nil
}

// ValueBinary = paren_open Value ( equal_compare | equal_not | equal_lesser |
//
//	equal_greater | bool_and | bool_or | bin_and | bin_or | bin_xor | shift_left |
//	shift_right | plus | slash | star | mod ) Value paren_close
func (p *parserMicroglotTokens) parseValueBinary() *astValueBinary {
	maybe_token := p.expectOne(idl.TokenTypeParenOpen)
	if maybe_token == nil {
		return nil
	}
	//TODO
	return nil
}

// ValueLiteral =  ValueLiteralBool | ValueLiteralInt | ValueLiteralFloat | ValueLiteralText | ValueLiteralData | ValueLiteralList | ValueLiteralStruct
func (p *parserMicroglotTokens) parseValueLiteral() *astValueLiteral {
	maybe_token := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeKeywordTrue,
		idl.TokenTypeKeywordFalse,
		idl.TokenTypeIntegerDecimal,
		idl.TokenTypeIntegerHex,
		idl.TokenTypeIntegerOctal,
		idl.TokenTypeIntegerBinary,
		idl.TokenTypeFloatDecimal,
		idl.TokenTypeFloatHex,
		idl.TokenTypeData,
		idl.TokenTypeSquareOpen,
		idl.TokenTypeCurlyOpen,
	})
	if maybe_token == nil {
		return nil
	}
	// TODO
	return nil
}

// ValueIdentifier = ValueIdentifier = QualifiedIdentifier .
func (p *parserMicroglotTokens) parseValueIdentifier() *astValueIdentifier {
	maybe_token := p.expectOne(idl.TokenTypeIdentifier)
	if maybe_token == nil {
		return nil
	}
	// TODO
	return nil
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

	i, err := strconv.ParseUint(maybe_token.Value, 0, 64)
	if err != nil {
		p.report(exc.CodeUnknownFatal, fmt.Sprintf("invalid integer literal %s", maybe_token.Value))
		return nil
	}

	return &astIntLit{
		token: *maybe_token,
		value: i,
	}
}

func (p *parserMicroglotTokens) parseTextLit() *astTextLit {
	t := p.expectOne(idl.TokenTypeText)
	if t == nil {
		return nil
	}
	return &astTextLit{
		value: *t,
	}
}
