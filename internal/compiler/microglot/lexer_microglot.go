// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package microglot

import (
	"context"
	"strings"
	"unicode"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/iter"
	"gopkg.microglot.org/compiler.go/internal/optional"
)

const (
	lexerMicroglotLookahead = 8
)

// LexerMicroglot implements a tokenizer for the Microglot IDL syntax.
type LexerMicroglot struct {
	reporter exc.Reporter
}

func NewLexerMicroglot(reporter exc.Reporter) *LexerMicroglot {
	return &LexerMicroglot{reporter: reporter}
}

func (self *LexerMicroglot) Lex(ctx context.Context, f idl.File) (idl.LexerFile, error) {
	return &lexerFileMicroglot{
		File:     f,
		reporter: self.reporter,
	}, nil
}

type lexerFileMicroglot struct {
	idl.File
	reporter exc.Reporter
}

func (self *lexerFileMicroglot) Tokens(ctx context.Context) (idl.Iterator[*idl.Token], error) {
	b, err := self.File.Body(ctx)
	if err != nil {
		return nil, err
	}
	points := iter.NewLookahead(iter.NewUnicodeFileBodyCtx(ctx, b), lexerMicroglotLookahead)
	return &lexerFileMicroglotTokens{
		uri:      self.File.Path(ctx),
		body:     points,
		reporter: self.reporter,
		line:     1,
		col:      0,
		offset:   -1,
	}, nil
}

type lexerFileMicroglotTokens struct {
	uri      string
	body     idl.Lookahead[idl.CodePoint]
	reporter exc.Reporter
	line     int32
	col      int32
	offset   int64
	hasBOM   bool
}

