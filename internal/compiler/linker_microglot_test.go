package compiler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/fs"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

type LinkerTestFile struct {
	kind               idl.FileKind
	uri                string
	contents           string
	expectCollectError bool
	expectLinkError    bool
}

func TestLinker(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name  string
		files []LinkerTestFile
	}{
		{
			name: "nothing",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           `syntax = "microglot0"`,
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "correctly linked type",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst foo :Bool = true",
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "duplicate declaration",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst foo :Bool = true\nconst foo :String = \"asdf\"\n",
					expectCollectError: true,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "duplicate protobuf declaration",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindProtobuf,
					uri:                "/test1.proto",
					contents:           "syntax = \"proto3\"; message Foo {}",
					expectCollectError: false,
					expectLinkError:    false,
				},
				{
					kind:               idl.FileKindProtobuf,
					uri:                "/test2.proto",
					contents:           "syntax = \"proto3\"; message Foo {}",
					expectCollectError: true,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "unknown type",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst boo :Boolean = bar",
					expectCollectError: false,
					expectLinkError:    true,
				},
			},
		},
		{
			name: "unknown import",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nimport \"/nonexistent.mgdl\" as n",
					expectCollectError: false,
					expectLinkError:    true,
				},
			},
		},
		{
			name: "module UID collision",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/one.mgdl",
					contents:           "syntax = \"microglot0\"\nmodule = @10",
					expectCollectError: false,
					expectLinkError:    false,
				},
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/two.mgdl",
					contents:           "syntax = \"microglot0\"\nmodule = @10",
					expectCollectError: true,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "type UID collision",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst x :Bool = true @10\nconst y :Bool = false @10",
					expectCollectError: true,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "correctly linked valueidentifier (type)",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst x :Bool = true\nconst y :Bool = x",
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "unknown valueidentifier (type)",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst y :Bool = x",
					expectCollectError: false,
					expectLinkError:    true,
				},
			},
		},
		{
			name: "correctly linked valueidentifier (attribute)",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nenum x { y }\nconst z :Bool = x.y",
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "unknown valueidentifier (attribute)",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nenum x { y }\nconst z :Bool = x.a",
					expectCollectError: false,
					expectLinkError:    true,
				},
			},
		},
		{
			name: "empty protobuf",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindProtobuf,
					uri:                "/test.proto",
					contents:           "syntax = \"proto3\";",
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "microglot reference to imported protobuf",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindProtobuf,
					uri:                "/test.proto",
					contents:           "syntax = \"proto3\"; message Foo {}",
					expectCollectError: false,
					expectLinkError:    false,
				},
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nimport \"/test.proto\" as p\nconst z :p.Foo = 0",
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "protobuf reference to imported microglot",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindMicroglot,
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nstruct Foo {}",
					expectCollectError: false,
					expectLinkError:    false,
				},
				{
					kind:               idl.FileKindProtobuf,
					uri:                "/test.proto",
					contents:           "syntax = \"proto3\";\npackage test;\nimport \"/test.mgdl\";\nmessage Bar { Foo x = 1; }",
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "protobuf namespace rules",
			files: []LinkerTestFile{
				{
					kind:               idl.FileKindProtobuf,
					uri:                "/outer.proto",
					contents:           "syntax = \"proto3\";\npackage outer;\nimport \"/inner.proto\";\nmessage Y { .outer.inner.X field = 1; outer.inner.X field2 = 2; inner.X field3 = 3; }",
					expectCollectError: false,
					expectLinkError:    false,
				},
				{
					kind:               idl.FileKindProtobuf,
					uri:                "/inner.proto",
					contents:           "syntax = \"proto3\";\npackage outer.inner;\nmessage X {};",
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
	}

	subcompilers := DefaultSubCompilers()
	ctx := context.Background()
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			r := exc.NewReporter(nil)
			files := make([]idl.File, 0, len(testCase.files))
			for _, f := range testCase.files {
				files = append(files, fs.NewFileString(f.uri, f.contents, f.kind))
			}

			parsedDescriptors := make([]*proto.Module, 0, len(testCase.files))
			for i, f := range files {
				d, err := subcompilers[f.Kind(ctx)].CompileFile(ctx, r, f, false, false)
				require.NoError(t, err, testCase.files[i].uri, r.Reported())
				parsedDescriptors = append(parsedDescriptors, d)
			}

			symbols := globalSymbolTable{}
			completedDescriptors := make([]*proto.Module, 0, len(parsedDescriptors))
			for i, parsedDescriptor := range parsedDescriptors {
				completedDescriptor := completeUIDs(*parsedDescriptor)
				err := symbols.collect(*completedDescriptor, r)
				if testCase.files[i].expectCollectError {
					require.Error(t, err, testCase.files[i].uri)
				} else {
					require.NoError(t, err, testCase.files[i].uri, r.Reported())
				}
				completedDescriptors = append(completedDescriptors, completedDescriptor)
			}

			linkedDescriptors := make([]*proto.Module, 0, len(completedDescriptors))
			if len(r.Reported()) == 0 {
				for i, completedDescriptor := range completedDescriptors {
					if completedDescriptor != nil {
						linkedDescriptor, err := link(*completedDescriptor, &symbols, r)
						if testCase.files[i].expectLinkError {
							require.Error(t, err, testCase.files[i].uri)
						} else {
							require.NoError(t, err, testCase.files[i].uri, r.Reported())
						}
						linkedDescriptors = append(linkedDescriptors, linkedDescriptor)
					}
				}
			}

			if len(r.Reported()) == 0 {
				for i, linkedDescriptor := range linkedDescriptors {
					walkModule(linkedDescriptor, func(node interface{}) {
						switch n := node.(type) {
						case *proto.TypeSpecifier:
							require.NotNil(t, n.Reference, testCase.files[i].uri)
							require.NotZero(t, *n, testCase.files[i].uri)
						case *proto.ValueIdentifier:
							require.NotNil(t, n.Reference, testCase.files[i].uri)
							require.NotZero(t, *n, testCase.files[i].uri)
						}
					})
				}
			}
		})
	}
}
