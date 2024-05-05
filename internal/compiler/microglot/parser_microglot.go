package microglot

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/iter"
	"gopkg.microglot.org/compiler.go/internal/proto"
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
	loc    proto.SourceLocation
	tokens idl.Lookahead[*idl.Token]
}

func (p *parserMicroglotTokens) report(code string, message string) {
	_ = p.reporter.Report(exc.New(exc.Location{
		URI:            p.uri,
		SourceLocation: p.loc,
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
	p.report(exc.CodeUnexpectedToken, fmt.Sprintf("unexpected %s (expecting %v)", maybeToken.Value, expectedTypes))
	return nil
}

// generic application of parsing lists of zero or more comma-separated nodes, allowing an optional trailing comma
func applyOverCommaSeparatedList[N node](p *parserMicroglotTokens, tOpen idl.TokenType, parser func() *N, tClose idl.TokenType) []N {
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
		maybeValue := parser()
		if maybeValue == nil {
			return nil
		}
		values = append(values, *maybeValue)

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

			maybeValue = parser()
			if maybeValue == nil {
				return nil
			}
			values = append(values, *maybeValue)
		}
	}

	if p.expectOne(tClose) == nil {
		return nil
	}

	return values
}

func applyOverCommentedBlockWithPrefix[N node, P node](p *parserMicroglotTokens, prefixToken idl.TokenType, prefixParser func() *P, valueParser func() *N) *astCommentedBlock[N, P] {
	if p.expectOne(idl.TokenTypeCurlyOpen) == nil {
		return nil
	}

	this := astCommentedBlock[N, P]{}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
		maybeCommentBlock := p.parseCommentBlock()
		if maybeCommentBlock == nil {
			return nil
		}
		this.innerComments = maybeCommentBlock
	}

	if prefixParser != nil {
		maybeToken := p.peek()
		if maybeToken != nil && maybeToken.Type == prefixToken {
			maybePrefix := prefixParser()
			if maybePrefix == nil {
				return nil
			}
			this.prefix = maybePrefix
		}
	}

	for {
		maybeToken := p.peek()
		if maybeToken != nil && maybeToken.Type == idl.TokenTypeCurlyClose {
			break
		}

		maybeValue := valueParser()
		if maybeValue == nil {
			return nil
		}
		this.values = append(this.values, *maybeValue)
	}

	if p.expectOne(idl.TokenTypeCurlyClose) == nil {
		return nil
	}

	return &this
}

func applyOverCommentedBlock[N node](p *parserMicroglotTokens, parser func() *N) *astCommentedBlock[N, node] {
	return applyOverCommentedBlockWithPrefix[N, node](p, idl.TokenTypeNewline, nil, parser)
}

