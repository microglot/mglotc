package compiler

import (
	"fmt"

	"gopkg.microglot.org/compiler.go/internal/idl"
)

// interface for all AST nodes
type node interface {
	node()
}

// interface for all statement types
type statement interface {
	node
	statement()
}

// interface for all expression types
type expression interface {
	node
	expression()
}

// interface for "valueorinvocation"
type valueorinvocation interface {
	node
	valueorinvocation()
}

// interface for switch element types
type switchelement interface {
	node
	switchelement()
}

// interface for all struct element types
type structelement interface {
	node
	structelement()
}

// interface for all implementation method types
type implmethod interface {
	node
	implmethod()
}

// interface for all step types
type step interface {
	node
	step()
}

type ast struct {
	comments   *astCommentBlock
	syntax     astStatementSyntax
	statements []statement
}

func (self ast) String() string {
	// TODO 2023.08.16: still hideous!
	formatted := ""
	formatted += fmt.Sprintf("%v\n", self.comments)
	for _, s := range self.statements {
		formatted += fmt.Sprintf("%v\n", s)
	}
	return formatted
}

type astStatementSyntax struct {
	syntax astValueLiteralText
}

type meta struct {
	uid                   *astValueLiteralInt
	annotationApplication *astAnnotationApplication
	comments              *astCommentBlock
}

type astStatementModuleMeta struct {
	uid                   astValueLiteralInt
	annotationApplication *astAnnotationApplication
	comments              *astCommentBlock
}

type astStatementImport struct {
	uri      astValueLiteralText
	name     idl.Token
	comments *astCommentBlock
}

type astStatementAnnotation struct {
	identifier       idl.Token
	annotationScopes []astAnnotationScope
	typeSpecifier    astTypeSpecifier
	uid              *astValueLiteralInt
	comments         *astCommentBlock
}

type astStatementConst struct {
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
	value         expression
	meta          astMetadata
}

type astStatementEnum struct {
	identifier    idl.Token
	innerComments *astCommentBlock
	enumerants    []astEnumerant
	meta          astMetadata
}

type astStatementStruct struct {
	typeName      astTypeName
	innerComments *astCommentBlock
	elements      []structelement
	meta          astMetadata
}

type astStatementAPI struct {
	typeName      astTypeName
	extends       *astExtension
	innerComments *astCommentBlock
	methods       []astAPIMethod
	meta          astMetadata
}

type astStatementSDK struct {
	typeName      astTypeName
	extends       *astExtension
	innerComments *astCommentBlock
	methods       []astSDKMethod
	meta          astMetadata
}

type astStatementImpl struct {
	typeName      astTypeName
	as            astImplAs
	innerComments *astCommentBlock
	requires      *astImplRequires
	methods       []implmethod
	meta          astMetadata
}

type astImplBlock struct {
	innerComments *astCommentBlock
	steps         []step
}

type astImplSDKMethod struct {
	identifier    idl.Token
	methodInput   astSDKMethodInput
	methodReturns *astSDKMethodReturns
	nothrows      bool
	block         astImplBlock
	meta          astMetadata
}

type astImplAPIMethod struct {
	identifier    idl.Token
	methodInput   astAPIMethodInput
	methodReturns astAPIMethodReturns
	block         astImplBlock
	meta          astMetadata
}

type astImplRequirement struct {
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
	comments      *astCommentBlock
}

type astImplRequires struct {
	innerComments *astCommentBlock
	requirements  []astImplRequirement
}

type astImplAs struct {
	types []astTypeSpecifier
}

type astSDKMethodInput struct {
	parameters []astSDKMethodParameter
}

type astSDKMethodReturns struct {
	typeSpecifier astTypeSpecifier
}

type astSDKMethod struct {
	identifier    idl.Token
	methodInput   astSDKMethodInput
	methodReturns *astSDKMethodReturns
	nothrows      bool
	meta          astMetadata
}

type astSDKMethodParameter struct {
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
}

type astExtension struct {
	extensions []astTypeSpecifier
}

type astAPIMethodInput struct {
	typeSpecifier astTypeSpecifier
}

type astAPIMethodReturns struct {
	typeSpecifier astTypeSpecifier
}

type astAPIMethod struct {
	identifier    idl.Token
	methodInput   astAPIMethodInput
	methodReturns astAPIMethodReturns
	meta          astMetadata
}

type astUnion struct {
	identifier    *idl.Token
	innerComments *astCommentBlock
	fields        []astUnionField
	meta          astMetadata
}

type astUnionField struct {
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
	meta          astMetadata
}

type astField struct {
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
	value         expression
	meta          astMetadata
}

type astEnumerant struct {
	identifier idl.Token
	meta       astMetadata
}

type astMetadata struct {
	uid                   *astValueLiteralInt
	annotationApplication *astAnnotationApplication
	comments              *astCommentBlock
}

type astAnnotationApplication struct {
	annotationInstances []astAnnotationInstance
}

type astAnnotationInstance struct {
	namespaceIdentifier *idl.Token
	identifier          idl.Token
	value               expression
}

type astAnnotationScope struct {
	scope idl.Token
}

type astTypeSpecifier struct {
	qualifier *idl.Token
	typeName  astTypeName
}

type astTypeName struct {
	identifier idl.Token
	parameters []astTypeSpecifier
}

type astCommentBlock struct {
	comments []idl.Token
}

type astValueUnary struct {
	operator idl.Token
	operand  expression
}

type astValueBinary struct {
	leftOperand  expression
	operator     idl.Token
	rightOperand expression
}

type astValueLiteralBool struct {
	value bool
}

type astValueLiteralInt struct {
	token idl.Token
	value uint64
}

