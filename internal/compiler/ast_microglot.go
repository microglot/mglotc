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

// interface for all value types
type value interface {
	node
	value()
}

// interface for all invocation types
type invocation interface {
	node
	invocation()
}

// superset of value and invocation types
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

type astModule struct {
	comments   *astCommentBlock
	syntax     astStatementSyntax
	statements []statement
}

func (self astModule) String() string {
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
	value         astValue
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
	value         astValue
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
	value               astValue
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
	operand  astValue
}

type astValueBinary struct {
	leftOperand  astValue
	operator     idl.Token
	rightOperand astValue
}

type astValueLiteralBool struct {
	val bool
}

type astValueLiteralInt struct {
	token idl.Token
	val   uint64
}

type astValueLiteralFloat struct {
	token idl.Token
	val   float64
}

type astValueLiteralText struct {
	val idl.Token
}

type astValueLiteralData struct {
	val idl.Token
}

type astValueLiteralList struct {
	vals []astValue
}

type astValueLiteralStruct struct {
	vals []astLiteralStructPair
}

type astLiteralStructPair struct {
	identifier idl.Token
	value      astValue
}

type astQualifiedIdentifier struct {
	components []idl.Token
}

type astValueIdentifier astQualifiedIdentifier

type astValue struct {
	value
}

type astImplIdentifier astQualifiedIdentifier

type astInvocationCatch struct {
	identifier idl.Token
	block      astImplBlock
}

type astInvocationAwait struct {
	identifier idl.Token
	catch      *astInvocationCatch
}

type astInvocationAsync struct {
	implIdentifier astImplIdentifier
	parameters     []astValue
}

type astInvocationDirect struct {
	implIdentifier astImplIdentifier
	parameters     []astValue
	catch          *astInvocationCatch
}

type astInvocation struct {
	invocation
}

type astStepProse struct {
	prose idl.Token
}

type astStepVar struct {
	identifier idl.Token
	value      valueorinvocation
}

type astStepSet struct {
	identifier astQualifiedIdentifier
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
	values []astValue
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
	value     astValue
	block     astImplBlock
}

type astStepReturn struct {
	value *astValue
}

type astStepThrow struct {
	value astValue
}

type astStepExec struct {
	invocation astInvocation
}

type astCommentedBlock[N node, P node] struct {
	innerComments *astCommentBlock
	prefix        *P
	values        []N
}

func (astModule) node()              {}
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
func (astInvocationAsync) node()     {}
func (astInvocationDirect) node()    {}
func (astInvocationAwait) node()     {}
func (astInvocation) node()          {}

func (astStatementModuleMeta) statement() {}
func (astStatementImport) statement()     {}
func (astStatementAnnotation) statement() {}
func (astStatementConst) statement()      {}
func (astStatementEnum) statement()       {}
func (astStatementStruct) statement()     {}
func (astStatementAPI) statement()        {}
func (astStatementSDK) statement()        {}
func (astStatementImpl) statement()       {}

func (astValueUnary) value()         {}
func (astValueBinary) value()        {}
func (astValueLiteralBool) value()   {}
func (astValueLiteralInt) value()    {}
func (astValueLiteralFloat) value()  {}
func (astValueLiteralText) value()   {}
func (astValueLiteralData) value()   {}
func (astValueLiteralList) value()   {}
func (astValueLiteralStruct) value() {}
func (astValueIdentifier) value()    {}

func (astInvocationAsync) invocation()  {}
func (astInvocationDirect) invocation() {}
func (astInvocationAwait) invocation()  {}

func (astValue) valueorinvocation()      {}
func (astInvocation) valueorinvocation() {}

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