// Module = [CommentBlock] StatementSyntax { Statement }
func (p *parserMicroglotTokens) ParseModule() *astModule {
	this := astModule{
		URI: p.uri,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
		maybeCommentBlock := p.parseCommentBlock()
		if maybeCommentBlock == nil {
			return nil
		}
		this.comments = maybeCommentBlock
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
			maybeStatementModuleMeta := p.parseStatementModuleMeta()
			if maybeStatementModuleMeta == nil {
				return nil
			}
			maybeStatement = maybeStatementModuleMeta
		case idl.TokenTypeKeywordImport:
			maybeStatementImport := p.parseStatementImport()
			if maybeStatementImport == nil {
				return nil
			}
			maybeStatement = maybeStatementImport
		case idl.TokenTypeKeywordAnnotation:
			maybeStatementAnnotation := p.parseStatementAnnotation()
			if maybeStatementAnnotation == nil {
				return nil
			}
			maybeStatement = maybeStatementAnnotation
		case idl.TokenTypeKeywordConst:
			maybeStatementConst := p.parseStatementConst()
			if maybeStatementConst == nil {
				return nil
			}
			maybeStatement = maybeStatementConst
		case idl.TokenTypeKeywordEnum:
			maybeStatementEnum := p.parseStatementEnum()
			if maybeStatementEnum == nil {
				return nil
			}
			maybeStatement = maybeStatementEnum
		case idl.TokenTypeKeywordStruct:
			maybeStatementStruct := p.parseStatementStruct()
			if maybeStatementStruct == nil {
				return nil
			}
			maybeStatement = maybeStatementStruct
		case idl.TokenTypeKeywordAPI:
			maybeStatementAPI := p.parseStatementAPI()
			if maybeStatementAPI == nil {
				return nil
			}
			maybeStatement = maybeStatementAPI
		case idl.TokenTypeKeywordSDK:
			maybeStatementSDK := p.parseStatementSDK()
			if maybeStatementSDK == nil {
				return nil
			}
			maybeStatement = maybeStatementSDK
		case idl.TokenTypeKeywordImpl:
			maybeStatementImpl := p.parseStatementImpl()
			if maybeStatementImpl == nil {
				return nil
			}
			maybeStatement = maybeStatementImpl
		default:
			p.report(exc.CodeUnexpectedToken, fmt.Sprintf("unexpected %s (expecting a statement)", maybeToken.Value))
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
		astNode: astNode{p.loc},
		syntax:  *textNode,
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
		maybeCommentBlock := p.parseCommentBlock()
		if maybeCommentBlock == nil {
			return nil
		}
		this.comments = maybeCommentBlock
	}

	this.loc = p.loc
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
		maybeCommentBlock := p.parseCommentBlock()
		if maybeCommentBlock == nil {
			return nil
		}
		this.comments = maybeCommentBlock
	}

	this.loc = p.loc
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
		p.parseAnnotationScope,
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
		maybeCommentBlock := p.parseCommentBlock()
		if maybeCommentBlock == nil {
			return nil
		}
		this.comments = maybeCommentBlock
	}

	this.loc = p.loc
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
		astNode:       astNode{p.loc},
		identifier:    *maybeIdentifier,
		typeSpecifier: *maybeTypeSpecifier,
		value:         *maybeValue,
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

	commentedBlock := applyOverCommentedBlock(p, p.parseEnumerant)
	if commentedBlock == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astStatementEnum{
		astNode:       astNode{p.loc},
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

	commentedBlock := applyOverCommentedBlock(p, p.parseStructElement)
	if commentedBlock == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astStatementStruct{
		astNode:       astNode{p.loc},
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

	commentedBlock := applyOverCommentedBlock(p, p.parseAPIMethod)
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

	this.loc = p.loc
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

	commentedBlock := applyOverCommentedBlock(p, p.parseSDKMethod)
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

	this.loc = p.loc
	return &this
}

// StatementImpl = impl TypeName ImplAs brace_open [CommentBlock] [ImplRequires] { ImplMethod } brace_close Metadata .
func (p *parserMicroglotTokens) parseStatementImpl() *astStatementImpl {
	if p.expectOne(idl.TokenTypeKeywordImpl) == nil {
		return nil
	}

	maybeTypeName := p.parseTypeName()
	if maybeTypeName == nil {
		return nil
	}

	maybeImplAs := p.parseImplAs()
	if maybeImplAs == nil {
		return nil
	}

	this := astStatementImpl{
		typeName: *maybeTypeName,
		as:       *maybeImplAs,
	}

	commentedBlock := applyOverCommentedBlockWithPrefix(p, idl.TokenTypeKeywordRequires, p.parseImplRequires, p.parseImplMethod)
	if commentedBlock == nil {
		return nil
	}
	this.innerComments = commentedBlock.innerComments
	this.requires = commentedBlock.prefix
	this.methods = commentedBlock.values

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	this.meta = *maybeMeta

	this.loc = p.loc
	return &this
}

// StepProse = prose .
func (p *parserMicroglotTokens) parseStepProse() *astStepProse {
	maybeProse := p.expectOne(idl.TokenTypeProse)
	if maybeProse == nil {
		return nil
	}
	return &astStepProse{
		astNode: astNode{p.loc},
		prose:   *maybeProse,
	}
}

// ValueOrInvocation = Invocation | Value .
func (p *parserMicroglotTokens) parseValueOrInvocation() *valueorinvocation {
	var this valueorinvocation
	maybeToken := p.peek()
	if maybeToken != nil && slices.Contains([]idl.TokenType{
		idl.TokenTypeKeywordAwait,
		idl.TokenTypeKeywordAsync,
		idl.TokenTypeDollar,
		idl.TokenTypeIdentifier,
	}, maybeToken.Type) {
		maybeInvocation := p.parseInvocation()
		if maybeInvocation == nil {
			return nil
		}
		this = *maybeInvocation
	} else {
		maybeValue := p.parseValue()
		if maybeValue == nil {
			return nil
		}
		this = *maybeValue
	}
	return &this
}

// StepVar = var identifier TypeSpecifier [equal ValueOrInvocation] .
func (p *parserMicroglotTokens) parseStepVar() *astStepVar {
	if p.expectOne(idl.TokenTypeKeywordVar) == nil {
		return nil
	}

	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	this := astStepVar{
		identifier: *maybeIdentifier,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeEqual {
		p.advance()

		maybeValue := p.parseValueOrInvocation()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	}

	this.loc = p.loc
	return &this
}

// StepSet = set QualifiedIdentifier equal ValueOrInvocation .
func (p *parserMicroglotTokens) parseStepSet() *astStepSet {
	if p.expectOne(idl.TokenTypeKeywordSet) == nil {
		return nil
	}

	maybeIdentifier := p.parseQualifiedIdentifier()
	if maybeIdentifier == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeEqual) == nil {
		return nil
	}

	maybeValue := p.parseValueOrInvocation()
	if maybeValue == nil {
		return nil
	}

	return &astStepSet{
		astNode:    astNode{p.loc},
		identifier: *maybeIdentifier,
		value:      *maybeValue,
	}
}

// ConditionBlock = ValueBinary ImplBlock
func (p *parserMicroglotTokens) parseConditionBlock() *astConditionBlock {
	maybeCondition := p.parseValueBinary()
	if maybeCondition == nil {
		return nil
	}

	maybeBlock := p.parseImplBlock()
	if maybeBlock == nil {
		return nil
	}

	return &astConditionBlock{
		astNode:   astNode{p.loc},
		condition: *maybeCondition,
		block:     *maybeBlock,
	}
}

// StepIf = if ConditionBlock { else if ConditionBlock } [ else ImplBlock ] .
func (p *parserMicroglotTokens) parseStepIf() *astStepIf {
	if p.expectOne(idl.TokenTypeKeywordIf) == nil {
		return nil
	}

	first := p.parseConditionBlock()
	if first == nil {
		return nil
	}

	conditions := []astConditionBlock{*first}
	for {
		maybeToken := p.peek()
		if maybeToken == nil || maybeToken.Type != idl.TokenTypeKeywordElse {
			break
		}
		maybeToken = p.peekN(1)
		if maybeToken == nil || maybeToken.Type != idl.TokenTypeKeywordIf {
			break
		}

		p.advance()
		p.advance()
		maybeConditionBlock := p.parseConditionBlock()
		if maybeConditionBlock == nil {
			return nil
		}
		conditions = append(conditions, *maybeConditionBlock)
	}

	this := astStepIf{
		conditions: conditions,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordElse {
		maybeElseBlock := p.parseImplBlock()
		if maybeElseBlock == nil {
			return nil
		}

		this.elseBlock = maybeElseBlock
	}

	this.loc = p.loc
	return &this
}

// SwitchCase = case Value {comma Value} ImplBlock .
func (p *parserMicroglotTokens) parseSwitchCase() *astSwitchCase {
	if p.expectOne(idl.TokenTypeKeywordCase) == nil {
		return nil
	}

	first := p.parseValue()
	if first == nil {
		return nil
	}
	values := []astValue{*first}

	for {
		maybeToken := p.peek()
		if maybeToken == nil || maybeToken.Type != idl.TokenTypeComma {
			break
		}

		maybeValue := p.parseValue()
		if maybeValue == nil {
			return nil
		}

		values = append(values, *maybeValue)
	}

	maybeBlock := p.parseImplBlock()
	if maybeBlock == nil {
		return nil
	}

	return &astSwitchCase{
		astNode: astNode{p.loc},
		values:  values,
		block:   *maybeBlock,
	}
}

// SwitchDefault = default ImplBlock .
func (p *parserMicroglotTokens) parseSwitchDefault() *astSwitchDefault {
	if p.expectOne(idl.TokenTypeKeywordDefault) == nil {
		return nil
	}

	maybeBlock := p.parseImplBlock()
	if maybeBlock == nil {
		return nil
	}

	return &astSwitchDefault{
		astNode: astNode{p.loc},
		block:   *maybeBlock,
	}
}

func (p *parserMicroglotTokens) parseSwitchElement() *switchelement {
	var value switchelement
	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordCase {
		switchCase := p.parseSwitchCase()
		if switchCase == nil {
			return nil
		}
		value = *switchCase
	} else {
		switchDefault := p.parseSwitchDefault()
		if switchDefault == nil {
			return nil
		}
		value = *switchDefault
	}
	return &value
}

// StepSwitch = switch Value curly_open {SwitchElement} curl_close .
func (p *parserMicroglotTokens) parseStepSwitch() *astStepSwitch {
	if p.expectOne(idl.TokenTypeKeywordSwitch) == nil {
		return nil
	}

	maybeValue := p.parseValue()
	if maybeValue == nil {
		return nil
	}

	commentedBlock := applyOverCommentedBlock(p, p.parseSwitchElement)
	if commentedBlock == nil {
		return nil
	}

	return &astStepSwitch{
		astNode:       astNode{p.loc},
		innerComments: commentedBlock.innerComments,
		cases:         commentedBlock.values,
	}
}

// StepWhile = while ConditionBlock .
func (p *parserMicroglotTokens) parseStepWhile() *astStepWhile {
	if p.expectOne(idl.TokenTypeKeywordWhile) == nil {
		return nil
	}

	maybeConditionBlock := p.parseConditionBlock()
	if maybeConditionBlock == nil {
		return nil
	}

	return &astStepWhile{
		astNode:        astNode{p.loc},
		conditionBlock: *maybeConditionBlock,
	}
}

// StepFor = for ForKeyName comma ForValueName in Value ImplBlock .
func (p *parserMicroglotTokens) parseStepFor() *astStepFor {
	if p.expectOne(idl.TokenTypeKeywordFor) == nil {
		return nil
	}

	maybeKeyName := p.expectOne(idl.TokenTypeIdentifier)
	if maybeKeyName == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeComma) == nil {
		return nil
	}

	maybeValueName := p.expectOne(idl.TokenTypeIdentifier)
	if maybeValueName == nil {
		return nil
	}

	if p.expectOne(idl.TokenTypeKeywordIn) == nil {
		return nil
	}

	maybeValue := p.parseValue()
	if maybeValue == nil {
		return nil
	}

	maybeBlock := p.parseImplBlock()
	if maybeBlock == nil {
		return nil
	}

	return &astStepFor{
		astNode:   astNode{p.loc},
		keyName:   *maybeKeyName,
		valueName: *maybeValueName,
		value:     *maybeValue,
		block:     *maybeBlock,
	}
}

// StepReturn = return [Value] .
func (p *parserMicroglotTokens) parseStepReturn() *astStepReturn {
	if p.expectOne(idl.TokenTypeKeywordReturn) == nil {
		return nil
	}

	this := astStepReturn{}

	maybeToken := p.peek()
	if maybeToken != nil && slices.Contains([]idl.TokenType{
		idl.TokenTypePlus,
		idl.TokenTypeMinus,
		idl.TokenTypeExclamation,
		idl.TokenTypeParenOpen,
		idl.TokenTypeKeywordTrue,
		idl.TokenTypeKeywordFalse,
		idl.TokenTypeIntegerDecimal,
		idl.TokenTypeIntegerHex,
		idl.TokenTypeIntegerOctal,
		idl.TokenTypeIntegerBinary,
		idl.TokenTypeFloatDecimal,
		idl.TokenTypeFloatHex,
		idl.TokenTypeText,
		idl.TokenTypeData,
		idl.TokenTypeSquareOpen,
		idl.TokenTypeCurlyOpen,
		idl.TokenTypeIdentifier,
	}, maybeToken.Type) {
		maybeValue := p.parseValue()
		if maybeValue == nil {
			return nil
		}
		this.value = maybeValue
	}

	this.loc = p.loc
	return &this
}

// StepThrow = throw Value .
func (p *parserMicroglotTokens) parseStepThrow() *astStepThrow {
	if p.expectOne(idl.TokenTypeKeywordThrow) == nil {
		return nil
	}

	maybeValue := p.parseValue()
	if maybeValue == nil {
		return nil
	}

	return &astStepThrow{
		astNode: astNode{p.loc},
		value:   *maybeValue,
	}
}

// InvocationCatch = catch identifier ImplBlock .
func (p *parserMicroglotTokens) parseInvocationCatch() *astInvocationCatch {
	if p.expectOne(idl.TokenTypeKeywordCatch) == nil {
		return nil
	}

	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeBlock := p.parseImplBlock()
	if maybeBlock == nil {
		return nil
	}

	return &astInvocationCatch{
		astNode:    astNode{p.loc},
		identifier: *maybeIdentifier,
		block:      *maybeBlock,
	}
}

// InvocationAwait = await identifier [InvocationCatch] .
func (p *parserMicroglotTokens) parseInvocationAwait() *astInvocationAwait {
	if p.expectOne(idl.TokenTypeKeywordAwait) == nil {
		return nil
	}

	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	this := astInvocationAwait{
		identifier: *maybeIdentifier,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordCatch {
		maybeCatch := p.parseInvocationCatch()
		if maybeCatch == nil {
			return nil
		}
		this.catch = maybeCatch
	}

	this.loc = p.loc
	return &this
}

// ImplIdentifier = [dollar dot] QualifiedIdentifier
func (p *parserMicroglotTokens) parseImplIdentifier() *astImplIdentifier {
	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeDollar {
		p.advance()
		if p.expectOne(idl.TokenTypeDot) == nil {
			return nil
		}
	}

	return (*astImplIdentifier)(p.parseQualifiedIdentifier())
}

// InvocationAsync = async ImplIdentifier paren_open [InvocationParameters] paren_close .
func (p *parserMicroglotTokens) parseInvocationAsync() *astInvocationAsync {
	if p.expectOne(idl.TokenTypeKeywordAsync) == nil {
		return nil
	}

	maybeImplIdentifier := p.parseImplIdentifier()
	if maybeImplIdentifier == nil {
		return nil
	}

	parameters := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		p.parseValue,
		idl.TokenTypeParenClose)
	if parameters == nil {
		return nil
	}

	return &astInvocationAsync{
		astNode:        astNode{p.loc},
		implIdentifier: *maybeImplIdentifier,
		parameters:     parameters,
	}
}

// InvocationDirect = ImplIdentifier paren_open [InvocationParameters] paren_close [InvocationCatch] .
func (p *parserMicroglotTokens) parseInvocationDirect() *astInvocationDirect {
	maybeImplIdentifier := p.parseImplIdentifier()
	if maybeImplIdentifier == nil {
		return nil
	}

	parameters := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		p.parseValue,
		idl.TokenTypeParenClose)
	if parameters == nil {
		return nil
	}

	this := astInvocationDirect{
		implIdentifier: *maybeImplIdentifier,
		parameters:     parameters,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordCatch {
		maybeCatch := p.parseInvocationCatch()
		if maybeCatch == nil {
			return nil
		}
		this.catch = maybeCatch
	}

	this.loc = p.loc
	return &this
}

// Invocation = InvocationAwait | InvocationAsync | InvocationDirect .
func (p *parserMicroglotTokens) parseInvocation() *astInvocation {
	maybeToken := p.peek()
	if maybeToken == nil {
		p.report(exc.CodeUnexpectedEOF, fmt.Sprint(exc.CodeUnexpectedEOF, "unexpected EOF (expecting an invocation)"))
		return nil
	}

	this := astInvocation{}

	switch maybeToken.Type {
	case idl.TokenTypeKeywordAwait:
		maybeInvocation := p.parseInvocationAwait()
		if maybeInvocation == nil {
			return nil
		}
		this.invocation = *maybeInvocation
	case idl.TokenTypeKeywordAsync:
		maybeInvocation := p.parseInvocationAsync()
		if maybeInvocation == nil {
			return nil
		}
		this.invocation = *maybeInvocation
	case idl.TokenTypeDollar, idl.TokenTypeIdentifier:
		maybeInvocation := p.parseInvocationDirect()
		if maybeInvocation == nil {
			return nil
		}
		this.invocation = *maybeInvocation
	}

	this.loc = p.loc
	return &this
}

// StepExec = exec Invocation .
func (p *parserMicroglotTokens) parseStepExec() *astStepExec {
	if p.expectOne(idl.TokenTypeKeywordExec) == nil {
		return nil
	}

	maybeInvocation := p.parseInvocation()
	if maybeInvocation == nil {
		return nil
	}

	return &astStepExec{
		astNode:    astNode{p.loc},
		invocation: *maybeInvocation,
	}
}

// ImplBlockStep = StepProse | StepVar | StepSet | StepIf | StepSwitch | StepWhile | StepFor | StepReturn | StepThrow | StepExec .
func (p *parserMicroglotTokens) parseImplBlockStep() *step {
	maybeToken := p.peek()
	if maybeToken == nil {
		p.report(exc.CodeUnexpectedEOF, fmt.Sprint(exc.CodeUnexpectedEOF, "unexpected EOF (expecting an implementation step"))
		return nil
	}

	var value step
	switch maybeToken.Type {
	case idl.TokenTypeProse:
		value = p.parseStepProse()
	case idl.TokenTypeKeywordVar:
		value = p.parseStepVar()
	case idl.TokenTypeKeywordSet:
		value = p.parseStepSet()
	case idl.TokenTypeKeywordIf:
		value = p.parseStepIf()
	case idl.TokenTypeKeywordSwitch:
		value = p.parseStepSwitch()
	case idl.TokenTypeKeywordWhile:
		value = p.parseStepWhile()
	case idl.TokenTypeKeywordFor:
		value = p.parseStepFor()
	case idl.TokenTypeKeywordReturn:
		value = p.parseStepReturn()
	case idl.TokenTypeKeywordThrow:
		value = p.parseStepThrow()
	case idl.TokenTypeKeywordExec:
		value = p.parseStepExec()
	default:
		p.report(exc.CodeUnexpectedToken, fmt.Sprintf("unexpected %s (expecting an implementation step)", maybeToken.Value))
		return nil
	}

	return &value
}

// ImplBlock = curly_open { ImplBlockStep } curl_close .
func (p *parserMicroglotTokens) parseImplBlock() *astImplBlock {
	commentedBlock := applyOverCommentedBlock(p, p.parseImplBlockStep)
	if commentedBlock == nil {
		return nil
	}

	return &astImplBlock{
		astNode:       astNode{p.loc},
		innerComments: commentedBlock.innerComments,
		steps:         commentedBlock.values,
	}
}

// ImplAPIMethod = identifier APIMethodInput APIMethodReturns ImplBlock Metadata .
func (p *parserMicroglotTokens) parseImplAPIMethod() *astImplAPIMethod {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeMethodInput := p.parseAPIMethodInput()
	if maybeMethodInput == nil {
		return nil
	}

	maybeMethodReturns := p.parseAPIMethodReturns()
	if maybeMethodReturns == nil {
		return nil
	}

	maybeBlock := p.parseImplBlock()
	if maybeBlock == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astImplAPIMethod{
		astNode:       astNode{p.loc},
		identifier:    *maybeIdentifier,
		methodInput:   *maybeMethodInput,
		methodReturns: *maybeMethodReturns,
		block:         *maybeBlock,
		meta:          *maybeMeta,
	}
}

// ImplSDKMethod = identifier SDKMethodInput [SDKMethodReturns] [nothrows] ImplBlock Metadata .
func (p *parserMicroglotTokens) parseImplSDKMethod() *astImplSDKMethod {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeMethodInput := p.parseSDKMethodInput()
	if maybeMethodInput == nil {
		return nil
	}

	this := astImplSDKMethod{
		identifier:  *maybeIdentifier,
		methodInput: *maybeMethodInput,
		nothrows:    false,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordReturns {
		maybeMethodReturns := p.parseSDKMethodReturns()
		if maybeMethodReturns == nil {
			return nil
		}
		this.methodReturns = maybeMethodReturns
	}
	maybeToken = p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordNothrows {
		p.advance()
		this.nothrows = true
	}

	maybeBlock := p.parseImplBlock()
	if maybeBlock == nil {
		return nil
	}
	this.block = *maybeBlock

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	this.meta = *maybeMeta

	this.loc = p.loc
	return &this
}

// ImplMethod = ( ImplAPIMethod | ImplSDKMethod ) .
func (p *parserMicroglotTokens) parseImplMethod() *implmethod {
	var value implmethod
	maybeToken := p.peekN(2)
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeColon {
		apiMethod := p.parseImplAPIMethod()
		if apiMethod == nil {
			return nil
		}
		value = *apiMethod
	} else {
		sdkMethod := p.parseImplSDKMethod()
		if sdkMethod == nil {
			return nil
		}
		value = *sdkMethod
	}
	return &value
}

// ImplRequirement = identifier TypeSpecifier [CommentBlock] .
func (p *parserMicroglotTokens) parseImplRequirement() *astImplRequirement {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeTypeSpecifier := p.parseTypeSpecifier()
	if maybeTypeSpecifier == nil {
		return nil
	}

	this := astImplRequirement{
		identifier:    *maybeIdentifier,
		typeSpecifier: *maybeTypeSpecifier,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeComment {
		maybeCommentBlock := p.parseCommentBlock()
		if maybeCommentBlock == nil {
			return nil
		}
		this.comments = maybeCommentBlock
	}

	this.loc = p.loc
	return &this
}

// ImplRequires = requires brace_open [CommentBlock] { ImplRequirement } brace_close
func (p *parserMicroglotTokens) parseImplRequires() *astImplRequires {
	if p.expectOne(idl.TokenTypeKeywordRequires) == nil {
		return nil
	}

	commentedBlock := applyOverCommentedBlock(p, p.parseImplRequirement)
	if commentedBlock == nil {
		return nil
	}

	return &astImplRequires{
		astNode:       astNode{p.loc},
		innerComments: commentedBlock.innerComments,
		requirements:  commentedBlock.values,
	}
}

// ImplAs = as paren_open TypeSpecifier { comma TypeSpecifier } [comma] paren_close .
func (p *parserMicroglotTokens) parseImplAs() *astImplAs {
	if p.expectOne(idl.TokenTypeKeywordAs) == nil {
		return nil
	}

	types := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		p.parseTypeSpecifier,
		idl.TokenTypeParenClose)
	if types == nil {
		return nil
	}

	return &astImplAs{
		astNode: astNode{p.loc},
		types:   types,
	}
}

// SDKMethodInput = paren_open [SDKMethodParameter {comma SDKMethodParameter} [comma]] paren_close .
func (p *parserMicroglotTokens) parseSDKMethodInput() *astSDKMethodInput {
	parameters := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		p.parseSDKMethodParameter,
		idl.TokenTypeParenClose)
	if parameters == nil {
		return nil
	}

	return &astSDKMethodInput{
		astNode:    astNode{p.loc},
		parameters: parameters,
	}
}

// SDKMethodReturns = returns paren_open TypeSpecifier paren_close .
func (p *parserMicroglotTokens) parseSDKMethodReturns() *astSDKMethodReturns {
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
	return &astSDKMethodReturns{
		astNode:       astNode{p.loc},
		typeSpecifier: *maybeTypeSpecifier,
	}
}

// SDKMethod = identifier SDKMethodInput [SDKMethodReturns] [nothrows] Metadata .
func (p *parserMicroglotTokens) parseSDKMethod() *astSDKMethod {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeMethodInput := p.parseSDKMethodInput()
	if maybeMethodInput == nil {
		return nil
	}

	this := astSDKMethod{
		identifier:  *maybeIdentifier,
		methodInput: *maybeMethodInput,
		nothrows:    false,
	}

	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordReturns {
		maybeMethodReturns := p.parseSDKMethodReturns()
		if maybeMethodReturns == nil {
			return nil
		}
		this.methodReturns = maybeMethodReturns
	}

	maybeToken = p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordNothrows {
		p.advance()
		this.nothrows = true
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	this.meta = *maybeMeta

	this.loc = p.loc
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
		astNode:       astNode{p.loc},
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
		p.parseTypeSpecifier,
		idl.TokenTypeParenClose)
	if extensions == nil {
		return nil
	}

	return &astExtension{
		astNode:    astNode{p.loc},
		extensions: extensions,
	}
}

// APIMethodInput = paren_open TypeSpecifier paren_close .
func (p *parserMicroglotTokens) parseAPIMethodInput() *astAPIMethodInput {
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

	return &astAPIMethodInput{
		astNode:       astNode{p.loc},
		typeSpecifier: *maybeTypeSpecifier,
	}
}

// APIMethodReturns = returns paren_open TypeSpecifier paren_close .
func (p *parserMicroglotTokens) parseAPIMethodReturns() *astAPIMethodReturns {
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
	return &astAPIMethodReturns{
		astNode:       astNode{p.loc},
		typeSpecifier: *maybeTypeSpecifier,
	}
}

// APIMethod = identifier APIMethodInput APIMethodReturns Metadata .
func (p *parserMicroglotTokens) parseAPIMethod() *astAPIMethod {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
	if maybeIdentifier == nil {
		return nil
	}

	maybeMethodInput := p.parseAPIMethodInput()
	if maybeMethodInput == nil {
		return nil
	}

	maybeMethodReturns := p.parseAPIMethodReturns()
	if maybeMethodReturns == nil {
		return nil
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}

	return &astAPIMethod{
		astNode:       astNode{p.loc},
		identifier:    *maybeIdentifier,
		methodInput:   *maybeMethodInput,
		methodReturns: *maybeMethodReturns,
		meta:          *maybeMeta,
	}
}

// StructElement = Field | Union .
func (p *parserMicroglotTokens) parseStructElement() *structelement {
	var value structelement
	maybeToken := p.peek()
	if maybeToken != nil && maybeToken.Type == idl.TokenTypeKeywordUnion {
		value = p.parseUnion()
	} else {
		value = p.parseField()
	}
	return &value
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

	commentedBlock := applyOverCommentedBlock(p, p.parseUnionField)
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

	this.loc = p.loc
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
		astNode:       astNode{p.loc},
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
		this.value = *maybeValue
	}

	maybeMeta := p.parseMetadata()
	if maybeMeta == nil {
		return nil
	}
	this.meta = *maybeMeta

	this.loc = p.loc
	return &this
}

