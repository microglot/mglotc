package compiler

import (
	"errors"

	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

func fromModule(module *astModule) (*proto.Module, error) {
	this := proto.Module{}

	for _, statement := range module.statements {
		switch s := statement.(type) {
		case *astStatementModuleMeta:
			this.UID = s.uid.val
		case *astStatementImport:
			this.Imports = append(this.Imports, fromStatementImport(s))
		case *astStatementAnnotation:
			this.Annotations = append(this.Annotations, fromStatementAnnotation(s))
		case *astStatementConst:
			// this.Constants = append(this.Constants, fromStatementConst(s))
		case *astStatementEnum:
			// this.Enums = append(this.Enums, fromStatementEnum(s))
		case *astStatementStruct:
			// this.Structs = append(this.Structs, fromStatementStruct(s))
		case *astStatementAPI:
			// this.APIs = append(this.APIs, fromStatementAPI(s))
		case *astStatementSDK:
			// this.SDKs = append(this.SDKs, fromStatementSDK(s))
		case *astStatementImpl:
			// TODO 2023.09.05: missing from descriptor?
		default:
			return nil, errors.New("unknown statement type")
		}
	}
	return &this, nil
}

func fromStatementImport(statementImport *astStatementImport) *proto.Import {
	return &proto.Import{
		// TODO ModuleUID:
		ImportedURI: statementImport.uri.val.Value,
		// TODO ImportedUID:
		Alias: statementImport.name.Value,
		// TODO IsDot:
		CommentBlock: fromCommentBlock(statementImport.comments),
		// TODO AnnotationApplications:
	}
}

func fromStatementAnnotation(statementAnnotation *astStatementAnnotation) *proto.Annotation {
	return &proto.Annotation{
		// TODO Reference:
		Name:                   statementAnnotation.identifier.Value,
		Scopes:                 fromAnnotationScopes(statementAnnotation.annotationScopes),
		Type:                   fromTypeSpecifier(&statementAnnotation.typeSpecifier),
		DescriptorCommentBlock: fromCommentBlock(statementAnnotation.comments),
	}
}

func fromTypeSpecifier(typeSpecifier *astTypeSpecifier) *proto.TypeSpecifier {
	return &proto.TypeSpecifier{
		// TODO Reference:
		// TODO IsList:
		// TODO IsMap:
		// TODO HasPresence:
	}
}

func fromAnnotationScopes(annotationScopes []astAnnotationScope) []proto.AnnotationScope {
	if annotationScopes != nil {
		scopes := []proto.AnnotationScope{}

		for _, annotationScope := range annotationScopes {
			var t proto.AnnotationScope
			switch annotationScope.scope.Type {
			case idl.TokenTypeKeywordModule:
				t = proto.AnnotationScope_AnnotationScopeModule
			case idl.TokenTypeKeywordUnion:
				t = proto.AnnotationScope_AnnotationScopeUnion
			case idl.TokenTypeKeywordStruct:
				t = proto.AnnotationScope_AnnotationScopeStruct
			case idl.TokenTypeKeywordField:
				t = proto.AnnotationScope_AnnotationScopeField
			case idl.TokenTypeKeywordEnumerant:
				t = proto.AnnotationScope_AnnotationScopeEnumerant
			case idl.TokenTypeKeywordEnum:
				t = proto.AnnotationScope_AnnotationScopeEnum
			case idl.TokenTypeKeywordAPI:
				t = proto.AnnotationScope_AnnotationScopeAPI
			// TODO 2023.09.05: missing from the lexer and parser
			// case idl.TokenTypeKeywordAPIMethod:
			//   t = proto.AnnotationScope_AnnotationScopeAPIMethod
			case idl.TokenTypeKeywordSDK:
				t = proto.AnnotationScope_AnnotationScopeSDK
			// TODO 2023.09.05: missing from the lexer and parser
			// case idl.TokenTypeKeywordSDKMethod:
			//   t = proto.AnnotationScope_AnnotationScopeSDKMethod
			case idl.TokenTypeKeywordConst:
				t = proto.AnnotationScope_AnnotationScopeConst
			case idl.TokenTypeStar:
				t = proto.AnnotationScope_AnnotationScopeStar
			}

			scopes = append(scopes, t)
		}
		return scopes
	}
	return nil
}

func fromCommentBlock(commentBlock *astCommentBlock) *proto.CommentBlock {
	if commentBlock != nil {
		lines := []string{}

		for _, line := range commentBlock.comments {
			lines = append(lines, line.Value)
		}
		return &proto.CommentBlock{
			Lines: lines,
		}
	}
	return nil
}
