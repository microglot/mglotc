// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

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

func mapFrom[F any, T any](in []F, f func(*F) T) []T {
	if in != nil {
		out := make([]T, 0, len(in))

		for _, element := range in {
			out = append(out, f(&element))
		}
		return out
	}
	return nil
}

func FromModule(module *astModule) (*proto.Module, error) {
	this := proto.Module{
		URI: module.URI,
	}

	for _, statement := range module.statements {
		switch s := statement.(type) {
		case *astStatementModuleMeta:
			this.UID = s.uid.val
			this.AnnotationApplications = fromAnnotationApplication(s.annotationApplication)
		case *astStatementImport:
			this.Imports = append(this.Imports, fromStatementImport(s))
		case *astStatementAnnotation:
			this.Annotations = append(this.Annotations, fromStatementAnnotation(s))
		case *astStatementConst:
			this.Constants = append(this.Constants, fromStatementConst(s))
		case *astStatementEnum:
			this.Enums = append(this.Enums, fromStatementEnum(s))
		case *astStatementStruct:
			this.Structs = append(this.Structs, fromStatementStruct(s))
		case *astStatementAPI:
			this.APIs = append(this.APIs, fromStatementAPI(s))
		case *astStatementSDK:
			this.SDKs = append(this.SDKs, fromStatementSDK(s))
		case *astStatementImpl:
			// TODO 2023.09.05: missing from descriptor?
		default:
			return nil, errors.New("unknown statement type")
		}
	}

	if this.UID == 0 {
		return nil, fmt.Errorf("you must specify a UID for module %s", module.URI)
	}

	pkg, err := protobufPackage(this.AnnotationApplications, module.URI)
	if err != nil {
		return nil, err
	}
	this.ProtobufPackage = pkg

	return &this, nil
}

