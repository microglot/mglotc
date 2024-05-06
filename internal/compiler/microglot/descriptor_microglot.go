package microglot

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

func (ast *astModule) ToModule() (*proto.Module, error) {
	converter := astConverter{
		ast: ast,
	}
	return converter.convert()
}

type astConverter struct {
	ast *astModule

	// SourceCodeInfo is accumulated here, as side-effects of the conversion from ast -> descriptor
	p        *idl.PathState
	location []*proto.Location
}

func mapFrom[F any, T any](c *astConverter, in []F, f func(*F) T) []T {
	if in != nil {
		out := make([]T, 0, len(in))

		for _, element := range in {
			out = append(out, f(&element))
			c.p.IncrementIndex()
		}
		return out
	}
	return nil
}

func (c *astConverter) resetPathState() {
	c.p = &idl.PathState{}
	c.location = []*proto.Location{}
}

func (c *astConverter) maybeEmitLocation(loc *proto.SourceLocation) {
	location := proto.Location{
		Path: c.p.CopyPath(),
		Span: &proto.Span{
			Start: start,
			End:   end,
		},
	}
	c.location = append(c.location, &location)
}

func (c *astConverter) convert() (*proto.Module, error) {
	c.resetPathState()

	this := proto.Module{
		URI: c.ast.URI,
	}

	for _, statement := range c.ast.statements {
		s, ok := statement.(*astStatementModuleMeta)
		if ok {
			c.p.PushFieldNumber( /* UID */ 2)
			this.UID = s.uid.val
			c.maybeEmitLocation(&s.uid.loc, &s.uid.loc)
			c.p.PopFieldNumber()

			c.p.PushFieldNumber( /* AnnotationApplications */ 4)
			this.AnnotationApplications = c.fromAnnotationApplication(s.annotationApplication)
			c.p.PopFieldNumber()
		}
	}

	c.p.PushFieldNumber( /* Imports */ 5)
	c.p.PushIndex()
	for _, statement := range c.ast.statements {
		s, ok := statement.(*astStatementImport)
		if ok {
			this.Imports = append(this.Imports, c.fromStatementImport(s))
			c.p.IncrementIndex()
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Annotations */ 11)
	c.p.PushIndex()
	for _, statement := range c.ast.statements {
		s, ok := statement.(*astStatementAnnotation)
		if ok {
			this.Annotations = append(this.Annotations, c.fromStatementAnnotation(s))
			c.p.IncrementIndex()
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Constants */ 10)
	c.p.PushIndex()
	for _, statement := range c.ast.statements {
		s, ok := statement.(*astStatementConst)
		if ok {
			this.Constants = append(this.Constants, c.fromStatementConst(s))
			c.p.IncrementIndex()
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Enums */ 7)
	c.p.PushIndex()
	for _, statement := range c.ast.statements {
		s, ok := statement.(*astStatementEnum)
		if ok {
			this.Enums = append(this.Enums, c.fromStatementEnum(s))
			c.p.IncrementIndex()
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Structs */ 6)
	c.p.PushIndex()
	for _, statement := range c.ast.statements {
		s, ok := statement.(*astStatementStruct)
		if ok {
			this.Structs = append(this.Structs, c.fromStatementStruct(s))
			c.p.IncrementIndex()
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* APIs */ 8)
	c.p.PushIndex()
	for _, statement := range c.ast.statements {
		s, ok := statement.(*astStatementAPI)
		if ok {
			this.APIs = append(this.APIs, c.fromStatementAPI(s))
			c.p.IncrementIndex()
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* SDKs */ 9)
	c.p.PushIndex()
	for _, statement := range c.ast.statements {
		s, ok := statement.(*astStatementSDK)
		if ok {
			this.SDKs = append(this.SDKs, c.fromStatementSDK(s))
			c.p.IncrementIndex()
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	// TODO 2023.09.05: missing from descriptor?
	for _, statement := range c.ast.statements {
		_, ok := statement.(*astStatementImpl)
		if ok {
		}
	}

	if this.UID == 0 {
		return nil, fmt.Errorf("you must specify a UID for module %s", c.ast.URI)
	}

	pkg, err := c.protobufPackage(this.AnnotationApplications, c.ast.URI)
	if err != nil {
		return nil, err
	}
	this.ProtobufPackage = pkg

	this.SourceCodeInfo = &proto.SourceCodeInfo{
		Locations: c.location,
	}

	return &this, nil
}

func (c *astConverter) protobufPackage(annotationApplications []*proto.AnnotationApplication, moduleURI string) (string, error) {
	for _, annotationApplication := range annotationApplications {
		forward, ok := annotationApplication.Annotation.Reference.(*proto.TypeSpecifier_Forward)
		if ok {
			microglot, ok := forward.Forward.Reference.(*proto.ForwardReference_Microglot)
			if ok {
				// this annotation is special, in that we are using it before linking!
				// TODO 2023.11.02: this isn't quite right!
				// Specifically, it assumes the user hasn't given the protobuf import an
				// alias other than "Protobuf".
				// However, as of right now, the protobuf import doesn't even exist, and
				// it's not 100% clear whether it should be allowed to give it a different
				// alias, or it should be built-in in some way, or something else entirely.
				if microglot.Microglot.Qualifier == "Protobuf" && microglot.Microglot.Name.Name == "Package" {
					text, ok := annotationApplication.Value.Kind.(*proto.Value_Text)
					if ok {
						return text.Text.Value, nil
					} else {
						return "", errors.New("$Protobuf.Package() annotation value must be text")
					}
				}
			}
		}
	}

	// in absence of a $Protobuf.Package() annotation, derive a default from the module URI
	u, err := url.Parse(moduleURI)
	if err != nil {
		return "", err
	}
	hostSegments := strings.Split(strings.TrimRight(u.Host, "."), ".") // Remove trailing dot from host to handle fully qualified domains
	base := strings.Join(hostSegments, ".")
	if base != "" {
		base = base + "."
	}
	pathSegments := strings.Split(strings.TrimLeft(u.Path, "/"), "/")
	pathSegments[len(pathSegments)-1], _, _ = strings.Cut(pathSegments[len(pathSegments)-1], ".")
	p := strings.Join(pathSegments, ".")
	defaultProtobufPackage := base + p

	return defaultProtobufPackage, nil
}

func (c *astConverter) fromStatementImport(statementImport *astStatementImport) *proto.Import {
	c.maybeEmitLocation(&statementImport.loc, &statementImport.loc)

	this := proto.Import{
		// ModuleUID:
		// ImportedUID:
		IsDot:       statementImport.name.Value == ".",
		ImportedURI: statementImport.uri.val.Value,
		Alias:       statementImport.name.Value,
	}

	c.p.PushFieldNumber( /* CommentBlock */ 6)
	this.CommentBlock = c.fromCommentBlock(statementImport.comments)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromStatementAnnotation(statementAnnotation *astStatementAnnotation) *proto.Annotation {
	c.maybeEmitLocation(&statementAnnotation.loc, &statementAnnotation.loc)
	this := proto.Annotation{
		Reference: fromTypeUID(statementAnnotation.uid),
		Name:      statementAnnotation.identifier.Value,
	}

	c.p.PushFieldNumber( /* Scopes */ 3)
	this.Scopes = c.fromAnnotationScopes(statementAnnotation.annotationScopes)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Type */ 4)
	this.Type = c.fromTypeSpecifier(&statementAnnotation.typeSpecifier)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* CommentBlock */ 5)
	this.DescriptorCommentBlock = c.fromCommentBlock(statementAnnotation.comments)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromStatementConst(statementConst *astStatementConst) *proto.Constant {
	c.maybeEmitLocation(&statementConst.loc, &statementConst.loc)
	this := proto.Constant{
		Reference: fromTypeUID(statementConst.meta.uid),
		Name:      statementConst.identifier.Value,
	}

	c.p.PushFieldNumber( /* Type */ 3)
	this.Type = c.fromTypeSpecifier(&statementConst.typeSpecifier)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Value */ 4)
	this.Value = c.fromValue(&statementConst.value)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplications */ 5)
	this.AnnotationApplications = c.fromAnnotationApplication(statementConst.meta.annotationApplication)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* CommentBlock */ 6)
	this.CommentBlock = c.fromCommentBlock(statementConst.meta.comments)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromStatementEnum(statementEnum *astStatementEnum) *proto.Enum {
	c.maybeEmitLocation(&statementEnum.loc, &statementEnum.loc)
	result := proto.Enum{
		Reference: fromTypeUID(statementEnum.meta.uid),
		Name:      statementEnum.identifier.Value,
	}

	c.p.PushFieldNumber( /* CommentBlock */ 6)
	result.CommentBlock = c.fromCommentBlock(statementEnum.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnoationApplications */ 7)
	result.AnnotationApplications = c.fromAnnotationApplication(statementEnum.meta.annotationApplication)
	c.p.PopFieldNumber()

	normalizedEnumerants := statementEnum.enumerants

	var foundZero bool
	for _, en := range statementEnum.enumerants {
		if fromAttributeUID(en.meta.uid).AttributeUID == 0 {
			foundZero = true
			break
		}
	}
	if !foundZero {
		normalizedEnumerants = append(normalizedEnumerants, astEnumerant{
			identifier: idl.Token{
				Type:  idl.TokenTypeIdentifier,
				Value: "None",
			},
			meta: astMetadata{
				uid: &astValueLiteralInt{val: 0},
			},
		})
	}
	sort.Slice(normalizedEnumerants, func(i, j int) bool {
		return fromAttributeUID(normalizedEnumerants[i].meta.uid).AttributeUID < fromAttributeUID(normalizedEnumerants[j].meta.uid).AttributeUID
	})

	c.p.PushFieldNumber( /* Enumerants */ 3)
	c.p.PushIndex()
	result.Enumerants = mapFrom(c, normalizedEnumerants, c.fromEnumerant)
	c.p.PopIndex()
	c.p.PopFieldNumber()

	return &result
}

func (c *astConverter) fromStatementStruct(statementStruct *astStatementStruct) *proto.Struct {
	this := proto.Struct{
		Reference: fromTypeUID(statementStruct.meta.uid),
		Name:      c.fromTypeName(&statementStruct.typeName),
	}

	c.p.PushFieldNumber( /* CommentBlock */ 6)
	this.CommentBlock = c.fromCommentBlock(statementStruct.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplications */ 7)
	this.AnnotationApplications = c.fromAnnotationApplication(statementStruct.meta.annotationApplication)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Unions */ 4)
	c.p.PushIndex()
	for _, element := range statementStruct.elements {
		e, ok := element.(*astUnion)
		if ok {
			this.Unions = append(this.Unions, c.fromUnion(e))
			c.p.IncrementIndex()
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Fields */ 3)
	c.p.PushIndex()
	for _, element := range statementStruct.elements {
		switch e := element.(type) {
		case *astField:
			this.Fields = append(this.Fields, c.fromField(e))
			c.p.IncrementIndex()
		case *astUnion:
			for _, unionField := range e.fields {
				this.Fields = append(this.Fields, c.fromUnionField(&unionField, uint64(len(this.Unions)-1)))
				c.p.IncrementIndex()
			}
		}
	}
	c.p.PopIndex()
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromStatementAPI(statementAPI *astStatementAPI) *proto.API {
	var extends []*proto.TypeSpecifier
	if statementAPI.extends != nil {
		extends = mapFrom(c, statementAPI.extends.extensions, c.fromTypeSpecifier)
	}

	return &proto.API{
		Reference: fromTypeUID(statementAPI.meta.uid),
		Name:      c.fromTypeName(&statementAPI.typeName),
		Methods:   mapFrom(c, statementAPI.methods, c.fromAPIMethod),
		Extends:   extends,
		// Reserved:
		// ReservedNames:
		CommentBlock:           c.fromCommentBlock(statementAPI.meta.comments),
		AnnotationApplications: c.fromAnnotationApplication(statementAPI.meta.annotationApplication),
	}
}

func (c *astConverter) fromStatementSDK(statementSDK *astStatementSDK) *proto.SDK {
	this := proto.SDK{
		Reference: fromTypeUID(statementSDK.meta.uid),
	}

	c.p.PushFieldNumber( /* Name */ 2)
	this.Name = c.fromTypeName(&statementSDK.typeName)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Methods */ 3)
	c.p.PushIndex()
	this.Methods = mapFrom(c, statementSDK.methods, c.fromSDKMethod)
	c.p.PopIndex()
	c.p.PopFieldNumber()

	if statementSDK.extends != nil {
		c.p.PushFieldNumber( /* Extends */ 4)
		c.p.PushIndex()
		this.Extends = mapFrom(c, statementSDK.extends.extensions, c.fromTypeSpecifier)
		c.p.PopIndex()
		c.p.PopFieldNumber()
	}

	c.p.PushFieldNumber( /* CommentBlock */ 7)
	this.CommentBlock = c.fromCommentBlock(statementSDK.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplications */ 8)
	this.AnnotationApplications = c.fromAnnotationApplication(statementSDK.meta.annotationApplication)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromAPIMethod(apiMethod *astAPIMethod) *proto.APIMethod {
	this := proto.APIMethod{
		Reference: fromAttributeUID(apiMethod.meta.uid),
		Name:      apiMethod.identifier.Value,
	}

	c.p.PushFieldNumber( /* Input */ 3)
	this.Input = c.fromTypeSpecifier(&apiMethod.methodInput.typeSpecifier)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Output */ 4)
	this.Output = c.fromTypeSpecifier(&apiMethod.methodReturns.typeSpecifier)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* CommentBlock */ 5)
	this.CommentBlock = c.fromCommentBlock(apiMethod.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplications */ 6)
	this.AnnotationApplications = c.fromAnnotationApplication(apiMethod.meta.annotationApplication)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromSDKMethod(sdkMethod *astSDKMethod) *proto.SDKMethod {
	this := proto.SDKMethod{
		Reference: fromAttributeUID(sdkMethod.meta.uid),
		Name:      sdkMethod.identifier.Value,
		NoThrows:  sdkMethod.nothrows,
	}

	c.p.PushFieldNumber( /* Input */ 3)
	c.p.PushIndex()
	this.Input = mapFrom(c, sdkMethod.methodInput.parameters, c.fromSDKMethodParameter)
	c.p.PopIndex()
	c.p.PopFieldNumber()

	if sdkMethod.methodReturns != nil {
		c.p.PushFieldNumber( /* Output */ 4)
		this.Output = c.fromTypeSpecifier(&sdkMethod.methodReturns.typeSpecifier)
		c.p.PopFieldNumber()
	}

	c.p.PushFieldNumber( /* CommentBlock */ 6)
	this.CommentBlock = c.fromCommentBlock(sdkMethod.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplications */ 7)
	this.AnnotationApplications = c.fromAnnotationApplication(sdkMethod.meta.annotationApplication)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromSDKMethodParameter(sdkMethodParameter *astSDKMethodParameter) *proto.SDKMethodInput {
	this := proto.SDKMethodInput{
		// TODO 2023.10.29: the ebnf and ast don't actually allow setting the InputUID,
		// which is different from every other kind of UID. Is this intentional?
		Reference: fromInputUID(nil),
		Name:      sdkMethodParameter.identifier.Value,
	}

	c.p.PushFieldNumber( /* Type */ 3)
	this.Type = c.fromTypeSpecifier(&sdkMethodParameter.typeSpecifier)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromField(field *astField) *proto.Field {
	this := proto.Field{
		Reference:  fromAttributeUID(field.meta.uid),
		Name:       field.identifier.Value,
		UnionIndex: nil,
	}

	c.p.PushFieldNumber( /* Type */ 3)
	this.Type = c.fromTypeSpecifier(&field.typeSpecifier)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* DefaultValue */ 4)
	this.DefaultValue = c.fromValue(&field.value)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* CommentBlock */ 6)
	this.CommentBlock = c.fromCommentBlock(field.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplications */ 7)
	this.AnnotationApplications = c.fromAnnotationApplication(field.meta.annotationApplication)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromUnion(union *astUnion) *proto.Union {
	this := proto.Union{
		Reference: fromAttributeUID(union.meta.uid),
		Name:      union.identifier.Value,
	}

	c.p.PushFieldNumber( /* CommentBlock */ 3)
	this.CommentBlock = c.fromCommentBlock(union.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplications */ 4)
	this.AnnotationApplications = c.fromAnnotationApplication(union.meta.annotationApplication)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromUnionField(unionField *astUnionField, unionIndex uint64) *proto.Field {
	this := proto.Field{
		Reference:    fromAttributeUID(unionField.meta.uid),
		Name:         unionField.identifier.Value,
		DefaultValue: nil,
		UnionIndex:   &unionIndex,
	}

	c.p.PushFieldNumber( /* Type */ 3)
	this.Type = c.fromTypeSpecifier(&unionField.typeSpecifier)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* CommentBlock */ 6)
	this.CommentBlock = c.fromCommentBlock(unionField.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplication */ 7)
	this.AnnotationApplications = c.fromAnnotationApplication(unionField.meta.annotationApplication)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromEnumerant(enumerant *astEnumerant) *proto.Enumerant {
	this := proto.Enumerant{
		Reference: fromAttributeUID(enumerant.meta.uid),
		Name:      enumerant.identifier.Value,
	}

	c.p.PushFieldNumber( /* CommentBlock */ 3)
	this.CommentBlock = c.fromCommentBlock(enumerant.meta.comments)
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* AnnotationApplications */ 4)
	this.AnnotationApplications = c.fromAnnotationApplication(enumerant.meta.annotationApplication)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromTypeSpecifier(typeSpecifier *astTypeSpecifier) *proto.TypeSpecifier {
	qualifier := ""
	if typeSpecifier.qualifier != nil {
		qualifier = typeSpecifier.qualifier.Value
	}

	return &proto.TypeSpecifier{
		Reference: &proto.TypeSpecifier_Forward{
			Forward: &proto.ForwardReference{
				Reference: &proto.ForwardReference_Microglot{
					Microglot: &proto.MicroglotForwardReference{
						Qualifier: qualifier,
						Name:      c.fromTypeName(&typeSpecifier.typeName),
					},
				},
			},
		},
	}
}

func (c *astConverter) fromAnnotationScope(annotationScope *astAnnotationScope) proto.AnnotationScope {
	switch annotationScope.scope.Type {
	case idl.TokenTypeKeywordModule:
		return proto.AnnotationScope_AnnotationScopeModule
	case idl.TokenTypeKeywordUnion:
		return proto.AnnotationScope_AnnotationScopeUnion
	case idl.TokenTypeKeywordStruct:
		return proto.AnnotationScope_AnnotationScopeStruct
	case idl.TokenTypeKeywordField:
		return proto.AnnotationScope_AnnotationScopeField
	case idl.TokenTypeKeywordEnumerant:
		return proto.AnnotationScope_AnnotationScopeEnumerant
	case idl.TokenTypeKeywordEnum:
		return proto.AnnotationScope_AnnotationScopeEnum
	case idl.TokenTypeKeywordAPI:
		return proto.AnnotationScope_AnnotationScopeAPI
	// TODO 2023.09.05: missing from the lexer and parser
	// case idl.TokenTypeKeywordAPIMethod:
	//   return proto.AnnotationScope_AnnotationScopeAPIMethod
	case idl.TokenTypeKeywordSDK:
		return proto.AnnotationScope_AnnotationScopeSDK
	// TODO 2023.09.05: missing from the lexer and parser
	// case idl.TokenTypeKeywordSDKMethod:
	//   return proto.AnnotationScope_AnnotationScopeSDKMethod
	case idl.TokenTypeKeywordConst:
		return proto.AnnotationScope_AnnotationScopeConst
	case idl.TokenTypeStar:
		return proto.AnnotationScope_AnnotationScopeStar
	}
	return proto.AnnotationScope_AnnotationScopeZero
}

func (c *astConverter) fromAnnotationScopes(annotationScopes []astAnnotationScope) []proto.AnnotationScope {
	c.p.PushIndex()
	scopes := mapFrom(c, annotationScopes, c.fromAnnotationScope)
	c.p.PopIndex()
	return scopes
}

func (c *astConverter) fromAnnotationInstance(annotationInstance *astAnnotationInstance) *proto.AnnotationApplication {
	this := proto.AnnotationApplication{}

	c.p.PushFieldNumber( /* Annotation */ 1)
	// This is admittedly weird, but the pseudo-type specifiers in annotation applications
	// are grammatically slightly different from a full-blown type specifier.
	this.Annotation = c.fromTypeSpecifier(&astTypeSpecifier{
		qualifier: annotationInstance.namespaceIdentifier,
		typeName: astTypeName{
			identifier: annotationInstance.identifier,
		},
	})
	c.p.PopFieldNumber()

	c.p.PushFieldNumber( /* Value */ 2)
	this.Value = c.fromValue(&annotationInstance.value)
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromCommentBlock(commentBlock *astCommentBlock) *proto.CommentBlock {
	if commentBlock != nil {
		c.p.PushIndex()
		lines := mapFrom(c, commentBlock.comments, func(line *idl.Token) string { return line.Value })
		c.p.PopIndex()
		return &proto.CommentBlock{
			Lines: lines,
		}
	}
	return nil
}

func (c *astConverter) fromAnnotationApplication(annotationApplication *astAnnotationApplication) []*proto.AnnotationApplication {
	if annotationApplication != nil {
		c.p.PushIndex()
		annotations := mapFrom(c, annotationApplication.annotationInstances, c.fromAnnotationInstance)
		c.p.PopIndex()
		return annotations
	}
	return nil
}

func (c *astConverter) fromOperationUnary(t *idl.Token) proto.OperationUnary {
	switch t.Type {
	case idl.TokenTypePlus:
		return proto.OperationUnary_OperationUnaryPositive
	case idl.TokenTypeMinus:
		return proto.OperationUnary_OperationUnaryNegative
	case idl.TokenTypeExclamation:
		return proto.OperationUnary_OperationUnaryNot
	}
	return proto.OperationUnary_OperationUnaryZero
}

func (c *astConverter) fromOperationBinary(t *idl.Token) proto.OperationBinary {
	switch t.Type {
	case idl.TokenTypeComparison:
		return proto.OperationBinary_OperationBinaryEqual
	case idl.TokenTypeNotComparison:
		return proto.OperationBinary_OperationBinaryNotEqual
	case idl.TokenTypeLesserEqual:
		return proto.OperationBinary_OperationBinaryLessThanEqual
	case idl.TokenTypeGreaterEqual:
		return proto.OperationBinary_OperationBinaryGreaterThanEqual
	case idl.TokenTypeAmpersand:
		return proto.OperationBinary_OperationBinaryAnd
	case idl.TokenTypePipe:
		return proto.OperationBinary_OperationBinaryOr
	case idl.TokenTypeBinAnd:
		return proto.OperationBinary_OperationBinaryBinAnd
	case idl.TokenTypeBinOr:
		return proto.OperationBinary_OperationBinaryBinOr
	case idl.TokenTypeCaret:
		return proto.OperationBinary_OperationBinaryBitXor
	// TODO 2023.09.07: these don't seem to be lexed, currently?
	// case idl.TokenTypeShiftLeft:
	// case idl.TokenTypeShiftRight:
	case idl.TokenTypePlus:
		return proto.OperationBinary_OperationBinaryAdd
	case idl.TokenTypeMinus:
		return proto.OperationBinary_OperationBinarySubtract
	case idl.TokenTypeSlash:
		return proto.OperationBinary_OperationBinaryDivide
	case idl.TokenTypeStar:
		return proto.OperationBinary_OperationBinaryMultiply
	case idl.TokenTypePercent:
		return proto.OperationBinary_OperationBinaryModulo
	}
	return proto.OperationBinary_OperationBinaryZero
}

func (c *astConverter) fromValue(value *astValue) *proto.Value {
	this := proto.Value{}

	switch v := value.value.(type) {
	case astValueUnary:
		c.p.PushFieldNumber( /* Unary */ 20)
		{
			inner := proto.ValueUnary{}

			c.p.PushFieldNumber( /* Operation */ 1)
			inner.Operation = c.fromOperationUnary(&v.operator)
			c.p.PopFieldNumber()

			c.p.PushFieldNumber( /* Value */ 2)
			inner.Value = c.fromValue(&v.operand)
			c.p.PopFieldNumber()

			this.Kind = &proto.Value_Unary{
				Unary: &inner,
			}
		}
		c.p.PopFieldNumber()
	case astValueBinary:
		c.p.PushFieldNumber( /* Binary */ 21)
		{
			inner := proto.ValueBinary{}

			c.p.PushFieldNumber( /* Operation */ 1)
			inner.Operation = c.fromOperationBinary(&v.operator)
			c.p.PopFieldNumber()

			c.p.PushFieldNumber( /* Left */ 2)
			inner.Left = c.fromValue(&v.leftOperand)
			c.p.PopFieldNumber()

			c.p.PushFieldNumber( /* Right */ 3)
			inner.Right = c.fromValue(&v.rightOperand)
			c.p.PopFieldNumber()

			this.Kind = &proto.Value_Binary{
				Binary: &inner,
			}
		}
		c.p.PopFieldNumber()
	case astValueLiteralBool:
		c.p.PushFieldNumber( /* Bool */ 2)
		c.p.PushFieldNumber( /* Value */ 1)
		this.Kind = &proto.Value_Bool{
			Bool: &proto.ValueBool{
				Value: v.val,
				// Source:
			},
		}
		c.p.PopFieldNumber()
		c.p.PopFieldNumber()
	case astValueLiteralInt:
		c.p.PushFieldNumber( /* Int32 */ 7)
		c.p.PushFieldNumber( /* Value */ 1)
		this.Kind = &proto.Value_Int32{
			Int32: &proto.ValueInt32{
				Value:  (int32)(v.val),
				Source: v.token.Value,
			},
		}
		c.p.PopFieldNumber()
	case astValueLiteralFloat:
		c.p.PushFieldNumber( /* Float64 */ 14)
		c.p.PushFieldNumber( /* Value */ 1)
		this.Kind = &proto.Value_Float64{
			Float64: &proto.ValueFloat64{
				Value:  v.val,
				Source: v.token.Value,
			},
		}
		c.p.PopFieldNumber()
	case astValueLiteralText:
		c.p.PushFieldNumber( /* Text */ 3)
		c.p.PushFieldNumber( /* Value */ 1)
		this.Kind = &proto.Value_Text{
			Text: &proto.ValueText{
				Value:  v.val.Value,
				Source: v.val.Value,
			},
		}
		c.p.PopFieldNumber()
		c.p.PopFieldNumber()
	case astValueLiteralData:
		c.p.PushFieldNumber( /* Data */ 4)
		c.p.PushFieldNumber( /* Value */ 1)
		this.Kind = &proto.Value_Data{
			Data: &proto.ValueData{
				Value:  []byte(v.val.Value),
				Source: v.val.Value,
			},
		}
		c.p.PopFieldNumber()
		c.p.PopFieldNumber()
	case astValueLiteralList:
		c.p.PushFieldNumber( /* List */ 15)
		c.p.PushFieldNumber( /* Elements */ 1)
		c.p.PushIndex()
		this.Kind = &proto.Value_List{
			List: &proto.ValueList{
				Elements: mapFrom(c, v.vals, c.fromValue),
			},
		}
		c.p.PopIndex()
		c.p.PopFieldNumber()
		c.p.PopFieldNumber()
	case astValueLiteralStruct:
		c.p.PushFieldNumber( /* Struct */ 17)
		c.p.PushFieldNumber( /* Fields */ 1)
		c.p.PushIndex()
		this.Kind = &proto.Value_Struct{
			Struct: &proto.ValueStruct{
				Fields: mapFrom(c, v.vals, c.fromLiteralStructPair),
			},
		}
		c.p.PopIndex()
		c.p.PopFieldNumber()
		c.p.PopFieldNumber()
	case astValueIdentifier:
		c.p.PushFieldNumber( /* Identifier */ 19)
		this.Kind = &proto.Value_Identifier{
			Identifier: c.fromValueIdentifier(&v),
		}
		c.p.PopFieldNumber()
	default:
		return nil
	}

	return &this
}

func (c *astConverter) fromLiteralStructPair(literalStructPair *astLiteralStructPair) *proto.ValueStructField {
	return &proto.ValueStructField{
		Name:  literalStructPair.identifier.Value,
		Value: c.fromValue(&literalStructPair.value),
	}
}

func (c *astConverter) fromValueIdentifier(valueIdentifier *astValueIdentifier) *proto.ValueIdentifier {
	this := proto.ValueIdentifier{}

	c.p.PushFieldNumber( /* Names */ 2)
	c.p.PushIndex()
	this.Names = mapFrom(c, valueIdentifier.components, func(c *idl.Token) string { return c.Value })
	c.p.PopIndex()
	c.p.PopFieldNumber()

	return &this
}

func (c *astConverter) fromTypeName(typeName *astTypeName) *proto.TypeName {
	this := proto.TypeName{
		Name: typeName.identifier.Value,
	}

	c.p.PushFieldNumber( /* Parameters */ 2)
	c.p.PushIndex()
	this.Parameters = mapFrom(c, typeName.parameters, c.fromTypeSpecifier)
	c.p.PopIndex()
	c.p.PopFieldNumber()

	return &this
}

// n.b. this is *intentionally* not a method on astConverter, because it needs to be side-effect free!
func fromTypeUID(typeUID *astValueLiteralInt) *proto.TypeReference {
	this := proto.TypeReference{
		ModuleUID: idl.Incomplete,
	}
	if typeUID != nil {
		this.TypeUID = typeUID.val
	} else {
		this.TypeUID = idl.Incomplete
	}
	return &this
}

// n.b. this is *intentionally* not a method on astConverter, because it needs to be side-effect free!
func fromAttributeUID(attributeUID *astValueLiteralInt) *proto.AttributeReference {
	this := proto.AttributeReference{
		ModuleUID: idl.Incomplete,
		TypeUID:   idl.Incomplete,
	}
	if attributeUID != nil {
		this.AttributeUID = attributeUID.val
	} else {
		this.AttributeUID = idl.Incomplete
	}
	return &this
}

// n.b. this is *intentionally* not a method on astConverter, because it needs to be side-effect free!
func fromInputUID(inputUID *astValueLiteralInt) *proto.SDKInputReference {
	this := proto.SDKInputReference{
		ModuleUID:    idl.Incomplete,
		TypeUID:      idl.Incomplete,
		AttributeUID: idl.Incomplete,
	}
	if inputUID != nil {
		this.InputUID = inputUID.val
	} else {
		this.InputUID = idl.Incomplete
	}
	return &this
}
