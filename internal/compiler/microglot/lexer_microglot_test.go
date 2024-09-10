// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package microglot

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gopkg.microglot.org/mglotc/internal/exc"
	"gopkg.microglot.org/mglotc/internal/fs"
	"gopkg.microglot.org/mglotc/internal/idl"
	"gopkg.microglot.org/mglotc/internal/optional"
)

func TestLexer(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected []struct {
			token *idl.Token
			err   error
		}
		verifyLineCol bool
	}{
		{
			name:  "empty file",
			input: "",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			name:  "new lines",
			input: "\n\n\r\r\n",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newToken(1, 1, 0, 2, 1, 1, idl.TokenTypeNewline, "\n"),
					err:   nil,
				},
				{
					token: newToken(2, 1, 1, 3, 1, 2, idl.TokenTypeNewline, "\n"),
					err:   nil,
				},
				{
					token: newToken(3, 1, 2, 4, 1, 3, idl.TokenTypeNewline, "\r"),
					err:   nil,
				},
				{
					token: newToken(4, 1, 3, 5, 1, 5, idl.TokenTypeNewline, "\r\n"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(5, 1, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			name:  "spaces",
			input: "     \n   \n  ",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newToken(1, 6, 5, 2, 1, 6, idl.TokenTypeNewline, "\n"),
					err:   nil,
				},
				{
					token: newToken(2, 4, 6, 3, 1, 7, idl.TokenTypeNewline, "\n"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(3, 3, 7, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "1234",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 4, 4, 4, idl.TokenTypeIntegerDecimal, "1234"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 4, 4, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "12_34",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 5, 5, 5, idl.TokenTypeIntegerDecimal, "12_34"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0600",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 4, 4, 4, idl.TokenTypeIntegerOctal, "0600"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 4, 4, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0_600",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 5, 5, 5, idl.TokenTypeIntegerOctal, "0_600"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0o600",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 5, 5, 5, idl.TokenTypeIntegerOctal, "0o600"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0O600",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 5, 5, 5, idl.TokenTypeIntegerOctal, "0O600"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
		},
		{
			input: "0xBadFace",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 9, 9, 9, idl.TokenTypeIntegerHex, "0xBadFace"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 9, 9, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0xBad_Face",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 10, 10, 10, idl.TokenTypeIntegerHex, "0xBad_Face"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 10, 10, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0x_67_7a_2f_cc_40_c6",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 20, 20, 20, idl.TokenTypeIntegerHex, "0x_67_7a_2f_cc_40_c6"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 20, 20, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "170141183460469231731687303715884105727",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 39, 39, 39, idl.TokenTypeIntegerDecimal, "170141183460469231731687303715884105727"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 39, 39, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "170_141183_460469_231731_687303_715884_105727",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 45, 45, 45, idl.TokenTypeIntegerDecimal, "170_141183_460469_231731_687303_715884_105727"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 45, 45, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeIntegerDecimal, "0"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 1, 1, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0b0101",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 6, 6, 6, idl.TokenTypeIntegerBinary, "0b0101"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 6, 6, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0B0101",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 6, 6, 6, idl.TokenTypeIntegerBinary, "0B0101"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 6, 6, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0.",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 2, 2, 2, idl.TokenTypeFloatDecimal, "0."),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 2, 2, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0.E1",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 4, 4, 4, idl.TokenTypeFloatDecimal, "0.E1"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 4, 4, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0.E-1",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 5, 5, 5, idl.TokenTypeFloatDecimal, "0.E-1"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0.E+1",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 5, 5, 5, idl.TokenTypeFloatDecimal, "0.E+1"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "72.40",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 5, 5, 5, idl.TokenTypeFloatDecimal, "72.40"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "2.71828",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 7, 7, 7, idl.TokenTypeFloatDecimal, "2.71828"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 7, 7, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "1.e+0",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 5, 5, 5, idl.TokenTypeFloatDecimal, "1.e+0"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "6.67428e-11",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 11, 11, 11, idl.TokenTypeFloatDecimal, "6.67428e-11"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 11, 11, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
		},
		{
			input: "1E6",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 3, 3, 3, idl.TokenTypeFloatDecimal, "1E6"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 3, 3, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: ".25",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 3, 3, 3, idl.TokenTypeFloatDecimal, ".25"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 3, 3, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: ".12345E+5",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 9, 9, 9, idl.TokenTypeFloatDecimal, ".12345E+5"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 9, 9, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "1_5.",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 4, 4, 4, idl.TokenTypeFloatDecimal, "1_5."),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 4, 4, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0.15e+0_2",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 9, 9, 9, idl.TokenTypeFloatDecimal, "0.15e+0_2"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 9, 9, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
		},
		{
			input: "0x1p-2",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 6, 6, 6, idl.TokenTypeFloatHex, "0x1p-2"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 6, 6, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0x2.p10",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 7, 7, 7, idl.TokenTypeFloatHex, "0x2.p10"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 7, 7, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
		},
		{
			input: "0x1.Fp+0",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 8, 8, 8, idl.TokenTypeFloatHex, "0x1.Fp+0"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 8, 8, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0X.8p-0",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 7, 7, 7, idl.TokenTypeFloatHex, "0X.8p-0"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 7, 7, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "0X_1FFFP-16",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 11, 11, 11, idl.TokenTypeFloatHex, "0X_1FFFP-16"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 11, 11, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: `0x"FFF FFF FFF"`,
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 15, 15, 11, idl.TokenTypeData, "FFF FFF FFF"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 15, 15, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			name:  "text with newlines",
			input: "\"FFF FFF FFF asdjfh\n\n\"",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newToken(1, 2, 0, 3, 1, 20, idl.TokenTypeText, "FFF FFF FFF asdjfh\n\n"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(3, 1, 20, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			name:  "text with escaped characters",
			input: "\"FFF FFF FFF\\\" \\n asdjfh\n\n\"",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newToken(1, 2, 2, 3, 1, 27, idl.TokenTypeText, "FFF FFF FFF\\\" \\n asdjfh\n\n"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(3, 1, 27, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			name:  "prose with newlines",
			input: "`FFF FFF FFF asdjfh\n\n`",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newToken(1, 2, 2, 3, 1, 20, idl.TokenTypeProse, "FFF FFF FFF asdjfh\n\n"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(3, 1, 20, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			name:  "prose with escaped characters",
			input: "`FFF FFF FFF\\` \\n asdjfh\n\n`",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newToken(1, 2, 2, 3, 1, 27, idl.TokenTypeProse, "FFF FFF FFF\\` \\n asdjfh\n\n"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(3, 1, 27, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "// comment that ends in EOF",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 27, 27, 25, idl.TokenTypeComment, " comment that ends in EOF"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 27, 27, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			name:  "comment that ends in newline",
			input: "// comment that ends in newline\n",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 31, 31, 29, idl.TokenTypeComment, " comment that ends in newline"),
					err:   nil,
				},
				{
					token: newToken(1, 32, 32, 2, 1, 33, idl.TokenTypeNewline, "\n"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(2, 1, 28, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "/* comment block that ends in EOF",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 33, 33, 31, idl.TokenTypeComment, " comment block that ends in EOF"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 33, 33, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			name:  "comment block that ends in terminal",
			input: "/* comment block that ends in newline then terminal\n*/",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newToken(1, 2, 2, 2, 2, 53, idl.TokenTypeComment, " comment block that ends in newline then terminal\n"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(2, 2, 53, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "_",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeIdentifier, "_"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 1, 1, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "_abc_1234",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 9, 9, 9, idl.TokenTypeIdentifier, "_abc_1234"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 9, 9, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "______abc_1234______",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 20, 20, 20, idl.TokenTypeIdentifier, "______abc_1234______"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 20, 20, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "ABC1234",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 7, 7, 7, idl.TokenTypeIdentifier, "ABC1234"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 7, 7, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "@$*()+_-{}[]:.,<>~!%^&/?';|",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeAt, "@"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 2, 2, 1, idl.TokenTypeDollar, "$"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 3, 3, 1, idl.TokenTypeStar, "*"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 4, 4, 1, idl.TokenTypeParenOpen, "("),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 5, 5, 1, idl.TokenTypeParenClose, ")"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 6, 6, 1, idl.TokenTypePlus, "+"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 7, 7, 1, idl.TokenTypeIdentifier, "_"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 8, 8, 1, idl.TokenTypeMinus, "-"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 9, 9, 1, idl.TokenTypeCurlyOpen, "{"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 10, 10, 1, idl.TokenTypeCurlyClose, "}"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 11, 11, 1, idl.TokenTypeSquareOpen, "["),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 12, 12, 1, idl.TokenTypeSquareClose, "]"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 13, 13, 1, idl.TokenTypeColon, ":"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 14, 14, 1, idl.TokenTypeDot, "."),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 15, 15, 1, idl.TokenTypeComma, ","),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 16, 16, 1, idl.TokenTypeAngleOpen, "<"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 17, 17, 1, idl.TokenTypeAngleClose, ">"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 18, 18, 1, idl.TokenTypeTilde, "~"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 19, 19, 1, idl.TokenTypeExclamation, "!"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 20, 20, 1, idl.TokenTypePercent, "%"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 21, 21, 1, idl.TokenTypeCaret, "^"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 22, 22, 1, idl.TokenTypeAmpersand, "&"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 23, 23, 1, idl.TokenTypeSlash, "/"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 24, 24, 1, idl.TokenTypeQuestion, "?"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 25, 25, 1, idl.TokenTypeSquote, "'"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 26, 26, 1, idl.TokenTypeSemicolon, ";"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 27, 27, 1, idl.TokenTypePipe, "|"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 27, 27, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "1+2",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeIntegerDecimal, "1"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 2, 2, 1, idl.TokenTypePlus, "+"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 3, 3, 1, idl.TokenTypeIntegerDecimal, "2"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 3, 3, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "_",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeIdentifier, "_"),
					err:   nil,
				},
				{
					token: newTokenLineSpan(1, 1, 1, 0, idl.TokenTypeEOF, ""),
					err:   nil,
				},
			},
			verifyLineCol: true,
		},
		{
			input: "&&&",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 2, 2, 2, idl.TokenTypeBinAnd, "&&"),
				},
				{
					token: newTokenLineSpan(1, 3, 3, 1, idl.TokenTypeAmpersand, "&"),
				},
				{
					token: newTokenLineSpan(1, 3, 3, 0, idl.TokenTypeEOF, ""),
				},
			},
			verifyLineCol: true,
		},
		{
			input: "|||",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 2, 2, 2, idl.TokenTypeBinOr, "||"),
				},
				{
					token: newTokenLineSpan(1, 3, 3, 1, idl.TokenTypePipe, "|"),
				},
				{
					token: newTokenLineSpan(1, 3, 3, 0, idl.TokenTypeEOF, ""),
				},
			},
			verifyLineCol: true,
		},
		{
			input: ">>=<<=",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 1, 1, 1, idl.TokenTypeAngleClose, ">"),
				},
				{
					token: newTokenLineSpan(1, 3, 3, 2, idl.TokenTypeGreaterEqual, ">="),
				},
				{
					token: newTokenLineSpan(1, 4, 4, 1, idl.TokenTypeAngleOpen, "<"),
				},
				{
					token: newTokenLineSpan(1, 6, 6, 2, idl.TokenTypeLesserEqual, "<="),
				},
				{
					token: newTokenLineSpan(1, 6, 6, 0, idl.TokenTypeEOF, ""),
				},
			},
			verifyLineCol: true,
		},
		{
			input: "++=--=**=/-/=",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 1, 1, 1, idl.TokenTypePlus, "+"),
				},
				{
					token: newTokenLineSpan(1, 3, 3, 2, idl.TokenTypePlusEqual, "+="),
				},
				{
					token: newTokenLineSpan(1, 4, 4, 1, idl.TokenTypeMinus, "-"),
				},
				{
					token: newTokenLineSpan(1, 6, 6, 2, idl.TokenTypeMinusEqual, "-="),
				},
				{
					token: newTokenLineSpan(1, 7, 7, 1, idl.TokenTypeStar, "*"),
				},
				{
					token: newTokenLineSpan(1, 9, 9, 2, idl.TokenTypeMultiplyEqual, "*="),
				},
				{
					token: newTokenLineSpan(1, 10, 10, 1, idl.TokenTypeSlash, "/"),
				},
				{
					token: newTokenLineSpan(1, 11, 11, 1, idl.TokenTypeMinus, "-"),
				},
				{
					token: newTokenLineSpan(1, 13, 13, 2, idl.TokenTypeDivideEqual, "/="),
				},
				{
					token: newTokenLineSpan(1, 13, 13, 0, idl.TokenTypeEOF, ""),
				},
			},
			verifyLineCol: true,
		},
		{
			input: "===!=",
			expected: []struct {
				token *idl.Token
				err   error
			}{
				{
					token: newTokenLineSpan(1, 2, 2, 2, idl.TokenTypeComparison, "=="),
				},
				{
					token: newTokenLineSpan(1, 3, 3, 1, idl.TokenTypeEqual, "="),
				},
				{
					token: newTokenLineSpan(1, 5, 5, 2, idl.TokenTypeNotComparison, "!="),
				},
				{
					token: newTokenLineSpan(1, 5, 5, 0, idl.TokenTypeEOF, ""),
				},
			},
			verifyLineCol: true,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		name := testCase.name
		if name == "" {
			name = testCase.input
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			input := fs.NewFileString("/test", testCase.input, idl.FileKindMicroglot)
			rep := exc.NewReporter(nil)
			lexer := &LexerMicroglot{
				reporter: rep,
			}
			lexerFile, err := lexer.Lex(ctx, input)
			require.Nil(t, err)
			stream, err := lexerFile.Tokens(ctx)
			require.Nil(t, err)
			for _, expectation := range testCase.expected {
				tok := stream.Next(ctx)
				if !tok.IsPresent() {
					err := stream.Close(ctx)
					if err == nil && expectation.err == nil && expectation.token.Type == idl.TokenTypeEOF {
						break
					}
					if expectation.err == nil {
						require.FailNow(t, "token stream ended unexpectedly", rep.Reported())
					}
					if err == nil {
						require.FailNow(t, "expected to fail but got no error", "expected: %s", expectation.err)
					}
					require.Equal(t, expectation.err, err)
					break
				}
				if tok.Value().Type != expectation.token.Type {
					t.Errorf("type: expected: %s -- got %s", expectation.token.Type, tok.Value().Type)
				}
				if tok.Value().Value != expectation.token.Value {
					exp := strings.ReplaceAll(expectation.token.Value, "\n", "<N>")
					exp = strings.ReplaceAll(exp, "\r", "<R>")
					act := strings.ReplaceAll(tok.Value().Value, "\n", "<N>")
					act = strings.ReplaceAll(act, "\r", "<R>")
					t.Errorf("value: expected: %s -- got %s", exp, act)
				}
				if testCase.verifyLineCol {
					if tok.Value().Span.Start.Line != expectation.token.Span.Start.Line {
						t.Errorf("line start: expected: %d -- got %d", expectation.token.Span.Start.Line, tok.Value().Span.Start.Line)
					}
					if tok.Value().Span.End.Line != expectation.token.Span.End.Line {
						t.Errorf("line end: expected: %d -- got %d", expectation.token.Span.End.Line, tok.Value().Span.End.Line)
					}
					if tok.Value().Span.Start.Column != expectation.token.Span.Start.Column {
						t.Errorf("col start: expected: %d -- got %d", expectation.token.Span.Start.Column, tok.Value().Span.Start.Column)
					}
					if tok.Value().Span.End.Column != expectation.token.Span.End.Column {
						t.Errorf("col end: expected: %d -- got %d", expectation.token.Span.End.Column, tok.Value().Span.End.Column)
					}
				}
			}
		})
	}
}

