package compiler

import (
	"fmt"
)

// interface for all statement types
type statement interface {
	statement()
}

type ast struct {
	comments   astCommentBlock
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

func (*astStatementSyntax) statement()     {}
func (*astStatementModuleMeta) statement() {}

type astAnnotationApplication struct {
	// TODO 2023.08.16: incomplete
}

type astTextLit struct {
	value string
}

type astIntLit struct {
	value int64
}

type astCommentBlock struct {
	comments []astComment
}

type astComment struct {
	value string
}