func (self *lexerFileMicroglotTokens) Next(ctx context.Context) optional.Optional[*idl.Token] {
	for point := self.next(ctx); point.IsPresent(); point = self.next(ctx) {
		r := rune(point.Value())
		if r == 0xEFBBBF {
			if self.offset != 0 {
				e := self.exc(exc.CodeUnsupportedFileFormat, "invalid UTF-8 BOM location")
				_ = self.reporter.Report(e)
				return optional.None[*idl.Token]()
			}
			if self.hasBOM {
				e := self.exc(exc.CodeUnsupportedFileFormat, "duplicate UTF-8 BOM location")
				_ = self.reporter.Report(e)
				return optional.None[*idl.Token]()
			}
			self.hasBOM = true
			self.offset = -1
			self.col = 0
			continue
		}
		switch r {
		case 0x00:
			return optional.None[*idl.Token]() // Treat null byte as EOF as it's not allowed.
		case 0x0009, 0x0020:
			continue // Generally ignore space and tab.
		case '\n':
			return self.newLineToken("\n", 1)
		case '\r':
			if n := self.body.Lookahead(ctx, 1); n.IsPresent() && n.Value() == '\n' {
				_ = self.next(ctx)
				return self.newLineToken("\r\n", 2)
			}
			return self.newLineToken("\r", 1)
		case '0':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeIntegerDecimal, string(r))
				return optional.Some(t)
			}
			switch n.Value() {
			case 'x', 'X':
				_ = self.next(ctx)
				prefix := string(r) + string(rune(n.Value()))
				nn := self.body.Lookahead(ctx, 1)
				if !nn.IsPresent() {
					_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading hex or data literal"))
					return optional.None[*idl.Token]()
				}
				switch nn.Value() {
				case '"':
					_ = self.next(ctx)
					return self.readData(ctx)
				default:
					return self.readNumber(ctx, prefix)
				}
			case 'b', 'B':
				_ = self.next(ctx)
				prefix := string(r) + string(rune(n.Value()))
				return self.readNumber(ctx, prefix)
			case 'o', 'O':
				_ = self.next(ctx)
				prefix := string(r) + string(rune(n.Value()))
				return self.readNumber(ctx, prefix)
			default:
				return self.readNumber(ctx, string(r))
			}
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return self.readDecimal(ctx, string(r))
		case '.':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeDot, ".")
				return optional.Some(t)
			}
			switch n.Value() {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				return self.readNumber(ctx, ".")
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeDot, ".")
				return optional.Some(t)
			}
		case '"':
			return self.readText(ctx)
		case '/':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeSlash, "/")
				return optional.Some(t)
			}
			switch n.Value() {
			case '/':
				_ = self.next(ctx)
				return self.readCommentLine(ctx)
			case '*':
				_ = self.next(ctx)
				return self.readCommentBlock(ctx)
			case '=':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeDivideEqual, "/=")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeSlash, "/")
				return optional.Some(t)
			}
		case '`':
			return self.readProse(ctx)
		case '{':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeCurlyOpen, "{")
			return optional.Some(t)
		case '}':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeCurlyClose, "}")
			return optional.Some(t)
		case '[':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeSquareOpen, "[")
			return optional.Some(t)
		case ']':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeSquareClose, "]")
			return optional.Some(t)
		case '(':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeParenOpen, "(")
			return optional.Some(t)
		case ')':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeParenClose, ")")
			return optional.Some(t)
		case '<':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeAngleOpen, "<")
				return optional.Some(t)
			}
			switch n.Value() {
			case '=':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeLesserEqual, "<=")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeAngleOpen, "<")
				return optional.Some(t)
			}
		case '>':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeAngleClose, ">")
				return optional.Some(t)
			}
			switch n.Value() {
			case '=':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeGreaterEqual, ">=")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeAngleClose, ">")
				return optional.Some(t)
			}
		case '+':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypePlus, "+")
				return optional.Some(t)
			}
			switch n.Value() {
			case '=':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypePlusEqual, "+=")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypePlus, "+")
				return optional.Some(t)
			}
		case '-':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeMinus, "-")
				return optional.Some(t)
			}
			switch n.Value() {
			case '=':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeMinusEqual, "-=")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeMinus, "-")
				return optional.Some(t)
			}
		case '*':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeStar, "*")
				return optional.Some(t)
			}
			switch n.Value() {
			case '=':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeMultiplyEqual, "*=")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeStar, "*")
				return optional.Some(t)
			}
		case ',':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeComma, ",")
			return optional.Some(t)
		case ':':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeColon, ":")
			return optional.Some(t)
		case '@':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeAt, "@")
			return optional.Some(t)
		case '$':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeDollar, "$")
			return optional.Some(t)
		case '~':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeTilde, "~")
			return optional.Some(t)
		case '!':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeExclamation, "!")
				return optional.Some(t)
			}
			switch n.Value() {
			case '=':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeNotComparison, "!=")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeExclamation, "!")
				return optional.Some(t)
			}
		case '%':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypePercent, "%")
			return optional.Some(t)
		case '^':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeCaret, "^")
			return optional.Some(t)
		case '&':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeAmpersand, "&")
				return optional.Some(t)
			}
			switch n.Value() {
			case '&':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeBinAnd, "&&")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeAmpersand, "&")
				return optional.Some(t)
			}
		case '\'':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeSquote, "'")
			return optional.Some(t)
		case '?':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeQuestion, "?")
			return optional.Some(t)
		case '|':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypePipe, "|")
				return optional.Some(t)
			}
			switch n.Value() {
			case '|':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeBinOr, "||")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypePipe, "|")
				return optional.Some(t)
			}
		case ';':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeSemicolon, ";")
			return optional.Some(t)
		case '=':
			n := self.body.Lookahead(ctx, 1)
			if !n.IsPresent() {
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeEqual, "=")
				return optional.Some(t)
			}
			switch n.Value() {
			case '=':
				_ = self.next(ctx)
				t := newTokenLineSpan(self.line, self.col, self.offset, 2, idl.TokenTypeComparison, "==")
				return optional.Some(t)
			default:
				t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeEqual, "=")
				return optional.Some(t)
			}
		case '_':
			// an underscore with no other known context is treated as an identifier
			// it may be either a free standing underscore or the beginning of a
			// longer identifier
			return self.readIdentifier(ctx, string(r))
		default:
			if unicode.IsLetter(r) {
				tok := self.readIdentifier(ctx, string(r))
				if !tok.IsPresent() {
					return optional.None[*idl.Token]()
				}
				t := tok.Value()
				switch t.Value {
				case "import":
					t.Type = idl.TokenTypeKeywordImport
				case "as":
					t.Type = idl.TokenTypeKeywordAs
				case "const":
					t.Type = idl.TokenTypeKeywordConst
				case "annotation":
					t.Type = idl.TokenTypeKeywordAnnotation
				case "struct":
					t.Type = idl.TokenTypeKeywordStruct
				case "field":
					t.Type = idl.TokenTypeKeywordField
				case "union":
					t.Type = idl.TokenTypeKeywordUnion
				case "enum":
					t.Type = idl.TokenTypeKeywordEnum
				case "enumerant":
					t.Type = idl.TokenTypeKeywordEnumerant
				case "interface":
					t.Type = idl.TokenTypeKeywordInterface
				case "api":
					t.Type = idl.TokenTypeKeywordAPI
				case "method":
					t.Type = idl.TokenTypeKeywordMethod
				case "sdk":
					t.Type = idl.TokenTypeKeywordSDK
				case "impl":
					t.Type = idl.TokenTypeKeywordImpl
				case "module":
					t.Type = idl.TokenTypeKeywordModule
				case "syntax":
					t.Type = idl.TokenTypeKeywordSyntax
				case "extends":
					t.Type = idl.TokenTypeKeywordExtends
				case "throws":
					t.Type = idl.TokenTypeKeywordThrows
				case "nothrows":
					t.Type = idl.TokenTypeKeywordNothrows
				case "throw":
					t.Type = idl.TokenTypeKeywordThrow
				case "returns":
					t.Type = idl.TokenTypeKeywordReturns
				case "return":
					t.Type = idl.TokenTypeKeywordReturn
				case "catch":
					t.Type = idl.TokenTypeKeywordCatch
				case "switch":
					t.Type = idl.TokenTypeKeywordSwitch
				case "case":
					t.Type = idl.TokenTypeKeywordCase
				case "default":
					t.Type = idl.TokenTypeKeywordDefault
				case "var":
					t.Type = idl.TokenTypeKeywordVar
				case "for":
					t.Type = idl.TokenTypeKeywordFor
				case "in":
					t.Type = idl.TokenTypeKeywordIn
				case "while":
					t.Type = idl.TokenTypeKeywordWhile
				case "set":
					t.Type = idl.TokenTypeKeywordSet
				case "requires":
					t.Type = idl.TokenTypeKeywordRequires
				case "if":
					t.Type = idl.TokenTypeKeywordIf
				case "else":
					t.Type = idl.TokenTypeKeywordElse
				case "true":
					t.Type = idl.TokenTypeKeywordTrue
				case "false":
					t.Type = idl.TokenTypeKeywordFalse
				case "async":
					t.Type = idl.TokenTypeKeywordAsync
				case "await":
					t.Type = idl.TokenTypeKeywordAwait
				case "exec":
					t.Type = idl.TokenTypeKeywordExec
				}
				return optional.Some(t)
			}
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeUnknown, string(r))
			return optional.Some(t)
		}
	}
	return optional.None[*idl.Token]()
}