type astValueLiteralFloat struct {
	token idl.Token
	value float64
}

type astValueLiteralText struct {
	value idl.Token
}

type astValueLiteralData struct {
	value idl.Token
}

type astValueLiteralList struct {
	values []expression
}

type astValueLiteralStruct struct {
	values []astLiteralStructPair
}

type astLiteralStructPair struct {
	identifier astValueIdentifier
	value      expression
}

type astValueIdentifier struct {
	qualifiedIdentifier []idl.Token
}

type astInvocation struct {
}

type astStepProse struct {
	prose idl.Token
}

type astStepVar struct {
	identifier idl.Token
	value      valueorinvocation
}

type astStepSet struct {
	identifier astValueIdentifier
	value      valueorinvocation
}

type astConditionBlock struct {
	condition astValueBinary
	block     astImplBlock
}

type astStepIf struct {
	conditions []astConditionBlock
	elseBlock  *astImplBlock
}

type astSwitchCase struct {
	values []expression
	block  astImplBlock
}

type astSwitchDefault struct {
	block astImplBlock
}

type astStepSwitch struct {
	innerComments *astCommentBlock
	cases         []switchelement
}

type astStepWhile struct {
	conditionBlock astConditionBlock
}

type astStepFor struct {
	keyName   idl.Token
	valueName idl.Token
	value     expression
	block     astImplBlock
}

type astStepReturn struct {
	value *expression
}

type astStepThrow struct {
	value expression
}

type astStepExec struct {
	invocation astInvocation
}

type astCommentedBlock[N node, P node] struct {
	innerComments *astCommentBlock
	prefix        *P
	values        []N
}

func (ast) node()                    {}
func (astStatementSyntax) node()     {}
func (astStatementModuleMeta) node() {}
func (astStatementImport) node()     {}
func (astStatementAnnotation) node() {}
func (astStatementConst) node()      {}
func (astStatementEnum) node()       {}
func (astStatementStruct) node()     {}
func (astStatementAPI) node()        {}
func (astStatementSDK) node()        {}
func (astStatementImpl) node()       {}
func (astImplBlock) node()           {}
func (astImplSDKMethod) node()       {}
func (astImplAPIMethod) node()       {}
func (astImplRequirement) node()     {}
func (astImplRequires) node()        {}
func (astSDKMethod) node()           {}
func (astSDKMethodParameter) node()  {}
func (astAPIMethod) node()           {}
func (astEnumerant) node()           {}
func (astAnnotationInstance) node()  {}
func (astValueLiteralText) node()    {}
func (astCommentBlock) node()        {}
func (astValueUnary) node()          {}
func (astValueBinary) node()         {}
func (astValueLiteralBool) node()    {}
func (astValueLiteralInt) node()     {}
func (astValueLiteralFloat) node()   {}
func (astValueLiteralData) node()    {}
func (astValueLiteralList) node()    {}
func (astValueLiteralStruct) node()  {}
func (astValueIdentifier) node()     {}
func (astLiteralStructPair) node()   {}
func (astAnnotationScope) node()     {}
func (astTypeSpecifier) node()       {}
func (astTypeName) node()            {}
func (astUnion) node()               {}
func (astUnionField) node()          {}
func (astField) node()               {}
func (astStepProse) node()           {}
func (astStepVar) node()             {}
func (astStepSet) node()             {}
func (astStepIf) node()              {}
func (astSwitchCase) node()          {}
func (astSwitchDefault) node()       {}
func (astStepSwitch) node()          {}
func (astStepWhile) node()           {}
func (astStepFor) node()             {}
func (astStepReturn) node()          {}
func (astStepThrow) node()           {}
func (astStepExec) node()            {}

func (astStatementModuleMeta) statement() {}
func (astStatementImport) statement()     {}
func (astStatementAnnotation) statement() {}
func (astStatementConst) statement()      {}
func (astStatementEnum) statement()       {}
func (astStatementStruct) statement()     {}
func (astStatementAPI) statement()        {}
func (astStatementSDK) statement()        {}
func (astStatementImpl) statement()       {}

func (astValueUnary) expression()         {}
func (astValueBinary) expression()        {}
func (astValueLiteralBool) expression()   {}
func (astValueLiteralInt) expression()    {}
func (astValueLiteralFloat) expression()  {}
func (astValueLiteralText) expression()   {}
func (astValueLiteralData) expression()   {}
func (astValueLiteralList) expression()   {}
func (astValueLiteralStruct) expression() {}
func (astValueIdentifier) expression()    {}

func (astValueUnary) valueorinvocation()         {}
func (astValueBinary) valueorinvocation()        {}
func (astValueLiteralBool) valueorinvocation()   {}
func (astValueLiteralInt) valueorinvocation()    {}
func (astValueLiteralFloat) valueorinvocation()  {}
func (astValueLiteralText) valueorinvocation()   {}
func (astValueLiteralData) valueorinvocation()   {}
func (astValueLiteralList) valueorinvocation()   {}
func (astValueLiteralStruct) valueorinvocation() {}
func (astValueIdentifier) valueorinvocation()    {}

func (astUnion) structelement() {}
func (astField) structelement() {}

func (astImplSDKMethod) implmethod() {}
func (astImplAPIMethod) implmethod() {}

func (astStepProse) step()  {}
func (astStepVar) step()    {}
func (astStepSet) step()    {}
func (astStepIf) step()     {}
func (astStepSwitch) step() {}
func (astStepWhile) step()  {}
func (astStepFor) step()    {}
func (astStepReturn) step() {}
func (astStepThrow) step()  {}
func (astStepExec) step()   {}

func (astSwitchCase) switchelement()    {}
func (astSwitchDefault) switchelement() {}
