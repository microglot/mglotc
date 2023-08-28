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

func (p *parserMicroglotTokens) peekN(n uint8) *idl.Token {
	maybeToken := p.tokens.Lookahead(p.ctx, n)
	if !maybeToken.IsPresent() {
		return nil
	}
	return maybeToken.Value()
}

func (p *parserMicroglotTokens) peek() *idl.Token {
	return p.peekN(0)
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
func applyOverCommaSeparatedList[N interface {
	node
	comparable
}](p *parserMicroglotTokens, tOpen idl.TokenType, parser func(*parserMicroglotTokens) N, tClose idl.TokenType) []N {
	var zeroValue N

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
		if maybeValue == zeroValue {
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
			if maybeValue == zeroValue {
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

func applyOverCommentedBlock[N interface {
	node
	comparable
}](p *parserMicroglotTokens, parser func(*parserMicroglotTokens) N) *astCommentedBlock[N] {
	var zeroValue N
	if p.expectOne(idl.TokenTypeCurlyOpen) == nil {
		return nil
	}

	this := astCommentedBlock[N]{
		values: []N{},
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
		this.innerComments = *p.parseCommentBlock()
	}

	for {
		maybeToken := p.peek()
		if maybeToken != nil && maybeToken.Type == idl.TokenTypeCurlyClose {
			break
		}

		maybeValue := parser(p)
		if maybeValue == zeroValue {
			return nil
		}
		this.values = append(this.values, maybeValue)
	}

	if p.expectOne(idl.TokenTypeCurlyClose) == nil {
		return nil
	}

	return &this
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

		var maybeStatement statement
		switch maybeToken.Type {
		case idl.TokenTypeKeywordModule:
			maybeStatement = p.parseStatementModuleMeta()
		case idl.TokenTypeKeywordImport:
			maybeStatement = p.parseStatementImport()
		case idl.TokenTypeKeywordAnnotation:
			maybeStatement = p.parseStatementAnnotation()
		case idl.TokenTypeKeywordConst:
			maybeStatement = p.parseStatementConst()
		case idl.TokenTypeKeywordEnum:
			maybeStatement = p.parseStatementEnum()
		case idl.TokenTypeKeywordStruct:
			maybeStatement = p.parseStatementStruct()
		case idl.TokenTypeKeywordAPI:
			maybeStatement = p.parseStatementAPI()
		case idl.TokenTypeKeywordSDK:
			maybeStatement = p.parseStatementSDK()
		default:
			// TODO 2023.08.21: replace CodeUnknownFatal with something meaningful
			p.report(exc.CodeUnknownFatal, fmt.Sprintf("unexpected %s (expecting a statement)", maybeToken.Value))
			return nil
		}

		if maybeStatement == nil {
			return nil
		}
		this.statements = append(this.statements, maybeStatement)
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

		this.annotationApplication = annotationApplicationNode
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
		annotationScopes: annotationScopes,
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

// StatementConst = const identifier TypeSpecifier equal Value Metadata .
func (p *parserMicroglotTokens) parseStatementConst() *astStatementConst {
	if p.expectOne(idl.TokenTypeKeywordConst) == nil {
		return nil
	}

	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeTypeSpecifier := p.parseTypeSpecifier()
	if maybeTypeSpecifier == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeEqual) == nil {
		return nil
	}

	maybeValue := p.parseValue()
	if maybeValue == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astStatementConst{
		identifier:    *maybeIdentifier,
		typeSpecifier: *maybeTypeSpecifier,
		value:         maybeValue,
		meta:          *maybeMeta,
	}
}

// StatementEnum = enum identifier brace_open [CommentBlock] { Enumerant } brace_close Metadata .
func (p *parserMicroglotTokens) parseStatementEnum() *astStatementEnum {
	if p.expectOne(idl.TokenTypeKeywordEnum) == nil {
		return nil
	}

	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	commentedBlock := applyOverCommentedBlock(p, (*parserMicroglotTokens).parseEnumerant)
	if commentedBlock == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astStatementEnum{
		identifier:    *maybeIdentifier,
		enumerants:    commentedBlock.values,
		innerComments: commentedBlock.innerComments,
		meta:          *maybeMeta,
	}
}

// StatementStruct = struct TypeName brace_open [CommentBlock] { StructElement } brace_close Metadata .
func (p *parserMicroglotTokens) parseStatementStruct() *astStatementStruct {
	if p.expectOne(idl.TokenTypeKeywordStruct) == nil {
		return nil
	}

	maybeTypeName := p.parseTypeName()
	if maybeTypeName == nil {
		return nil
	}

	commentedBlock := applyOverCommentedBlock(p, (*parserMicroglotTokens).parseStructElement)
	if commentedBlock == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astStatementStruct{
		typeName:      *maybeTypeName,
		innerComments: commentedBlock.innerComments,
		elements:      commentedBlock.values,
		meta:          *maybeMeta,
	}
}

// StatementAPI = api TypeName [Extension] brace_open [CommentBlock] { APIMethod } brace_close Metadata .
func (p *parserMicroglotTokens) parseStatementAPI() *astStatementAPI {
	if p.expectOne(idl.TokenTypeKeywordAPI) == nil {
		return nil
	}

	maybeTypeName := p.parseTypeName()
	if maybeTypeName == nil {
		return nil
	}

	this := astStatementAPI{
		typeName: *maybeTypeName,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordExtends {
		maybeExtends := p.parseExtension()
		if maybeExtends == nil {
			return nil
		}
		this.extends = maybeExtends
	}

	commentedBlock := applyOverCommentedBlock(p, (*parserMicroglotTokens).parseAPIMethod)
	if commentedBlock == nil {
		return nil
	}
	this.innerComments = commentedBlock.innerComments
	this.methods = commentedBlock.values

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	this.meta = *maybeMeta

	return &this
}

// StatementSDK = sdk TypeName [Extension] brace_open [CommentBlock] { SDKMethod } brace_close Metadata .
func (p *parserMicroglotTokens) parseStatementSDK() *astStatementSDK {
	if p.expectOne(idl.TokenTypeKeywordSDK) == nil {
		return nil
	}

	maybeTypeName := p.parseTypeName()
	if maybeTypeName == nil {
		return nil
	}

	this := astStatementSDK{
		typeName: *maybeTypeName,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordExtends {
		maybeExtends := p.parseExtension()
		if maybeExtends == nil {
			return nil
		}
		this.extends = maybeExtends
	}

	commentedBlock := applyOverCommentedBlock(p, (*parserMicroglotTokens).parseSDKMethod)
	if commentedBlock == nil {
		return nil
	}
	this.innerComments = commentedBlock.innerComments
	this.methods = commentedBlock.values

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	return &this
}

// SDKMethod = identifier SDKMethodInput [SDKMethodReturns] [nothrows] Metadata .
// SDKMethodInput      = paren_open [SDKMethodParameter {comma SDKMethodParameter} [comma]] paren_close .
// SDKMethodReturns    = returns paren_open TypeSpecifier paren_close .
func (p *parserMicroglotTokens) parseSDKMethod() *astSDKMethod {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	parameters := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		(*parserMicroglotTokens).parseSDKMethodParameter,
		idl.TokenTypeParenClose)
	if parameters == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeKeywordReturns) == nil {
		return nil
	}
	if p.expectOne(idl.TokenTypeParenOpen) == nil {
		return nil
	}
	maybeTypeSpecifier := p.parseTypeSpecifier()
	if maybeTypeSpecifier == nil {
		return nil
	}
	if p.expectOne(idl.TokenTypeParenClose) == nil {
		return nil
	}

	this := astSDKMethod{
		identifier:          *maybeIdentifier,
		parameters:          parameters,
		returnTypeSpecifier: *maybeTypeSpecifier,
		nothrows:            false,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordNothrows {
		p.advance()
		this.nothrows = true
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	this.meta = *maybeMeta

	return &this
}

// SDKMethodParameter  = identifier TypeSpecifier .
func (p *parserMicroglotTokens) parseSDKMethodParameter() *astSDKMethodParameter {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeTypeSpecifier := p.parseTypeSpecifier()
	if maybeTypeSpecifier == nil {
		return nil
	}

	return &astSDKMethodParameter{
		identifier:    *maybeIdentifier,
		typeSpecifier: *maybeTypeSpecifier,
	}
}

// Extension: extends paren_open TypeSpecifier { comma TypeSpecifier } [comma] paren_close .
func (p *parserMicroglotTokens) parseExtension() *astExtension {
	if p.expectOne(idl.TokenTypeKeywordExtends) == nil {
		return nil
	}

	extensions := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		(*parserMicroglotTokens).parseTypeSpecifier,
		idl.TokenTypeParenClose)
	if extensions == nil {
		return nil
	}

	return &astExtension{
		extensions,
	}
}

// APIMethod         = identifier APIMethodInput APIMethodReturns Metadata .
// APIMethodInput    = paren_open [TypeSpecifier] paren_close .
// APIMethodReturns  = returns paren_open [TypeSpecifier] paren_close .
func (p *parserMicroglotTokens) parseAPIMethod() *astAPIMethod {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	this := astAPIMethod{
		identifier: *maybeIdentifier,
	}

	if p.expectOne(idl.TokenTypeParenOpen) == nil {
		return nil
	}
	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeColon {
		maybeTypeSpecifier := p.parseTypeSpecifier()
		if maybeTypeSpecifier == nil {
			return nil
		}
		this.inputTypeSpecifier = maybeTypeSpecifier
	}
	if p.expectOne(idl.TokenTypeParenClose) == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeKeywordReturns) == nil {
		return nil
	}
	if p.expectOne(idl.TokenTypeParenOpen) == nil {
		return nil
	}
	maybeToken = p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeColon {
		maybeTypeSpecifier := p.parseTypeSpecifier()
		if maybeTypeSpecifier == nil {
			return nil
		}
		this.returnTypeSpecifier = maybeTypeSpecifier
	}
	if p.expectOne(idl.TokenTypeParenClose) == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	this.meta = *maybeMeta

	return &this
}

// StructElement = Field | Union .
func (p *parserMicroglotTokens) parseStructElement() structelement {
	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordUnion {
		return p.parseUnion()
	} else {
		return p.parseField()
	}
}

// Union = union [identifier] brace_open [CommentBlock] { UnionField } brace_close Metadata .
func (p *parserMicroglotTokens) parseUnion() *astUnion {
	if p.expectOne(idl.TokenTypeKeywordUnion) == nil {
		return nil
	}

	this := astUnion{}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeIdentifier {
		maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
		if maybeIdentifier == nil {
			return nil
		}
		this.identifier = maybeIdentifier
	}

	commentedBlock := applyOverCommentedBlock(p, (*parserMicroglotTokens).parseUnionField)
	if commentedBlock == nil {
		return nil
	}
	this.innerComments = commentedBlock.innerComments
	this.fields = commentedBlock.values

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	this.meta = *maybeMeta

	return &this
}

// UnionField = identifier TypeSpecifier Metadata .
func (p *parserMicroglotTokens) parseUnionField() *astUnionField {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeTypeSpecifier := p.parseTypeSpecifier()
	if maybeTypeSpecifier == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astUnionField{
		identifier:    *maybeIdentifier,
		typeSpecifier: *maybeTypeSpecifier,
		meta:          *maybeMeta,
	}
}

// Field = identifier TypeSpecifier [equal Value] Metadata .
func (p *parserMicroglotTokens) parseField() *astField {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeTypeSpecifier := p.parseTypeSpecifier()
	if maybeTypeSpecifier == nil {
		return nil
	}

	this := astField{
		identifier:    *maybeIdentifier,
		typeSpecifier: *maybeTypeSpecifier,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeEqual {
		p.advance()

		maybeValue := p.parseValue()
		if maybeValue == nil {
			return nil
		}
		this.value = maybeValue
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	this.meta = *maybeMeta

	return &this
}

// TODO 2023.08.22: this doesn't exist in the EBNF, but it's useful for reducing duplicate parse code, so...
// Metadata: [UID] [AnnotationApplication] [CommentBlock]
func (p *parserMicroglotTokens) parseMetadata() *astMetadata {
	this := astMetadata{}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeAt {
		maybeUid := p.parseUID()
		if maybeUid == nil {
			return nil
		}
		this.uid = maybeUid
	}

	maybeToken = p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeDollar {
		maybeAnnotationApplication := p.parseAnnotationApplication()
		if maybeAnnotationApplication == nil {
			return nil
		}
		this.annotationApplication = maybeAnnotationApplication
	}

	maybeToken = p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
		this.comments = *p.parseCommentBlock()
	}

	return &this
}

// AnnotationScope = module | union | struct | field | enumerant | enum | api | apimethod | sdk | sdkmethod | const | star .
func (p *parserMicroglotTokens) parseAnnotationScope() *astAnnotationScope {
	maybeToken := p.expectOneOf([]idl.TokenType{
		idl.TokenTypeKeywordModule,
		idl.TokenTypeKeywordUnion,
		idl.TokenTypeKeywordStruct,
		idl.TokenTypeKeywordField,
		idl.TokenTypeKeywordEnumerant,
		idl.TokenTypeKeywordEnum,
		idl.TokenTypeKeywordAPI,
		// TODO 2023.08.22: appears to be missing from the lexer
		// idl.TokenTypeKeywordAPIMethod,
		idl.TokenTypeKeywordSDK,
		// TODO 2023.08.22: appears to be missing from the lexer
		// idl.TokenTypeKeywordSDKMethod,
		idl.TokenTypeKeywordConst,
		idl.TokenTypeStar,
	})
	if maybeToken == nil {
		return nil
	}
	return &astAnnotationScope{
		scope: *maybeToken,
	}
}

// TypeSpecifier = colon QualifiedTypeName .
// QualifiedTypeName = [identifier dot] TypeName .
func (p *parserMicroglotTokens) parseTypeSpecifier() *astTypeSpecifier {
	if p.expectOne(idl.TokenTypeColon) == nil {
		return nil
	}

	this := astTypeSpecifier{}

	maybeToken := p.peekN(1)
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeDot {
		maybeQualifier := p.expectOne(idl.TokenTypeIdentifier)
		if maybeQualifier == nil {
			return nil
		}
		this.qualifier = maybeQualifier

		if p.expectOne(idl.TokenTypeDot) == nil {
			return nil
		}
	}

	maybeTypeName := p.parseTypeName()
	if maybeTypeName == nil {
		return nil
	}
	this.typeName = *maybeTypeName
	return &this
}

// TypeName = identifer [TypeParameters] .
// TypeParameters = angle_open TypeSpecifier {comma TypeSpecifier} [comma] angle_close
func (p *parserMicroglotTokens) parseTypeName() *astTypeName {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	this := astTypeName{
		identifier: *maybeIdentifier,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeAngleOpen {
		parameters := applyOverCommaSeparatedList(p,
			idl.TokenTypeAngleOpen,
			(*parserMicroglotTokens).parseTypeSpecifier,
			idl.TokenTypeAngleClose)
		if parameters == nil {
			return nil
		}
		this.parameters = parameters
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

// Enumerant = identifier Metadata .
func (p *parserMicroglotTokens) parseEnumerant() *astEnumerant {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astEnumerant{
		identifier: *maybeIdentifier,
		meta:       *maybeMeta,
	}
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