func (self *lexerFileMicroglotTokens) readIdentifier(ctx context.Context, prefix string) optional.Optional[*idl.Token] {
	var builder strings.Builder
	_, _ = builder.WriteString(prefix)
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeIdentifier, builder.String())
			return optional.Some(t)
		}
		if unicode.IsLetter(rune(n.Value())) || unicode.IsDigit(rune(n.Value())) || n.Value() == '_' {
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			continue
		}
		t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeIdentifier, builder.String())
		return optional.Some(t)
	}
}

func (self *lexerFileMicroglotTokens) readCommentLine(ctx context.Context) optional.Optional[*idl.Token] {
	var builder strings.Builder
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeComment, builder.String())
			return optional.Some(t)
		}
		switch n.Value() {
		case '\r':
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeComment, builder.String())
			return optional.Some(t)
		case '\n':
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeComment, builder.String())
			return optional.Some(t)
		default:
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		}
	}
}

func (self *lexerFileMicroglotTokens) readCommentBlock(ctx context.Context) optional.Optional[*idl.Token] {
	var builder strings.Builder
	startLine := self.line
	startCol := self.col
	startOffset := self.offset
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading comment block"))
			t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset, idl.TokenTypeComment, builder.String())
			return optional.Some(t)
		}
		switch n.Value() {
		case '\n':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			self.newLine()
		case '\r':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			nn := self.body.Lookahead(ctx, 2)
			if nn.IsPresent() && nn.Value() == '\n' {
				_ = self.next(ctx)
				_, _ = builder.WriteRune(rune(nn.Value()))
			}
			self.newLine()
		case '*':
			nn := self.body.Lookahead(ctx, 2)
			if !nn.IsPresent() {
				_ = self.next(ctx)
				_, _ = builder.WriteRune(rune(n.Value()))
			}
			if nn.Value() == '/' {
				_ = self.next(ctx)
				_ = self.next(ctx)
				t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset, idl.TokenTypeComment, builder.String())
				return optional.Some(t)
			}
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		default:
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		}
	}
}

