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

type ast struct {
	comments   astCommentBlock
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

type astStatementModuleMeta struct {
	uid                   astValueLiteralInt
	annotationApplication astAnnotationApplication
	comments              astCommentBlock
}

type astStatementImport struct {
	uri      astValueLiteralText
	name     idl.Token
	comments astCommentBlock
}

type astStatementAnnotation struct {
	identifier       idl.Token
	annotationScopes []astAnnotationScope
	typeSpecifier    astTypeSpecifier
	uid              *astValueLiteralInt
	comments         astCommentBlock
}

type astAnnotationApplication struct {
	annotationInstances []*astAnnotationInstance
}

type astAnnotationInstance struct {
	namespaceIdentifier *idl.Token
	identifier          idl.Token
	value               expression
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
	values []*astLiteralStructPair
}

type astLiteralStructPair struct {
	identifier astValueIdentifier
	value      expression
}

type astValueIdentifier struct {
	qualifiedIdentifier []idl.Token
}

func (*ast) node()                    {}
func (*astStatementSyntax) node()     {}
func (*astStatementModuleMeta) node() {}
func (*astStatementImport) node()     {}
func (*astStatementAnnotation) node() {}
func (*astAnnotationInstance) node()  {}
func (*astValueLiteralText) node()    {}
func (*astCommentBlock) node()        {}
func (*astValueUnary) node()          {}
func (*astValueBinary) node()         {}
func (*astValueLiteralBool) node()    {}
func (*astValueLiteralInt) node()     {}
func (*astValueLiteralFloat) node()   {}
func (*astValueLiteralData) node()    {}
func (*astValueLiteralList) node()    {}
func (*astValueLiteralStruct) node()  {}
func (*astValueIdentifier) node()     {}
func (*astLiteralStructPair) node()   {}

func (*astStatementModuleMeta) statement() {}
func (*astStatementImport) statement()     {}
func (*astStatementAnnotation) statement() {}

func (*astValueUnary) expression()         {}
func (*astValueBinary) expression()        {}
func (*astValueLiteralBool) expression()   {}
func (*astValueLiteralInt) expression()    {}
func (*astValueLiteralFloat) expression()  {}
func (*astValueLiteralText) expression()   {}
func (*astValueLiteralData) expression()   {}
func (*astValueLiteralList) expression()   {}
func (*astValueLiteralStruct) expression() {}
func (*astValueIdentifier) expression()    {}