// Note: this doesn't exist in the EBNF, but it's useful for reducing duplicate parse code, so...
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
		maybeCommentBlock := p.parseCommentBlock()
		if maybeCommentBlock == nil {
			return nil
		}
		this.comments = maybeCommentBlock
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
		astNode: astNode{p.loc},
		scope:   *maybeToken,
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
	this.loc = p.loc
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
			p.parseTypeSpecifier,
			idl.TokenTypeAngleClose)
		if parameters == nil {
			return nil
		}
		this.parameters = parameters
	}

	this.loc = p.loc
	return &this
}

func (p *parserMicroglotTokens) parseCommentBlock() *astCommentBlock {
	comments := p.expectSeveral(idl.TokenTypeComment)
	if comments == nil {
		return nil
	}
	return &astCommentBlock{
		astNode:  astNode{p.loc},
		comments: comments,
	}
}

// AnnotationApplication = dollar paren_open [AnnotationInstance] { comma AnnotationInstance } [comma] paren_close
func (p *parserMicroglotTokens) parseAnnotationApplication() *astAnnotationApplication {
	if p.expectOne(idl.TokenTypeDollar) == nil {
		return nil
	}

	annotationInstances := applyOverCommaSeparatedList(p,
		idl.TokenTypeParenOpen,
		p.parseAnnotationInstance,
		idl.TokenTypeParenClose)
	if annotationInstances == nil {
		return nil
	}

	return &astAnnotationApplication{
		astNode:             astNode{p.loc},
		annotationInstances: annotationInstances,
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
		astNode:             astNode{p.loc},
		namespaceIdentifier: namespaceIdentifier,
		identifier:          *identifier,
		value:               *value,
	}
}

