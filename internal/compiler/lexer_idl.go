// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package compiler

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
	lexerIDLLookahead = 8
)

// LexerIDL is a minimal lexer that only emits tokens relevant to syntax
// detection. This is used to parse files that may be in any of the supported
// IDL syntaxes before switching to a more specific lexer.
type LexerIDL struct {
	reporter exc.Reporter
}

func NewLexerIDL(reporter exc.Reporter) *LexerIDL {
	return &LexerIDL{reporter: reporter}
}

func (self *LexerIDL) Lex(ctx context.Context, f idl.File) (idl.LexerFile, error) {
	return &lexerFileIDL{
		File:     f,
		reporter: self.reporter,
	}, nil
}

type lexerFileIDL struct {
	idl.File
	reporter exc.Reporter
}

func (self *lexerFileIDL) Tokens(ctx context.Context) (idl.Iterator[*idl.Token], error) {
	b, err := self.File.Body(ctx)
	if err != nil {
		return nil, err
	}
	points := iter.NewLookahead(iter.NewUnicodeFileBodyCtx(ctx, b), lexerIDLLookahead)
	return &lexerFileIDLTokens{
		uri:        self.File.Path(ctx),
		codePoints: points,
		reporter:   self.reporter,
		line:       1,
		col:        0,
		offset:     -1,
	}, nil
}

type lexerFileIDLTokens struct {
	uri        string
	codePoints idl.Lookahead[idl.CodePoint]
	reporter   exc.Reporter
	line       int32
	col        int32
	offset     int64
	hasBOM     bool
}

func (self *lexerFileIDLTokens) Next(ctx context.Context) optional.Optional[*idl.Token] {
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
			return optional.None[*idl.Token]()
		case '\n':
			return self.newLineToken("\n", 1)
		case '\r':
			if n := self.codePoints.Lookahead(ctx, 1); n.IsPresent() && n.Value() == '\n' {
				_ = self.next(ctx)
				return self.newLineToken("\r\n", 2)
			}
			return self.newLineToken("\r", 1)
		case '=':
			t := newToken(self.line, self.col-1, self.offset, self.line, self.col, self.offset+1, idl.TokenTypeEqual, "=")
			return optional.Some(t)
		case '"':
			return self.readText(ctx)
		default:
			if unicode.IsLetter(r) {
				tok := self.readIdentifier(ctx, string(r))
				if !tok.IsPresent() {
					continue
				}
				t := tok.Value()
				switch t.Value {
				case "syntax":
					t.Type = idl.TokenTypeKeywordSyntax
					return optional.Some(t)
				}
			}
		}
	}
	return optional.None[*idl.Token]()
}

func (self *lexerFileIDLTokens) readText(ctx context.Context) optional.Optional[*idl.Token] {
	var builder strings.Builder
	startLine := self.line
	startCol := self.col
	startOffset := self.offset
	for {
		n := self.codePoints.Lookahead(ctx, 1)
		if !n.IsPresent() {
			_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading Text literal"))
			return optional.None[*idl.Token]()
		}
		_ = self.next(ctx)
		switch n.Value() {
		case '"':
			t := newToken(startLine, startCol, startOffset, self.line, self.col, self.offset+1, idl.TokenTypeText, builder.String())
			return optional.Some(t)
		case '\\':
			_, _ = builder.WriteRune(rune(n.Value()))
			nn := self.codePoints.Lookahead(ctx, 1)
			if !nn.IsPresent() {
				_ = self.reporter.Report(self.exc(exc.CodeUnexpectedEOF, "EOF while reading Text literal"))
				return optional.None[*idl.Token]()
			}
			if nn.Value() == '"' {
				_ = self.next(ctx)
				_, _ = builder.WriteRune('"')
			}
		default:
			_, _ = builder.WriteRune(rune(n.Value()))
		}
	}
}

func (self *lexerFileIDLTokens) readIdentifier(ctx context.Context, prefix string) optional.Optional[*idl.Token] {
	var builder strings.Builder
	_, _ = builder.WriteString(prefix)
	for {
		n := self.codePoints.Lookahead(ctx, 1)
		if !n.IsPresent() {
			t := newToken(self.line, self.col-int32(builder.Len()), self.offset-int64(builder.Len()), self.line, self.col, self.offset, idl.TokenTypeIdentifier, builder.String())
			return optional.Some(t)
		}
		if unicode.IsLetter(rune(n.Value())) || unicode.IsDigit(rune(n.Value())) || n.Value() == '_' {
			_ = self.next(ctx)
			_, _ = builder.WriteRune(rune(n.Value()))
			continue
		}
		t := newToken(self.line, self.col-int32(builder.Len()), self.offset-int64(builder.Len()), self.line, self.col, self.offset, idl.TokenTypeIdentifier, builder.String())
		return optional.Some(t)
	}
}

func (self *lexerFileIDLTokens) next(ctx context.Context) optional.Optional[idl.CodePoint] {
	n := self.codePoints.Next(ctx)
	if n.IsPresent() {
		self.addCol(rune(n.Value()))
	}
	return n
}

func (self *lexerFileIDLTokens) exc(code string, message string) exc.Exception {
	return exc.New(exc.Location{URI: self.uri, Location: idl.Location{Line: self.line, Column: self.col, Offset: self.offset}}, code, message)
}

func (self *lexerFileIDLTokens) newLine() {
	self.line = self.line + 1
	self.col = 0
	self.offset = self.offset + 1
}

func (self *lexerFileIDLTokens) newLineToken(v string, size int) optional.Optional[*idl.Token] {
	t := newToken(self.line, self.col-int32(size-1), self.offset-int64(size), self.line+1, 1, self.offset, idl.TokenTypeNewline, v)
	self.newLine()
	return optional.Some(t)
}

func (self *lexerFileIDLTokens) addCol(r rune) {
	self.col = self.col + 1
	self.offset = self.offset + int64(len(string(r)))
}

func (self *lexerFileIDLTokens) Close(ctx context.Context) error {
	return self.codePoints.Close(ctx)
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