func protobufPackage(annotationApplications []*proto.AnnotationApplication, moduleURI string) (string, error) {
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

func fromStatementImport(statementImport *astStatementImport) *proto.Import {
	return &proto.Import{
		// ModuleUID:
		// ImportedUID:
		IsDot:        statementImport.name.Value == ".",
		ImportedURI:  statementImport.uri.val.Value,
		Alias:        statementImport.name.Value,
		CommentBlock: fromCommentBlock(statementImport.comments),
	}
}

func fromStatementAnnotation(statementAnnotation *astStatementAnnotation) *proto.Annotation {
	return &proto.Annotation{
		Reference:              fromTypeUID(statementAnnotation.uid),
		Name:                   statementAnnotation.identifier.Value,
		Scopes:                 fromAnnotationScopes(statementAnnotation.annotationScopes),
		Type:                   fromTypeSpecifier(&statementAnnotation.typeSpecifier),
		DescriptorCommentBlock: fromCommentBlock(statementAnnotation.comments),
	}
}

func fromStatementConst(statementConst *astStatementConst) *proto.Constant {
	x := proto.Constant{
		Reference:              fromTypeUID(statementConst.meta.uid),
		Name:                   statementConst.identifier.Value,
		Type:                   fromTypeSpecifier(&statementConst.typeSpecifier),
		Value:                  fromValue(&statementConst.value),
		AnnotationApplications: fromAnnotationApplication(statementConst.meta.annotationApplication),
		CommentBlock:           fromCommentBlock(statementConst.meta.comments),
	}
	return &x
}

func fromStatementEnum(statementEnum *astStatementEnum) *proto.Enum {
	result := &proto.Enum{
		Reference:  fromTypeUID(statementEnum.meta.uid),
		Name:       statementEnum.identifier.Value,
		Enumerants: mapFrom(statementEnum.enumerants, fromEnumerant),
		// Reserved:
		// ReservedNames:
		CommentBlock:           fromCommentBlock(statementEnum.meta.comments),
		AnnotationApplications: fromAnnotationApplication(statementEnum.meta.annotationApplication),
	}
	var foundZero bool
	for _, en := range result.Enumerants {
		if en.Reference.AttributeUID == 0 {
			foundZero = true
			break
		}
	}
	if !foundZero {
		result.Enumerants = append(result.Enumerants, &proto.Enumerant{
			Reference: fromAttributeUID(&astValueLiteralInt{val: 0}),
			Name:      "None",
		})
	}
	sort.Slice(result.Enumerants, func(i, j int) bool {
		return result.Enumerants[i].Reference.AttributeUID < result.Enumerants[j].Reference.AttributeUID
	})
	return result
}

func fromStatementStruct(statementStruct *astStatementStruct) *proto.Struct {
	this := proto.Struct{
		Reference: fromTypeUID(statementStruct.meta.uid),
		Name:      fromTypeName(&statementStruct.typeName),
		Fields:    nil,
		Unions:    nil,
		// Reserved:
		CommentBlock:           fromCommentBlock(statementStruct.meta.comments),
		AnnotationApplications: fromAnnotationApplication(statementStruct.meta.annotationApplication),
		// IsSynthetic:
	}

	for _, element := range statementStruct.elements {
		switch e := element.(type) {
		case *astField:
			this.Fields = append(this.Fields, fromField(e))
		case *astUnion:
			this.Unions = append(this.Unions, fromUnion(e))
			for _, unionField := range e.fields {
				this.Fields = append(this.Fields, fromUnionField(&unionField, uint64(len(this.Unions)-1)))
			}
		}
	}

	return &this
}

func fromStatementAPI(statementAPI *astStatementAPI) *proto.API {
	var extends []*proto.TypeSpecifier
	if statementAPI.extends != nil {
		extends = mapFrom(statementAPI.extends.extensions, fromTypeSpecifier)
	}

	return &proto.API{
		Reference: fromTypeUID(statementAPI.meta.uid),
		Name:      fromTypeName(&statementAPI.typeName),
		Methods:   mapFrom(statementAPI.methods, fromAPIMethod),
		Extends:   extends,
		// Reserved:
		// ReservedNames:
		CommentBlock:           fromCommentBlock(statementAPI.meta.comments),
		AnnotationApplications: fromAnnotationApplication(statementAPI.meta.annotationApplication),
	}
}

func fromStatementSDK(statementSDK *astStatementSDK) *proto.SDK {
	var extends []*proto.TypeSpecifier
	if statementSDK.extends != nil {
		extends = mapFrom(statementSDK.extends.extensions, fromTypeSpecifier)
	}

	return &proto.SDK{
		Reference: fromTypeUID(statementSDK.meta.uid),
		Name:      fromTypeName(&statementSDK.typeName),
		Methods:   mapFrom(statementSDK.methods, fromSDKMethod),
		Extends:   extends,
		// Reserved:
		// ReservedNames:
		CommentBlock:           fromCommentBlock(statementSDK.meta.comments),
		AnnotationApplications: fromAnnotationApplication(statementSDK.meta.annotationApplication),
	}
}

func fromAPIMethod(apiMethod *astAPIMethod) *proto.APIMethod {
	return &proto.APIMethod{
		Reference:              fromAttributeUID(apiMethod.meta.uid),
		Name:                   apiMethod.identifier.Value,
		Input:                  fromTypeSpecifier(&apiMethod.methodInput.typeSpecifier),
		Output:                 fromTypeSpecifier(&apiMethod.methodReturns.typeSpecifier),
		CommentBlock:           fromCommentBlock(apiMethod.meta.comments),
		AnnotationApplications: fromAnnotationApplication(apiMethod.meta.annotationApplication),
	}
}

func fromSDKMethod(sdkMethod *astSDKMethod) *proto.SDKMethod {
	var output *proto.TypeSpecifier
	if sdkMethod.methodReturns != nil {
		output = fromTypeSpecifier(&sdkMethod.methodReturns.typeSpecifier)
	}

	return &proto.SDKMethod{
		Reference:              fromAttributeUID(sdkMethod.meta.uid),
		Name:                   sdkMethod.identifier.Value,
		Input:                  mapFrom(sdkMethod.methodInput.parameters, fromSDKMethodParameter),
		Output:                 output,
		NoThrows:               sdkMethod.nothrows,
		CommentBlock:           fromCommentBlock(sdkMethod.meta.comments),
		AnnotationApplications: fromAnnotationApplication(sdkMethod.meta.annotationApplication),
	}
}

func fromSDKMethodParameter(sdkMethodParameter *astSDKMethodParameter) *proto.SDKMethodInput {
	return &proto.SDKMethodInput{
		// TODO 2023.10.29: the ebnf and ast don't actually allow setting the InputUID,
		// which is different from every other kind of UID. Is this intentional?
		Reference: fromInputUID(nil),
		Name:      sdkMethodParameter.identifier.Value,
		Type:      fromTypeSpecifier(&sdkMethodParameter.typeSpecifier),
	}
}

func fromField(field *astField) *proto.Field {
	return &proto.Field{
		Reference:              fromAttributeUID(field.meta.uid),
		Name:                   field.identifier.Value,
		Type:                   fromTypeSpecifier(&field.typeSpecifier),
		DefaultValue:           fromValue(&field.value),
		UnionIndex:             nil,
		CommentBlock:           fromCommentBlock(field.meta.comments),
		AnnotationApplications: fromAnnotationApplication(field.meta.annotationApplication),
	}
}

func fromUnion(union *astUnion) *proto.Union {
	return &proto.Union{
		Reference:              fromAttributeUID(union.meta.uid),
		Name:                   union.identifier.Value,
		CommentBlock:           fromCommentBlock(union.meta.comments),
		AnnotationApplications: fromAnnotationApplication(union.meta.annotationApplication),
	}
}

func fromUnionField(unionField *astUnionField, unionIndex uint64) *proto.Field {
	return &proto.Field{
		Reference:              fromAttributeUID(unionField.meta.uid),
		Name:                   unionField.identifier.Value,
		Type:                   fromTypeSpecifier(&unionField.typeSpecifier),
		DefaultValue:           nil,
		UnionIndex:             &unionIndex,
		CommentBlock:           fromCommentBlock(unionField.meta.comments),
		AnnotationApplications: fromAnnotationApplication(unionField.meta.annotationApplication),
	}
}

func fromEnumerant(enumerant *astEnumerant) *proto.Enumerant {
	return &proto.Enumerant{
		Reference:              fromAttributeUID(enumerant.meta.uid),
		Name:                   enumerant.identifier.Value,
		CommentBlock:           fromCommentBlock(enumerant.meta.comments),
		AnnotationApplications: fromAnnotationApplication(enumerant.meta.annotationApplication),
	}
}

func fromTypeSpecifier(typeSpecifier *astTypeSpecifier) *proto.TypeSpecifier {
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
						Name:      fromTypeName(&typeSpecifier.typeName),
					},
				},
			},
		},
	}
}

