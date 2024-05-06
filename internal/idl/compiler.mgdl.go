package idl

import (
	"context"
	"fmt"
	"math"

	"gopkg.microglot.org/compiler.go/internal/optional"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

type Closer interface {
	Close(ctx context.Context) error
}

type CodePoint uint32

type Iterator[T any] interface {
	Next(ctx context.Context) optional.Optional[T]
	Closer
}

type Lookahead[T any] interface {
	Iterator[T]
	Lookahead(ctx context.Context, n uint8) optional.Optional[T]
}

type Filter[T any] interface {
	Keep(ctx context.Context, v T) bool
}

type Reader interface {
	Read(ctx context.Context, size int32) ([]byte, error)
}

type FileBody interface {
	Reader
	Closer
}

type FileKind uint32

// TODO 2023.11.09: this should be derived from the .mgdl descriptor. Right now we're still using the
// .proto descriptor, which doesn't support consts.
const Incomplete uint64 = math.MaxUint64

const (
	FileKindNone FileKind = iota
	FileKindMicroglot
	FileKindMicroglotDescBinary
	FileKindMicroglotDescJSON
	FileKindMicroglotDescProto
	FileKindProtobuf
	FileKindProtobufDesc
)

func (k FileKind) String() string {
	switch k {
	case FileKindMicroglot:
		return "microglot"
	case FileKindMicroglotDescBinary:
		return "microglot-descriptor"
	case FileKindMicroglotDescJSON:
		return "microglot-descriptor-json"
	case FileKindMicroglotDescProto:
		return "microglot-descriptor-proto"
	case FileKindNone:
		return "none"
	case FileKindProtobuf:
		return "protobuf"
	case FileKindProtobufDesc:
		return "protobuf-descriptor"
	default:
		return fmt.Sprintf("unkown-%d", k)
	}
}

type File interface {
	Path(ctx context.Context) string
	Kind(ctx context.Context) FileKind
	Body(ctx context.Context) (FileBody, error)
}

type FileSystem interface {
	Open(ctx context.Context, uri string) ([]File, error)
	Write(ctx context.Context, uri string, content string) error
}

type Compiler interface {
	Compile(ctx context.Context, req *CompileRequest) (*CompileResponse, error)
}

type CompileRequest struct {
	Files      []string
	DumpTokens bool
	DumpTree   bool
}

type CompileResponse struct {
	Image *Image
}

type Image struct {
	// TODO 2023.09.12: this should be re-pointed at the local Module def
	Modules []*proto.Module
}
type Module struct {
	URI string
	// TODO: Populate this with descriptor content
}

type LexerFile interface {
	File
	Tokens(ctx context.Context) (Iterator[*Token], error)
}

type Lexer interface {
	Lex(ctx context.Context, f File) (LexerFile, error)
}

type Parser interface {
	Parse(ctx context.Context, f LexerFile) (*Module, error)
}

type Token struct {
	Span  *proto.Span
	Type  TokenType
	Value string
}

type TokenType uint16

//go:generate stringer -type=TokenType
const (
	TokenTypeUnknown           TokenType = 0
	TokenTypeIdentifier        TokenType = 1
	TokenTypeIntegerDecimal    TokenType = 2
	TokenTypeIntegerHex        TokenType = 3
	TokenTypeIntegerOctal      TokenType = 4
	TokenTypeIntegerBinary     TokenType = 5
	TokenTypeFloatDecimal      TokenType = 6
	TokenTypeFloatHex          TokenType = 7
	TokenTypeText              TokenType = 8
	TokenTypeData              TokenType = 9
	TokenTypeComment           TokenType = 10
	TokenTypeEscaped           TokenType = 11
	TokenTypeProse             TokenType = 12
	TokenTypeQuote             TokenType = 13
	TokenTypeTick              TokenType = 14
	TokenTypeCurlyOpen         TokenType = 15
	TokenTypeCurlyClose        TokenType = 16
	TokenTypeSquareOpen        TokenType = 17
	TokenTypeSquareClose       TokenType = 18
	TokenTypeParenOpen         TokenType = 19
	TokenTypeParenClose        TokenType = 20
	TokenTypePlus              TokenType = 21
	TokenTypePlusEqual         TokenType = 22
	TokenTypeMinus             TokenType = 23
	TokenTypeMinusEqual        TokenType = 24
	TokenTypeDot               TokenType = 25
	TokenTypeUnderscore        TokenType = 26
	TokenTypeStar              TokenType = 27
	TokenTypeMultiplyEqual     TokenType = 28
	TokenTypeComma             TokenType = 29
	TokenTypeColon             TokenType = 30
	TokenTypeAngleOpen         TokenType = 31
	TokenTypeLesserEqual       TokenType = 32
	TokenTypeAngleClose        TokenType = 33
	TokenTypeGreaterEqual      TokenType = 34
	TokenTypeDollar            TokenType = 35
	TokenTypeAt                TokenType = 36
	TokenTypeEqual             TokenType = 37
	TokenTypeComparison        TokenType = 38
	TokenTypeNotComparison     TokenType = 39
	TokenTypeSlash             TokenType = 40
	TokenTypeDivideEqual       TokenType = 41
	TokenTypeExclamation       TokenType = 42
	TokenTypePercent           TokenType = 43
	TokenTypeCaret             TokenType = 44
	TokenTypeAmpersand         TokenType = 45
	TokenTypeBinAnd            TokenType = 46
	TokenTypePipe              TokenType = 47
	TokenTypeBinOr             TokenType = 48
	TokenTypeQuestion          TokenType = 49
	TokenTypeSquote            TokenType = 50
	TokenTypeTilde             TokenType = 51
	TokenTypeSemicolon         TokenType = 52
	TokenTypeKeywordImport     TokenType = 53
	TokenTypeKeywordAs         TokenType = 54
	TokenTypeKeywordConst      TokenType = 55
	TokenTypeKeywordAnnotation TokenType = 56
	TokenTypeKeywordStruct     TokenType = 57
	TokenTypeKeywordField      TokenType = 58
	TokenTypeKeywordUnion      TokenType = 59
	TokenTypeKeywordEnum       TokenType = 60
	TokenTypeKeywordEnumerant  TokenType = 61
	TokenTypeKeywordInterface  TokenType = 62
	TokenTypeKeywordAPI        TokenType = 63
	TokenTypeKeywordMethod     TokenType = 64
	TokenTypeKeywordSDK        TokenType = 65
	TokenTypeKeywordImpl       TokenType = 66
	TokenTypeKeywordModule     TokenType = 67
	TokenTypeKeywordSyntax     TokenType = 68
	TokenTypeKeywordExtends    TokenType = 69
	TokenTypeKeywordThrows     TokenType = 70
	TokenTypeKeywordNothrows   TokenType = 71
	TokenTypeKeywordReturns    TokenType = 72
	TokenTypeKeywordThrow      TokenType = 73
	TokenTypeKeywordCatch      TokenType = 74
	TokenTypeKeywordReturn     TokenType = 75
	TokenTypeKeywordSwitch     TokenType = 76
	TokenTypeKeywordDefault    TokenType = 77
	TokenTypeKeywordVar        TokenType = 78
	TokenTypeKeywordFor        TokenType = 79
	TokenTypeKeywordIn         TokenType = 80
	TokenTypeKeywordWhile      TokenType = 81
	TokenTypeKeywordSet        TokenType = 82
	TokenTypeKeywordRequires   TokenType = 83
	TokenTypeKeywordCase       TokenType = 84
	TokenTypeKeywordIf         TokenType = 85
	TokenTypeKeywordElse       TokenType = 86
	TokenTypeKeywordTrue       TokenType = 87
	TokenTypeKeywordFalse      TokenType = 88
	TokenTypeKeywordAsync      TokenType = 89
	TokenTypeKeywordAwait      TokenType = 90
	TokenTypeKeywordExec       TokenType = 91
	TokenTypeWhitespace        TokenType = 92
	TokenTypeNewline           TokenType = 93
	TokenTypeEOF               TokenType = 94
)