// UID = at int_lit
func (p *parserMicroglotTokens) parseUID() *astValueLiteralInt {
	if p.expectOne(idl.TokenTypeAt) == nil {
		return nil
	}
	maybeUid := p.parseValueLiteralInt()
	if maybeUid == nil {
		return nil
	}
	// The compiler reserves MaxUint64 (a.k.a. Incomplete) to mean "generate a value at compile-time",
	// so it's not allowed to be explicitly used as a uid.
	if (*maybeUid).val == idl.Incomplete {
		return nil
	}
	return maybeUid
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
		astNode:    astNode{p.loc},
		identifier: *maybeIdentifier,
		meta:       *maybeMeta,
	}
}

// Value = ValueUnary | ValueBinary | ValueLiteral | ValueIdentifier
func (p *parserMicroglotTokens) parseValue() *astValue {
	maybeToken := p.peek()
	if maybeToken == nil {
		p.report(exc.CodeUnexpectedEOF, fmt.Sprint(exc.CodeUnexpectedEOF, "unexpected EOF (expecting a value)"))
		return nil
	}

	this := astValue{}

	switch maybeToken.Type {
	case idl.TokenTypePlus, idl.TokenTypeMinus, idl.TokenTypeExclamation:
		maybeValue := p.parseValueUnary()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeParenOpen:
		maybeValue := p.parseValueBinary()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeKeywordTrue, idl.TokenTypeKeywordFalse:
		maybeValue := p.parseValueLiteralBool()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeIntegerDecimal, idl.TokenTypeIntegerHex, idl.TokenTypeIntegerOctal, idl.TokenTypeIntegerBinary:
		maybeValue := p.parseValueLiteralInt()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeFloatDecimal, idl.TokenTypeFloatHex:
		maybeValue := p.parseValueLiteralFloat()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeText:
		maybeValue := p.parseValueLiteralText()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeData:
		maybeValue := p.parseValueLiteralData()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeSquareOpen:
		maybeValue := p.parseValueLiteralList()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeCurlyOpen:
		maybeValue := p.parseValueLiteralStruct()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	case idl.TokenTypeIdentifier:
		maybeValue := p.parseValueIdentifier()
		if maybeValue == nil {
			return nil
		}
		this.value = *maybeValue
	default:
		p.report(exc.CodeUnexpectedToken, fmt.Sprintf("unexpected %s (expecting a value)", maybeToken.Value))
		return nil
	}

	return &this
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
		astNode:  astNode{p.loc},
		operator: *maybeOperator,
		operand:  *maybeOperand,
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
		astNode:      astNode{p.loc},
		leftOperand:  *maybeLeftOperand,
		operator:     *maybeOperator,
		rightOperand: *maybeRightOperand,
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

	value := maybeToken.Type == idl.TokenTypeKeywordTrue
	return &astValueLiteralBool{
		astNode: astNode{p.loc},
		val:     value,
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
		p.report(exc.CodeInvalidLiteral, fmt.Sprintf("invalid integer literal %s", maybeToken.Value))
		return nil
	}

	return &astValueLiteralInt{
		astNode: astNode{p.loc},
		token:   *maybeToken,
		val:     i,
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
		p.report(exc.CodeInvalidLiteral, fmt.Sprintf("invalid floating-point literal %s", maybeToken.Value))
		return nil
	}

	return &astValueLiteralFloat{
		astNode: astNode{p.loc},
		token:   *maybeToken,
		val:     f,
	}
}

