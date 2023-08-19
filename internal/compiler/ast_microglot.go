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
	statement()
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
	syntax astTextLit
}

type astStatementModuleMeta struct {
	uid                   astIntLit
	annotationApplication astAnnotationApplication
	comments              astCommentBlock
}

type astAnnotationApplication struct {
	annotationInstances []astAnnotationInstance
}

type astAnnotationInstance struct {
	namespace_identifier *idl.Token
	identifier           idl.Token
	value                astValue
}

type astTextLit struct {
	value idl.Token
}

type astIntLit struct {
	token idl.Token
	value uint64
}

type astCommentBlock struct {
	comments []idl.Token
}

type astValue struct {
	//TODO
}

type astValueUnary struct {
	//TODO
}

type astValueBinary struct {
	//TODO
}

type astValueIdentifier struct {
	//TODO
}

type astValueLiteral struct {
	//TODO
}

func (*ast) node()                    {}
func (*astStatementSyntax) node()     {}
func (*astStatementModuleMeta) node() {}
func (*astAnnotationInstance) node()  {}
func (*astTextLit) node()             {}
func (*astIntLit) node()              {}
func (*astCommentBlock) node()        {}
func (*astValue) node()               {}
func (*astValueUnary) node()          {}
func (*astValueBinary) node()         {}
func (*astValueIdentifier) node()     {}
func (*astValueLiteral) node()        {}

func (*astStatementModuleMeta) statement() {}
