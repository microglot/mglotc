package compiler

import (
	"context"
	"fmt"
	"reflect"
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
	filteredTokens := iter.NewIteratorFilter(ft, idl.Filter[*idl.Token](iter.FilterFunc[*idl.Token](func(ctx context.Context, t *idl.Token) bool {
		switch t.Type {
		case idl.TokenTypeNewline, idl.TokenTypeSemicolon:
			return false
		default:
			return true
		}
	})))

	tokens := iter.NewLookahead(filteredTokens, 8)

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
	maybeToken := p.tokens.Lookahead(p.ctx, 0)
	if maybeToken.IsPresent() {
		p.loc = *maybeToken.Value().Span.End
	}
	_ = p.tokens.Next(p.ctx)
}

func (p *parserMicroglotTokens) peek() *idl.Token {
	maybeToken := p.tokens.Lookahead(p.ctx, 0)
	if !maybeToken.IsPresent() {
		return nil
	}
	return maybeToken.Value()
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
		maybeToken := p.peek()
		if maybeToken == nil || maybeToken.Type != expectedType {
			break
		}
		p.advance()
		values = append(values, *maybeToken)
	}
	return values
}

// reports an error if current token isn't one of the given expected types.
// advances on success
func (p *parserMicroglotTokens) expectOneOf(expectedTypes []idl.TokenType) *idl.Token {
	maybeToken := p.peek()
	if maybeToken == nil {
		p.report(exc.CodeUnexpectedEOF, fmt.Sprintf("unexpected EOF (expecting %v)", expectedTypes))
		return nil
	}
	for _, expectedType := range expectedTypes {
		if maybeToken.Type == expectedType {
			p.advance()
			return maybeToken
		}
	}
	// TODO 2023.08.21: replace CodeUnknownFatal with something meaningful
	p.report(exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting %v)", maybeToken.Value, expectedTypes))
	return nil
}

// generic application of parsing lists of zero or more comma-separated nodes, allowing an optional trailing comma
func applyOverCommaSeparatedList[N node](p *parserMicroglotTokens, tOpen idl.TokenType, parser func(*parserMicroglotTokens) N, tClose idl.TokenType) []N {
	if p.expectOne(tOpen) == nil {
		return nil
	}
	values := []N{}

	maybeToken := p.peek()
	if maybeToken == nil {
		p.report(exc.CodeUnexpectedEOF, fmt.Sprintf("unexpected EOF (expecting a list of %T)", values))
		return nil
	}
	if maybeToken.Type != tClose {
		maybeValue := parser(p)
		if reflect.ValueOf(maybeValue).IsNil() {
			return nil
		}
		values = append(values, maybeValue)

		for {
			maybeToken = p.peek()
			if maybeToken == nil {
				p.report(exc.CodeUnexpectedEOF, fmt.Sprintf("unexpected EOF (expecting a list of %T)", values))
				return nil
			}
			if maybeToken.Type == tClose {
				break
			}

			if p.expectOne(idl.TokenTypeComma) == nil {
				return nil
			}

			maybeToken = p.peek()
			if maybeToken == nil {
				p.report(exc.CodeUnexpectedEOF, fmt.Sprintf("unexpected EOF (expecting a list of %T)", values))
				return nil
			}
			if maybeToken.Type == tClose {
				break
			}

			maybeValue = parser(p)
			if reflect.ValueOf(maybeValue).IsNil() {
				return nil
			}
			values = append(values, maybeValue)
		}
	}

	if p.expectOne(tClose) == nil {
		return nil
	}

	return values
}

// microglot = [CommentBlock] StatementSyntax { Statement }
func (p *parserMicroglotTokens) parse() *ast {
	this := ast{}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
		this.comments = *p.parseCommentBlock()
	}

	syntax := p.parseStatementSyntax()
	if syntax == nil {
		return nil
	}
	this.syntax = *syntax

	for {
		maybeToken := p.peek()
		if maybeToken == nil {
			break
		}

		switch maybeToken.Type {
		case idl.TokenTypeKeywordModule:
			maybeStatement := p.parseStatementModuleMeta()
			if maybeStatement == nil {
				return nil
			}
			this.statements = append(this.statements, maybeStatement)
		case idl.TokenTypeKeywordImport:
			maybeStatement := p.parseStatementImport()
			if maybeStatement == nil {
				return nil
			}
			this.statements = append(this.statements, maybeStatement)
		case idl.TokenTypeKeywordAnnotation:
			maybeStatement := p.parseStatementAnnotation()
			if maybeStatement == nil {
				return nil
			}
			this.statements = append(this.statements, maybeStatement)
		default:
			// TODO 2023.08.21: replace CodeUnknownFatal with something meaningful
			p.report(exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting a statement)", maybeToken.Value))
			return nil
		}
	}

	return &this
}