var tokenTypeEscape idl.TokenType

// BenchmarkLexer is included mostly for future analysis of the lexer
// performance such as minimizing allocations.
func BenchmarkLexer(b *testing.B) {
	ctx := context.Background()
	input := fs.NewFileString("/test", desc, idl.FileKindMicroglot)
	rep := exc.NewReporter(nil)
	lexer := &LexerMicroglot{
		reporter: rep,
	}
	var err error
	var tok optional.Optional[*idl.Token]
	var tt idl.TokenType
	lexerFile, _ := lexer.Lex(ctx, input)
	stream, _ := lexerFile.Tokens(ctx)
	b.ResetTimer()
	for x := 0; x < b.N; x = x + 1 {
		for tok = stream.Next(ctx); tok.IsPresent(); tok = stream.Next(ctx) {
			tt = tok.Value().Type
		}
		if err = stream.Close(ctx); err != nil {
			b.Fatal(err)
		}
	}
	tokenTypeEscape = tt
}

var desc = `syntax = "mglot0"
module = @0x1

struct Exception {
    Source :ExceptionSource @1
    Range :ExceptionRange @2
    Code :ExceptionCode @3
    Extended :UInt16 @4
    Message :Text @5
    Metadata :Map<:Text, :Text> @6
} @0x1
`