func (p *parserMicroglotTokens) parseValueLiteralText() *astValueLiteralText {
	maybeToken := p.expectOne(idl.TokenTypeText)
	if maybeToken == nil {
		return nil
	}
	return &astValueLiteralText{
		astNode: astNode{p.loc},
		val:     *maybeToken,
	}
}

func (p *parserMicroglotTokens) parseValueLiteralData() *astValueLiteralData {
	maybeToken := p.expectOne(idl.TokenTypeData)
	if maybeToken == nil {
		return nil
	}
	return &astValueLiteralData{
		astNode: astNode{p.loc},
		val:     *maybeToken,
	}
}

// ValueLiteralList    = square_open [Value {comma Value} {comma}] square_close .
func (p *parserMicroglotTokens) parseValueLiteralList() *astValueLiteralList {
	values := applyOverCommaSeparatedList(p,
		idl.TokenTypeSquareOpen,
		p.parseValue,
		idl.TokenTypeSquareClose,
	)
	if values == nil {
		return nil
	}

	return &astValueLiteralList{
		astNode: astNode{p.loc},
		vals:    values,
	}
}

// ValueLiteralStruct         = brace_open [LiteralStructPair {comma LiteralStructPair} {comma}] brace_close
func (p *parserMicroglotTokens) parseValueLiteralStruct() *astValueLiteralStruct {
	values := applyOverCommaSeparatedList(p,
		idl.TokenTypeCurlyOpen,
		p.parseLiteralStructPair,
		idl.TokenTypeCurlyClose)
	if values == nil {
		return nil
	}

	return &astValueLiteralStruct{
		astNode: astNode{p.loc},
		vals:    values,
	}
}

// LiteralStructPair = identifier colon Value .
func (p *parserMicroglotTokens) parseLiteralStructPair() *astLiteralStructPair {
	maybeIdentifier := p.expectOne(idl.TokenTypeIdentifier)
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
		astNode:    astNode{p.loc},
		identifier: *maybeIdentifier,
		value:      *maybeValue,
	}
}

// ValueIdentifier = QualifiedIdentifier
func (p *parserMicroglotTokens) parseValueIdentifier() *astValueIdentifier {
	return (*astValueIdentifier)(p.parseQualifiedIdentifier())
}

// QualifiedIdentifier = identifier { dot identifier } .
func (p *parserMicroglotTokens) parseQualifiedIdentifier() *astQualifiedIdentifier {
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

	return &astQualifiedIdentifier{
		astNode:    astNode{p.loc},
		components: components,
	}
}