// StatementSyntax = "syntax" "=" text_lit
func (p *parserMicroglotTokens) parseStatementSyntax() *astStatementSyntax {
	if p.expectOne(idl.TokenTypeKeywordSyntax) == nil {
		return nil
	}
	if p.expectOne(idl.TokenTypeEqual) == nil {
		return nil
	}
	textNode := p.parseValueLiteralText()
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

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeDollar {
		annotationApplicationNode := p.parseAnnotationApplication()
		if annotationApplicationNode == nil {
			return nil
		}

		this.annotationApplication = *annotationApplicationNode
	}

	maybeToken = p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
		this.comments = *p.parseCommentBlock()
	}

	return &this
}

// StatementImport = import ImportURI as ModuleName [CommentBlock] .
func (p *parserMicroglotTokens) parseStatementImport() *astStatementImport {
	if p.expectOne(idl.TokenTypeKeywordImport) == nil {
		return nil
	}
	maybeUri := p.parseValueLiteralText()
	if maybeUri == nil {
		return nil
	}
	if p.expectOne(idl.TokenTypeKeywordAs) == nil {
		return nil
	}
	maybeName := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeIdentifier,
		idl.TokenTypeDot,
	})
	if maybeName == nil {
		return nil
	}

	this := astStatementImport{
		uri:  *maybeUri,
		name: *maybeName,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
		this.comments = *p.parseCommentBlock()
	}
	return &this
}

// StatementAnnotation = annotation identifier paren_open AnnotationScope {comma AnnotationScope} [comma] paren_close TypeSpecifier [UID] [CommentBlock] .
func (p *parserMicroglotTokens) parseStatementAnnotation() *astStatementAnnotation {
	if p.expectOne(idl.TokenTypeKeywordAnnotation) == nil {
		return nil
	}
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}
	annotationScopes := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		(*parserMicroglotTokens).parseAnnotationScope,
		idl.TokenTypeParenClose)
	if annotationScopes == nil {
		return nil
	}

	maybeTypeSpecifier := p.parseTypeSpecifier()
	if maybeTypeSpecifier == nil {
		return nil
	}

	this := astStatementAnnotation{
		identifier:       *maybeIdentifier,
		annotationScopes: *annotationScopes,
		typeSpecifier:    *maybeTypeSpecifier,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeAt {
		maybeUid := p.parseUID()
		if maybeUid == nil {
			return nil
		}
		this.uid = maybeUid
	}

	maybeToken = p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
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

	annotationInstances := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		(*parserMicroglotTokens).parseAnnotationInstance,
		idl.TokenTypeParenClose)
	if annotationInstances == nil {
		return nil
	}

	return &astAnnotationApplication{
		annotationInstances,
	}
}

// TODO 2023.08.21: should this use QualifiedIdentifier instead of `identifier [dot identifier]`?
// AnnotationInstance = identifier [dot identifier] paren_open Value paren_close
func (p *parserMicroglotTokens) parseAnnotationInstance() *astAnnotationInstance {
	// TODO 2023.08.16: is "namespace" the right nomenclature?
	var namespaceIdentifier *idl.Token = nil
	identifier := p.expectOne(idl.TokenTypeIdentifier)
	if identifier == nil {
		return nil
	}

	maybeToken := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeDot,
		idl.TokenTypeParenOpen,
	})
	if maybeToken == nil {
		return nil
	}

	if maybeToken.Type == idl.TokenTypeDot {
		namespaceIdentifier = identifier
		identifier = p.expectOne(idl.TokenTypeIdentifier)
		if identifier == nil {
			return nil
		}

		maybeToken = p.expectOne(idl.TokenTypeParenOpen)
		if maybeToken == nil {
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
		namespaceIdentifier: namespaceIdentifier,
		identifier:          *identifier,
		value:               value,
	}
}

// UID = at int_lit
func (p *parserMicroglotTokens) parseUID() *astValueLiteralInt {
	if p.expectOne(idl.TokenTypeAt) == nil {
		return nil
	}
	return p.parseValueLiteralInt()
}

