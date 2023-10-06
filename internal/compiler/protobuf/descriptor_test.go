package protobuf

import (
	"strings"
	"testing"

	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/stretchr/testify/require"

	"gopkg.microglot.org/compiler.go/internal/proto"
)

func TestDescriptor(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected *proto.Module
	}{
		{
			name:  "bare minimum",
			input: "syntax = \"proto3\";",
			expected: &proto.Module{
				UID: 1449310910991872227,
			},
		},
		{
			name:  "message -> struct",
			input: "syntax = \"proto3\";\nmessage Foo {}\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{},
					},
				},
			},
		},
		{
			name:  "nested message -> struct",
			input: "syntax = \"proto3\";\nmessage Foo { message Bar { message Baz {} } }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo_Bar",
						},
						Reference: &proto.TypeReference{},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo_Bar_Baz",
						},
						Reference: &proto.TypeReference{},
					},
				},
			},
		},
		{
			name:  "enum -> enum",
			input: "syntax = \"proto3\";\nenum Foo { X = 0; }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Enums: []*proto.Enum{
					&proto.Enum{
						Name:      "Foo",
						Reference: &proto.TypeReference{},
					},
				},
			},
		},
		{
			name:  "nested enum -> enum",
			input: "syntax = \"proto3\";\nmessage Foo { message Bar { enum Baz { X = 0; } } }\n",
			expected: &proto.Module{
				UID: 1449310910991872227,
				Structs: []*proto.Struct{
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo",
						},
						Reference: &proto.TypeReference{},
					},
					&proto.Struct{
						Name: &proto.TypeName{
							Name: "Foo_Bar",
						},
						Reference: &proto.TypeReference{},
					},
				},
				Enums: []*proto.Enum{
					&proto.Enum{
						Name:      "Foo_Bar_Baz",
						Reference: &proto.TypeReference{},
					},
				},
			},
		},
	}
	for _, testCase := range testCases {
		name := testCase.name
		if name == "" {
			name = testCase.input
		}
		t.Run(name, func(t *testing.T) {
			h := reporter.NewHandler(reporter.NewReporter(
				func(err reporter.ErrorWithPos) error { return nil },
				func(err reporter.ErrorWithPos) { return },
			))
			ast, err := parser.Parse("", strings.NewReader(testCase.input), h)
			require.Nil(t, err)

			result, err := parser.ResultFromAST(ast, true, h)
			require.Nil(t, err)

			module, err := FromFileDescriptorProto(result.FileDescriptorProto())
			require.Nil(t, err)

			require.Equal(t, testCase.expected, module)
		})
	}
}