func (self *lexerFileMicroglotTokens) readProse(ctx context.Context) optional.Optional[*idl.Token] {
	var builder strings.Builder
	startLine := self.line
	startCol := self.col + 1       // Adjust by one to account for opening tick
	startOffset := self.offset + 1 // Adjust by one to account for opening tick
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading prose"))
			t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset, idl.TokenTypeProse, builder.String())
			return optional.Some(t)
		}
		switch n.Value() {
		case '\n':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			self.newLine()
		case '\r':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			nn := self.body.Lookahead(ctx, 2)
			if nn.IsPresent() && nn.Value() == '\n' {
				_ = self.next(ctx)
				_, _ = builder.WriteRune(rune(nn.Value()))
			}
			self.newLine()
		case '`':
			_ = self.next(ctx)
			t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset, idl.TokenTypeProse, builder.String())
			return optional.Some(t)
		case '\\':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			nn := self.body.Lookahead(ctx, 1)
			if !nn.IsPresent() {
				_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading prose"))
				t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset, idl.TokenTypeProse, builder.String())
				return optional.Some(t)
			}
			if nn.Value() == '`' {
				_ = self.next(ctx)
				_, _ = builder.WriteRune('`')
			}
		default:
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		}
	}
}

func (self *lexerFileMicroglotTokens) readText(ctx context.Context) optional.Optional[*idl.Token] {
	var builder strings.Builder
	startLine := self.line
	startCol := self.col + 1       // Adjust one to account for the leading quotation
	startOffset := self.offset + 1 // Adjust one to account for the leading quotation
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading text literal"))
			t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset, idl.TokenTypeText, builder.String())
			return optional.Some(t)
		}
		switch n.Value() {
		case '\n':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			self.newLine()
		case '\r':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			nn := self.body.Lookahead(ctx, 2)
			if nn.IsPresent() && nn.Value() == '\n' {
				_ = self.next(ctx)
				_, _ = builder.WriteRune(rune(nn.Value()))
			}
			self.newLine()
		case '"':
			_ = self.next(ctx)
			t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset, idl.TokenTypeText, builder.String())
			return optional.Some(t)
		case '\\':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			nn := self.body.Lookahead(ctx, 1)
			if !nn.IsPresent() {
				_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading text literal"))
				t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset, idl.TokenTypeText, builder.String())
				return optional.Some(t)
			}
			if nn.Value() == '"' {
				_ = self.next(ctx)
				_, _ = builder.WriteRune('"')
			}
		default:
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		}
	}
}

func (self *lexerFileMicroglotTokens) readData(ctx context.Context) optional.Optional[*idl.Token] {
	t := self.readText(ctx)
	if !t.IsPresent() {
		return t
	}
	tok := t.Value()
	tok.Type = idl.TokenTypeData
	return optional.Some(tok)
}