func fromAnnotationScope(annotationScope *astAnnotationScope) proto.AnnotationScope {
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

func fromAnnotationScopes(annotationScopes []astAnnotationScope) []proto.AnnotationScope {
	return mapFrom(annotationScopes, fromAnnotationScope)
}

func fromAnnotationInstance(annotationInstance *astAnnotationInstance) *proto.AnnotationApplication {
	return &proto.AnnotationApplication{
		// This is admittedly weird, but the pseudo-type specifiers in annotation applications
		// are grammatically slightly different from a full-blown type specifier.
		Annotation: fromTypeSpecifier(&astTypeSpecifier{
			qualifier: annotationInstance.namespaceIdentifier,
			typeName: astTypeName{
				identifier: annotationInstance.identifier,
			},
		}),
		Value: fromValue(&annotationInstance.value),
	}
}

func fromCommentBlock(commentBlock *astCommentBlock) *proto.CommentBlock {
	if commentBlock != nil {
		return &proto.CommentBlock{
			Lines: mapFrom(commentBlock.comments, func(line *idl.Token) string { return line.Value }),
		}
	}
	return nil
}

func fromAnnotationApplication(annotationApplication *astAnnotationApplication) []*proto.AnnotationApplication {
	if annotationApplication != nil {
		return mapFrom(annotationApplication.annotationInstances, fromAnnotationInstance)
	}
	return nil
}

func fromOperationUnary(t *idl.Token) proto.OperationUnary {
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

func fromOperationBinary(t *idl.Token) proto.OperationBinary {
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

func fromValue(value *astValue) *proto.Value {
	this := proto.Value{}

	switch v := value.value.(type) {
	case astValueUnary:
		this.Kind = &proto.Value_Unary{
			Unary: &proto.ValueUnary{
				Operation: fromOperationUnary(&v.operator),
				Value:     fromValue(&v.operand),
			},
		}
	case astValueBinary:
		this.Kind = &proto.Value_Binary{
			Binary: &proto.ValueBinary{
				Operation: fromOperationBinary(&v.operator),
				Left:      fromValue(&v.leftOperand),
				Right:     fromValue(&v.rightOperand),
			},
		}
	case astValueLiteralBool:
		this.Kind = &proto.Value_Bool{
			Bool: &proto.ValueBool{
				Value: v.val,
				// Source:
			},
		}
	case astValueLiteralInt:
		this.Kind = &proto.Value_Int32{
			Int32: &proto.ValueInt32{
				Value:  (int32)(v.val),
				Source: v.token.Value,
			},
		}
	case astValueLiteralFloat:
		this.Kind = &proto.Value_Float64{
			Float64: &proto.ValueFloat64{
				Value:  v.val,
				Source: v.token.Value,
			},
		}
	case astValueLiteralText:
		this.Kind = &proto.Value_Text{
			Text: &proto.ValueText{
				Value:  v.val.Value,
				Source: v.val.Value,
			},
		}
	case astValueLiteralData:
		this.Kind = &proto.Value_Data{
			Data: &proto.ValueData{
				Value:  []byte(v.val.Value),
				Source: v.val.Value,
			},
		}
	case astValueLiteralList:
		this.Kind = &proto.Value_List{
			List: &proto.ValueList{
				Elements: mapFrom(v.vals, fromValue),
			},
		}
	case astValueLiteralStruct:
		this.Kind = &proto.Value_Struct{
			Struct: &proto.ValueStruct{
				Fields: mapFrom(v.vals, fromLiteralStructPair),
			},
		}
	case astValueIdentifier:
		this.Kind = &proto.Value_Identifier{
			Identifier: fromValueIdentifier(&v),
		}
	default:
		return nil
	}

	return &this
}

func fromLiteralStructPair(literalStructPair *astLiteralStructPair) *proto.ValueStructField {
	return &proto.ValueStructField{
		Name:  literalStructPair.identifier.Value,
		Value: fromValue(&literalStructPair.value),
	}
}

func fromValueIdentifier(valueIdentifier *astValueIdentifier) *proto.ValueIdentifier {
	return &proto.ValueIdentifier{
		Names: mapFrom(valueIdentifier.components, func(c *idl.Token) string { return c.Value }),
	}
}

func fromTypeName(typeName *astTypeName) *proto.TypeName {
	return &proto.TypeName{
		Name:       typeName.identifier.Value,
		Parameters: mapFrom(typeName.parameters, fromTypeSpecifier),
	}
}

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