// Value = ValueUnary | ValueBinary | ValueLiteral | ValueIdentifier
func (p *parserMicroglotTokens) parseValue() expression {
	maybeToken := p.peek()
	if maybeToken == nil {
		p.report(exc.CodeUnexpectedEOF, fmt.Sprint(exc.CodeUnexpectedEOF, "unexpected EOF (expecting a value)"))
		return nil
	}

	switch maybeToken.Type {
	case idl.TokenTypePlus, idl.TokenTypeMinus, idl.TokenTypeExclamation:
		return p.parseValueUnary()
	case idl.TokenTypeParenOpen:
		return p.parseValueBinary()
	case idl.TokenTypeKeywordTrue, idl.TokenTypeKeywordFalse:
		return p.parseValueLiteralBool()
	case idl.TokenTypeIntegerDecimal, idl.TokenTypeIntegerHex, idl.TokenTypeIntegerOctal, idl.TokenTypeIntegerBinary:
		return p.parseValueLiteralInt()
	case idl.TokenTypeFloatDecimal, idl.TokenTypeFloatHex:
		return p.parseValueLiteralFloat()
	case idl.TokenTypeText:
		return p.parseValueLiteralText()
	case idl.TokenTypeData:
		return p.parseValueLiteralData()
	case idl.TokenTypeSquareOpen:
		return p.parseValueLiteralList()
	case idl.TokenTypeCurlyOpen:
		return p.parseValueLiteralStruct()
	case idl.TokenTypeIdentifier:
		return p.parseValueIdentifier()
	default:
		// TODO 2023.08.21: replace CodeUnknownFatal with something meaningful
		p.report(exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting an expression)", maybeToken.Value))
		return nil
	}
}

// ValueUnary = (plus | minus | bang ) Value
func (p *parserMicroglotTokens) parseValueUnary() *astValueUnary {
	maybeOperator := p.expectOneOf([]idl.TokenType{
		idl.TokenTypePlus,
		idl.TokenTypeMinus,
		idl.TokenTypeExclamation,
	})
	if maybeOperator == nil {
		return nil
	}

	maybeOperand := p.parseValue()
	if maybeOperand == nil {
		return nil
	}

	return &astValueUnary{
		operator: *maybeOperator,
		operand:  maybeOperand,
	}
}

// ValueBinary = paren_open Value ( equal_compare | equal_not | equal_lesser |
//
//	equal_greater | bool_and | bool_or | bin_and | bin_or | bin_xor | shift_left |
//	shift_right | plus | slash | star | mod ) Value paren_close
func (p *parserMicroglotTokens) parseValueBinary() *astValueBinary {
	if p.expectOne(idl.TokenTypeParenOpen) == nil {
		return nil
	}

	maybeLeftOperand := p.parseValue()
	if maybeLeftOperand == nil {
		return nil
	}
	maybeOperator := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeComparison,
		idl.TokenTypeNotComparison,
		idl.TokenTypeLesserEqual,
		idl.TokenTypeGreaterEqual,
		idl.TokenTypeAmpersand,
		idl.TokenTypePipe,
		idl.TokenTypeBinAnd,
		idl.TokenTypeBinOr,
		idl.TokenTypeCaret,
		// TODO 2023.08.21: these don't seem to be lexed, currently?
		// idl.TokenTypeShiftLeft,
		// idl.TokenTypeShiftRight,
		idl.TokenTypePlus,
		idl.TokenTypeMinus,
		idl.TokenTypeSlash,
		idl.TokenTypeStar,
		idl.TokenTypePercent,
	})
	if maybeOperator == nil {
		return nil
	}
	maybeRightOperand := p.parseValue()
	if maybeRightOperand == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeParenClose) == nil {
		return nil
	}

	return &astValueBinary{
		leftOperand:  maybeLeftOperand,
		operator:     *maybeOperator,
		rightOperand: maybeRightOperand,
	}
}

func (p *parserMicroglotTokens) parseValueLiteralBool() *astValueLiteralBool {
	maybeToken := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeKeywordTrue,
		idl.TokenTypeKeywordFalse,
	})
	if maybeToken == nil {
		return nil
	}

	var value bool
	if maybeToken.Type == idl.TokenTypeKeywordTrue {
		value = true
	} else {
		value = false
	}
	return &astValueLiteralBool{
		value,
	}
}