func (self *lexerFileMicroglotTokens) readNumber(ctx context.Context, prefix string) optional.Optional[*idl.Token] {
	switch prefix {
	case "0":
		// octal, plain zero decimal integer, or zero prefix decimal float
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			t := newTokenLineSpan(self.line, self.col, self.offset, 1, idl.TokenTypeIntegerDecimal, prefix)
			return optional.Some(t)
		}
		switch n.Value() {
		case '_', '0', '1', '2', '3', '4', '5', '6', '7':
			return self.readOctal(ctx, prefix)
		case '.':
			// 0 .
			prefix = prefix + string(rune(n.Value()))
			_ = self.next(ctx)
			tok := newTokenLineSpan(self.line, self.col, self.offset, len(prefix), idl.TokenTypeFloatDecimal, prefix)
			nn := self.body.Lookahead(ctx, 1)
			if !nn.IsPresent() {
				return optional.Some(tok)
			}
			switch nn.Value() {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				// 0 . DEC
				childTok := self.readDecimal(ctx, "")
				if !childTok.IsPresent() {
					_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid fractional in decimal float literal"))
					return optional.None[*idl.Token]()
				}
				tok.Value = tok.Value + childTok.Value().Value
				tok.Span.End = childTok.Value().Span.End
				nnn := self.body.Lookahead(ctx, 1)
				if !nnn.IsPresent() {
					return optional.Some(tok)
				}
				switch nnn.Value() {
				case 'e', 'E':
					// 0 . DEC EXP
					_ = self.next(ctx)
					childTok = self.readDecimalExponent(ctx, string(rune(nnn.Value())))
					if !childTok.IsPresent() {
						_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid exponent in decimal float literal"))
						return optional.None[*idl.Token]()
					}
					tok.Value = tok.Value + childTok.Value().Value
					tok.Span.End = childTok.Value().Span.End
					return optional.Some(tok)
				default:
					return optional.Some(tok)
				}
			case 'e', 'E':
				// 0 . EXP
				_ = self.next(ctx)
				childTok := self.readDecimalExponent(ctx, string(rune(nn.Value())))
				if !childTok.IsPresent() {
					_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid exponent in decimal float literal"))
					return optional.None[*idl.Token]()
				}
				tok.Value = tok.Value + childTok.Value().Value
				tok.Span.End = childTok.Value().Span.End
				return optional.Some(tok)
			default:
				// 0 . <NOTHING>
				return optional.Some(tok)
			}
		default:
			t := newTokenLineSpan(self.line, self.col, self.offset, len(prefix), idl.TokenTypeIntegerDecimal, prefix)
			return optional.Some(t)
		}
	case ".":
		// decimal
		tok := self.readDecimal(ctx, prefix)
		if !tok.IsPresent() {
			return optional.None[*idl.Token]()
		}
		t := tok.Value()
		t.Type = idl.TokenTypeFloatDecimal
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			return optional.Some(t)
		}
		switch n.Value() {
		case 'e', 'E':
			_ = self.next(ctx)
			childTok := self.readDecimalExponent(ctx, string(rune(n.Value())))
			if !childTok.IsPresent() {
				_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading exponent in decimal float literal"))
				return optional.None[*idl.Token]()
			}
			t.Value = t.Value + childTok.Value().Value
			t.Span.End = childTok.Value().Span.End
			return optional.Some(t)
		default:
			return optional.Some(t)
		}
	case "0x", "0X":
		// hex
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading hex literal"))
			return optional.None[*idl.Token]()
		}
		switch n.Value() {
		case '.':
			_ = self.next(ctx)
			tok := newTokenLineSpan(self.line, self.col, self.offset, len(prefix)+1, idl.TokenTypeFloatHex, prefix+".")
			nn := self.body.Lookahead(ctx, 1)
			if !nn.IsPresent() {
				_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading hex float literal"))
				return optional.None[*idl.Token]()
			}
			switch nn.Value() {
			case '_', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'A', 'B', 'C', 'D', 'E', 'F':
				// . HEX
				childTok := self.readHex(ctx, "")
				if !childTok.IsPresent() {
					_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid fractional in hex float literal"))
					return optional.None[*idl.Token]()
				}
				tok.Value = tok.Value + childTok.Value().Value
				tok.Span.End = childTok.Value().Span.End
				nnn := self.body.Lookahead(ctx, 1)
				if !nnn.IsPresent() {
					return optional.Some(tok)
				}
				switch nnn.Value() {
				case 'p', 'P':
					// . HEX EXP
					_ = self.next(ctx)
					childTok := self.readDecimalExponent(ctx, string(rune(nnn.Value())))
					if !childTok.IsPresent() {
						_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid exponent in hex float literal"))
						return optional.None[*idl.Token]()
					}
					tok.Value = tok.Value + childTok.Value().Value
					tok.Span.End = childTok.Value().Span.End
					return optional.Some(tok)
				default:
					return optional.Some(tok)
				}
			default:
				// . <NOTHING>
				_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "missing fractional part in hex float literal"))
				return optional.None[*idl.Token]()
			}
		case '_', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'A', 'B', 'C', 'D', 'E', 'F':
			// HEX
			tok := self.readHex(ctx, prefix)
			if !tok.IsPresent() {
				return optional.None[*idl.Token]()
			}
			nn := self.body.Lookahead(ctx, 1)
			if !nn.IsPresent() {
				return tok
			}
			switch nn.Value() {
			case '.':
				// HEX .
				t := tok.Value()
				t.Type = idl.TokenTypeFloatHex
				_ = self.next(ctx)
				t.Value = t.Value + "."
				nnn := self.body.Lookahead(ctx, 1)
				if !nnn.IsPresent() {
					return optional.Some(t)
				}
				switch nnn.Value() {
				case 'p', 'P':
					// HEX . EXP
					_ = self.next(ctx)
					childTok := self.readDecimalExponent(ctx, string(rune(nnn.Value())))
					if !childTok.IsPresent() {
						_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid exponent in hex float literal"))
						return optional.None[*idl.Token]()
					}
					t.Value = t.Value + childTok.Value().Value
					t.Span.End = childTok.Value().Span.End
					return optional.Some(t)
				case '_', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'A', 'B', 'C', 'D', 'E', 'F':
					// HEX . HEX
					childTok := self.readHex(ctx, "")
					if !childTok.IsPresent() {
						_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid fractional in hex float literal"))
						return optional.None[*idl.Token]()
					}
					t.Value = t.Value + childTok.Value().Value
					t.Span.End = childTok.Value().Span.End
					nnnn := self.body.Lookahead(ctx, 1)
					if !nnnn.IsPresent() {
						return optional.Some(t)
					}
					switch nnnn.Value() {
					case 'p', 'P':
						// HEX . HEX EXP
						_ = self.next(ctx)
						childTok := self.readDecimalExponent(ctx, string(rune(nnnn.Value())))
						if !childTok.IsPresent() {
							_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid exponent in hex float literal"))
							return optional.None[*idl.Token]()
						}
						t.Value = t.Value + childTok.Value().Value
						t.Span.End = childTok.Value().Span.End
						return optional.Some(t)
					default:
						return optional.Some(t)
					}
				default:
					// HEX . <NOTHING>
					return optional.Some(t)
				}
			case 'p', 'P':
				// HEX EXP
				_ = self.next(ctx)
				t := tok.Value()
				t.Type = idl.TokenTypeFloatHex
				childTok := self.readDecimalExponent(ctx, string(rune(nn.Value())))
				if !childTok.IsPresent() {
					_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid exponent in hex float literal"))
					return optional.None[*idl.Token]()
				}
				t.Value = t.Value + childTok.Value().Value
				t.Span.End = childTok.Value().Span.End
				return optional.Some(t)
			default:
				return tok
			}
		default:
			_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "missing hex value after hex literal prefix"))
			return optional.None[*idl.Token]()
		}
	case "0b", "0B":
		// binary
		return self.readBinary(ctx, prefix)
	case "0o", "0O":
		// octal
		return self.readOctal(ctx, prefix)
	default:
		_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "unrecognized numberic base"))
		return optional.None[*idl.Token]()
	}
}

