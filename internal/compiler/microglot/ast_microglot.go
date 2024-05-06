package microglot

import (
	"fmt"

	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
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

type astNode struct {
	loc proto.SourceLocation
}

type astModule struct {
	URI        string
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
	astNode
	syntax astValueLiteralText
}

type astStatementModuleMeta struct {
	astNode
	uid                   astValueLiteralInt
	annotationApplication *astAnnotationApplication
	comments              *astCommentBlock
}

type astStatementImport struct {
	astNode
	uri      astValueLiteralText
	name     idl.Token
	comments *astCommentBlock
}

type astStatementAnnotation struct {
	astNode
	identifier       idl.Token
	annotationScopes []astAnnotationScope
	typeSpecifier    astTypeSpecifier
	uid              *astValueLiteralInt
	comments         *astCommentBlock
}

type astStatementConst struct {
	astNode
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
	value         astValue
	meta          astMetadata
}

type astStatementEnum struct {
	astNode
	identifier    idl.Token
	innerComments *astCommentBlock
	enumerants    []astEnumerant
	meta          astMetadata
}

type astStatementStruct struct {
	astNode
	typeName      astTypeName
	innerComments *astCommentBlock
	elements      []structelement
	meta          astMetadata
}

type astStatementAPI struct {
	astNode
	typeName      astTypeName
	extends       *astExtension
	innerComments *astCommentBlock
	methods       []astAPIMethod
	meta          astMetadata
}

type astStatementSDK struct {
	astNode
	typeName      astTypeName
	extends       *astExtension
	innerComments *astCommentBlock
	methods       []astSDKMethod
	meta          astMetadata
}

type astStatementImpl struct {
	astNode
	typeName      astTypeName
	as            astImplAs
	innerComments *astCommentBlock
	requires      *astImplRequires
	methods       []implmethod
	meta          astMetadata
}

type astImplBlock struct {
	astNode
	innerComments *astCommentBlock
	steps         []step
}

type astImplSDKMethod struct {
	astNode
	identifier    idl.Token
	methodInput   astSDKMethodInput
	methodReturns *astSDKMethodReturns
	nothrows      bool
	block         astImplBlock
	meta          astMetadata
}

type astImplAPIMethod struct {
	astNode
	identifier    idl.Token
	methodInput   astAPIMethodInput
	methodReturns astAPIMethodReturns
	block         astImplBlock
	meta          astMetadata
}

type astImplRequirement struct {
	astNode
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
	comments      *astCommentBlock
}

type astImplRequires struct {
	astNode
	innerComments *astCommentBlock
	requirements  []astImplRequirement
}

type astImplAs struct {
	astNode
	types []astTypeSpecifier
}

type astSDKMethodInput struct {
	astNode
	parameters []astSDKMethodParameter
}

type astSDKMethodReturns struct {
	astNode
	typeSpecifier astTypeSpecifier
}

type astSDKMethod struct {
	astNode
	identifier    idl.Token
	methodInput   astSDKMethodInput
	methodReturns *astSDKMethodReturns
	nothrows      bool
	meta          astMetadata
}

type astSDKMethodParameter struct {
	astNode
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
}

type astExtension struct {
	astNode
	extensions []astTypeSpecifier
}

type astAPIMethodInput struct {
	astNode
	typeSpecifier astTypeSpecifier
}

type astAPIMethodReturns struct {
	astNode
	typeSpecifier astTypeSpecifier
}

type astAPIMethod struct {
	astNode
	identifier    idl.Token
	methodInput   astAPIMethodInput
	methodReturns astAPIMethodReturns
	meta          astMetadata
}

type astUnion struct {
	astNode
	identifier    *idl.Token
	innerComments *astCommentBlock
	fields        []astUnionField
	meta          astMetadata
}

type astUnionField struct {
	astNode
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
	meta          astMetadata
}

type astField struct {
	astNode
	identifier    idl.Token
	typeSpecifier astTypeSpecifier
	value         astValue
	meta          astMetadata
}

type astEnumerant struct {
	astNode
	identifier idl.Token
	meta       astMetadata
}

type astMetadata struct {
	uid                   *astValueLiteralInt
	annotationApplication *astAnnotationApplication
	comments              *astCommentBlock
}

type astAnnotationApplication struct {
	astNode
	annotationInstances []astAnnotationInstance
}

type astAnnotationInstance struct {
	astNode
	namespaceIdentifier *idl.Token
	identifier          idl.Token
	value               astValue
}

type astAnnotationScope struct {
	astNode
	scope idl.Token
}

type astTypeSpecifier struct {
	astNode
	qualifier *idl.Token
	typeName  astTypeName
}

type astTypeName struct {
	astNode
	identifier idl.Token
	parameters []astTypeSpecifier
}

type astCommentBlock struct {
	astNode
	comments []idl.Token
}

type astValueUnary struct {
	astNode
	operator idl.Token
	operand  astValue
}

type astValueBinary struct {
	astNode
	leftOperand  astValue
	operator     idl.Token
	rightOperand astValue
}

type astValueLiteralBool struct {
	astNode
	val bool
}

type astValueLiteralInt struct {
	astNode
	token idl.Token
	val   uint64
}

type astValueLiteralFloat struct {
	astNode
	token idl.Token
	val   float64
}

type astValueLiteralText struct {
	astNode
	val idl.Token
}

type astValueLiteralData struct {
	astNode
	val idl.Token
}

type astValueLiteralList struct {
	astNode
	vals []astValue
}

type astValueLiteralStruct struct {
	astNode
	vals []astLiteralStructPair
}

type astLiteralStructPair struct {
	astNode
	identifier idl.Token
	value      astValue
}

type astQualifiedIdentifier struct {
	astNode
	components []idl.Token
}

type astValueIdentifier astQualifiedIdentifier

type astValue struct {
	value
}

type astImplIdentifier astQualifiedIdentifier

type astInvocationCatch struct {
	astNode
	identifier idl.Token
	block      astImplBlock
}

type astInvocationAwait struct {
	astNode
	identifier idl.Token
	catch      *astInvocationCatch
}

type astInvocationAsync struct {
	astNode
	implIdentifier astImplIdentifier
	parameters     []astValue
}

type astInvocationDirect struct {
	astNode
	implIdentifier astImplIdentifier
	parameters     []astValue
	catch          *astInvocationCatch
}

type astInvocation struct {
	astNode
	invocation
}

type astStepProse struct {
	astNode
	prose idl.Token
}

type astStepVar struct {
	astNode
	identifier idl.Token
	value      valueorinvocation
}

type astStepSet struct {
	astNode
	identifier astQualifiedIdentifier
	value      valueorinvocation
}

type astConditionBlock struct {
	astNode
	condition astValueBinary
	block     astImplBlock
}

type astStepIf struct {
	astNode
	conditions []astConditionBlock
	elseBlock  *astImplBlock
}

type astSwitchCase struct {
	astNode
	values []astValue
	block  astImplBlock
}

type astSwitchDefault struct {
	astNode
	block astImplBlock
}

type astStepSwitch struct {
	astNode
	innerComments *astCommentBlock
	cases         []switchelement
}

type astStepWhile struct {
	astNode
	conditionBlock astConditionBlock
}

type astStepFor struct {
	astNode
	keyName   idl.Token
	valueName idl.Token
	value     astValue
	block     astImplBlock
}

type astStepReturn struct {
	astNode
	value *astValue
}

type astStepThrow struct {
	astNode
	value astValue
}

type astStepExec struct {
	astNode
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