// ValueLiteralInt = decimal_lit | binary_lit | octal_lit | hex_lit
func (p *parserMicroglotTokens) parseValueLiteralInt() *astValueLiteralInt {
	maybeToken := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeIntegerDecimal,
		idl.TokenTypeIntegerHex,
		idl.TokenTypeIntegerOctal,
		idl.TokenTypeIntegerBinary,
	})
	if maybeToken == nil {
		return nil
	}

	i, err := strconv.ParseUint(maybeToken.Value, 0, 64)
	if err != nil {
		// TODO 2023.08.21: replace CodeUnknownFatal with something meaningful
		p.report(exc.CodeUnknownFatal, fmt.Sprintf("invalid integer literal %s", maybeToken.Value))
		return nil
	}

	return &astValueLiteralInt{
		token: *maybeToken,
		value: i,
	}
}

func (p *parserMicroglotTokens) parseValueLiteralFloat() *astValueLiteralFloat {
	maybeToken := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeFloatDecimal,
		idl.TokenTypeFloatHex,
	})
	if maybeToken == nil {
		return nil
	}

	f, err := strconv.ParseFloat(maybeToken.Value, 64)
	if err != nil {
		// TODO 2023.08.21: replace CodeUnknownFatal with something meaningful
		p.report(exc.CodeUnknownFatal, fmt.Sprintf("invalid floating-point literal %s", maybeToken.Value))
		return nil
	}

	return &astValueLiteralFloat{
		token: *maybeToken,
		value: f,
	}
}

func (p *parserMicroglotTokens) parseValueLiteralText() *astValueLiteralText {
	maybeToken := p.expectOne(idl.TokenTypeText)
	if maybeToken == nil {
		return nil
	}
	return &astValueLiteralText{
		value: *maybeToken,
	}
}

func (p *parserMicroglotTokens) parseValueLiteralData() *astValueLiteralData {
	maybeToken := p.expectOne(idl.TokenTypeData)
	if maybeToken == nil {
		return nil
	}
	return &astValueLiteralData{
		value: *maybeToken,
	}
}

// ValueLiteralList    = square_open [Value {comma Value} {comma}] square_close .
func (p *parserMicroglotTokens) parseValueLiteralList() *astValueLiteralList {
	values := applyOverCommaSeparatedList(p,
		idl.TokenTypeSquareOpen,
		(*parserMicroglotTokens).parseValue,
		idl.TokenTypeSquareClose,
	)
	if values == nil {
		return nil
	}

	return &astValueLiteralList{
		values,
	}
}

// ValueLiteralStruct         = brace_open [LiteralStructPair {comma LiteralStructPair} {comma}] brace_close
func (p *parserMicroglotTokens) parseValueLiteralStruct() *astValueLiteralStruct {
	values := applyOverCommaSeparatedList(p,
		idl.TokenTypeCurlyOpen,
		(*parserMicroglotTokens).parseLiteralStructPair,
		idl.TokenTypeCurlyClose)
	if values == nil {
		return nil
	}

	return &astValueLiteralStruct{
		values,
	}
}

// LiteralStructPair    = identifier colon Value .
func (p *parserMicroglotTokens) parseLiteralStructPair() *astLiteralStructPair {
	maybeIdentifier := p.parseValueIdentifier()
	if maybeIdentifier == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeColon) == nil {
		return nil
	}

	maybeValue := p.parseValue()
	if maybeValue == nil {
		return nil
	}

	return &astLiteralStructPair{
		identifier: *maybeIdentifier,
		value:      maybeValue,
	}
}

// ValueIdentifier = QualifiedIdentifier
// QualifiedIdentifier = identifier { dot identifier } .
func (p *parserMicroglotTokens) parseValueIdentifier() *astValueIdentifier {
	identifier := p.expectOne(idl.TokenTypeIdentifier)
	if identifier == nil {
		return nil
	}

	components := []idl.Token{*identifier}
	for {
		maybeToken := p.peek()
		if maybeToken == nil || maybeToken.Type != idl.TokenTypeDot {
			break
		}
		if p.expectOne(idl.TokenTypeDot) == nil {
			return nil
		}
		identifier := p.expectOne(idl.TokenTypeIdentifier)
		if identifier == nil {
			return nil
		}
		components = append(components, *identifier)
	}

	return &astValueIdentifier{
		qualifiedIdentifier: components,
	}
}