func (self *lexerFileMicroglotTokens) readHex(ctx context.Context, prefix string) optional.Optional[*idl.Token] {
	var builder strings.Builder
	_, _ = builder.WriteString(prefix)
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			if builder.Len() == len(prefix) {
				_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading hex integer literal"))
				return optional.None[*idl.Token]()
			}
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeIntegerHex, builder.String())
			return optional.Some(t)
		}
		switch n.Value() {
		case '_', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'A', 'B', 'C', 'D', 'E', 'F':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		default:
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeIntegerHex, builder.String())
			return optional.Some(t)
		}
	}
}

func (self *lexerFileMicroglotTokens) readDecimalExponent(ctx context.Context, prefix string) optional.Optional[*idl.Token] {
	var builder strings.Builder
	_, _ = builder.WriteString(prefix)
	n := self.body.Lookahead(ctx, 1)
	if !n.IsPresent() {
		_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading exponent of decimal float literal"))
		return optional.None[*idl.Token]()
	}
	switch n.Value() {
	case '+', '-':
		_ = self.next(ctx)
		builder.WriteRune(rune(n.Value()))
	}
	childTok := self.readDecimal(ctx, "")
	if !childTok.IsPresent() {
		_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "invalid decimal in decimal exponent"))
		return optional.None[*idl.Token]()
	}
	ct := childTok.Value()
	ct.Value = builder.String() + ct.Value
	return optional.Some(ct)
}

func (self *lexerFileMicroglotTokens) readDecimal(ctx context.Context, prefix string) optional.Optional[*idl.Token] {
	var builder strings.Builder
	_, _ = builder.WriteString(prefix)
	tokType := idl.TokenTypeIntegerDecimal
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), tokType, builder.String())
			return optional.Some(t)
		}
		switch n.Value() {
		case '_', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		case '.':
			tokType = idl.TokenTypeFloatDecimal
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		case 'e', 'E':
			tokType = idl.TokenTypeFloatDecimal
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			expTok := self.readDecimalExponent(ctx, "")
			if !expTok.IsPresent() {
				_ = self.reporter.Report(self.exc(exc.CodeInvalidNumber, "invalid exponent in decimal float literal"))
				return optional.None[*idl.Token]()
			}
			builder.WriteString(expTok.Value().Value)
		default:
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), tokType, builder.String())
			return optional.Some(t)
		}
	}
}

func (self *lexerFileMicroglotTokens) readBinary(ctx context.Context, prefix string) optional.Optional[*idl.Token] {
	var builder strings.Builder
	_, _ = builder.WriteString(prefix)
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			if builder.Len() == len(prefix) {
				_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading binary integer literal"))
				return optional.None[*idl.Token]()
			}
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeIntegerBinary, builder.String())
			return optional.Some(t)
		}
		switch n.Value() {
		case '_', '0', '1':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		default:
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeIntegerBinary, builder.String())
			return optional.Some(t)
		}
	}
}

func (self *lexerFileMicroglotTokens) readOctal(ctx context.Context, prefix string) optional.Optional[*idl.Token] {
	var builder strings.Builder
	_, _ = builder.WriteString(prefix)
	for {
		n := self.body.Lookahead(ctx, 1)
		if !n.IsPresent() {
			if builder.Len() == len(prefix) {
				_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading octal integer literal"))
				return optional.None[*idl.Token]()
			}
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeIntegerOctal, builder.String())
			return optional.Some(t)
		}
		switch n.Value() {
		case '_', '0', '1', '2', '3', '4', '5', '6', '7':
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
		default:
			t := newTokenLineSpan(self.line, self.col, self.offset, builder.Len(), idl.TokenTypeIntegerOctal, builder.String())
			return optional.Some(t)
		}
	}
}

func (self *lexerFileMicroglotTokens) next(ctx context.Context) optional.Optional[idl.CodePoint] {
	n := self.body.Next(ctx)
	if n.IsPresent() {
		self.addCol(rune(n.Value()))
	}
	return n
}

func (self *lexerFileMicroglotTokens) exc(code string, message string) exc.Exception {
	return exc.New(exc.Location{URI: self.uri, Location: idl.Location{Line: self.line, Column: self.col, Offset: self.offset}}, code, message)
}

func (self *lexerFileMicroglotTokens) newLine() {
	self.line = self.line + 1
	self.col = 0
	self.offset = self.offset + 1
}

func (self *lexerFileMicroglotTokens) newLineToken(v string, size int) optional.Optional[*idl.Token] {
	t := newToken(self.line, self.col-int32(size-1), self.offset-int64(size), self.line+1, 1, self.offset, idl.TokenTypeNewline, v)
	self.newLine()
	return optional.Some(t)
}

func (self *lexerFileMicroglotTokens) addCol(r rune) {
	self.col = self.col + 1
	self.offset = self.offset + int64(len(string(r)))
}

func (self *lexerFileMicroglotTokens) Close(ctx context.Context) error {
	return self.body.Close(ctx)
}

func newTokenLineSpan(line int32, col int32, offset int64, size int, kind idl.TokenType, value string) *idl.Token {
	return &idl.Token{
		Span: &idl.Span{
			Start: &idl.Location{
				Line:   line,
				Column: col - int32(size),
				Offset: offset - int64(size),
			},
			End: &idl.Location{
				Line:   line,
				Column: col,
				Offset: offset,
			},
		},
		Type:  kind,
		Value: value,
	}
}

func newToken(startLine int32, startCol int32, startOffset int64, endLine int32, endCol int32, endOffset int64, kind idl.TokenType, value string) *idl.Token {
	return &idl.Token{
		Span: &idl.Span{
			Start: &idl.Location{
				Line:   startLine,
				Column: startCol,
				Offset: startOffset,
			},
			End: &idl.Location{
				Line:   endLine,
				Column: endCol,
				Offset: endOffset,
			},
		},
		Type:  kind,
		Value: value,
	}
}
