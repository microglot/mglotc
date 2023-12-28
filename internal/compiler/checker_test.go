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

type CheckerTestFile struct {
	kind     idl.FileKind
	uri      string
	contents string
}

func TestChecker(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name             string
		files            []CheckerTestFile
		expectCheckError bool
	}{
		{
			name: "nothing",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: `syntax = "microglot0"`,
				},
			},
			expectCheckError: false,
		},
		{
			name: "use annotation as field type",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct Foo { bar :Protobuf.Package }\n",
				},
			},
			expectCheckError: true,
		},
		{
			name: "use annotation as const type",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nconst Foo :Protobuf.Package = 1\n",
				},
			},
			expectCheckError: true,
		},

		{
			name: "allowed constant values",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nconst Foo :Int32 = 32\nconst Bar :Int32 = Foo\nconst Baz :Int32 = -Bar\nconst Barney :Int32 = (Foo + Bar)\n",
				},
			},
			expectCheckError: false,
		},
		{
			name: "default values are constants",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct Foo { bar :Int32 = [] }\n",
				},
			},
			expectCheckError: true,
		},
		{
			name: "annotation application arguments are constants",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nannotation Foo(struct) :Text\nstruct Bar {} $(Foo([]))\n",
				},
			},
			expectCheckError: true,
		},

		{
			name: "list literals in annotation applications",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct FooArgument { l :List<:Text> }\nannotation Foo(struct) :FooArgument\nstruct Bar {} $(Foo({l: []}))\n",
				},
			},
			expectCheckError: false,
		},
		{
			name: "list literals in annotation applications (non-list literal)",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct FooArgument { l :List<:Text> }\nannotation Foo(struct) :FooArgument\nstruct Bar {} $(Foo({l: 32}))\n",
				},
			},
			expectCheckError: true,
		},
		{
			name: "list literals in annotation applications (list literal of wrong type)",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct FooArgument { l :List<:Text> }\nannotation Foo(struct) :FooArgument\nstruct Bar {} $(Foo({l: [32]}))\n",
				},
			},
			expectCheckError: true,
		},

		{
			name: "presence literals in annotation applications (present)",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct FooArgument { p :Presence<:Text> }\nannotation Foo(struct) :FooArgument\nstruct Bar {} $(Foo({p: \"present\"}))\n",
				},
			},
			expectCheckError: false,
		},
		{
			name: "presence literals in annotation applications (absent)",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct FooArgument { p :Presence<:Text> }\nannotation Foo(struct) :FooArgument\nstruct Bar {} $(Foo({}))\n",
				},
			},
			expectCheckError: false,
		},
		{
			name: "presence literals in annotation applications (present but wrong type)",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct FooArgument { p :Presence<:Text> }\nannotation Foo(struct) :FooArgument\nstruct Bar {} $(Foo({p: 32}))\n",
				},
			},
			expectCheckError: true,
		},

		{
			name: "struct literals in annotation applications",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct Foo { bar :Text }\nannotation Bar(struct) :Foo\nstruct Baz {} $(Bar({ bar: \"ASDF\" }))\n",
				},
			},
			expectCheckError: false,
		},
		{
			name: "struct literals in annotation applications (missing field)",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct Foo { bar :Text }\nannotation Bar(struct) :Foo\nstruct Baz {} $(Bar({ }))\n",
				},
			},
			expectCheckError: false,
		},
		{
			name: "struct literals in annotation applications (spurious field)",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct Foo { bar :Text }\nannotation Bar(struct) :Foo\nstruct Baz {} $(Bar({ barney: \"ASDF\", bar: \"ASDF\" }))\n",
				},
			},
			expectCheckError: true,
		},
		{
			name: "struct literals in annotation applications (field value has wrong type)",
			files: []CheckerTestFile{
				{
					kind:     idl.FileKindMicroglot,
					uri:      "/test.mgdl",
					contents: "syntax = \"microglot0\"\nstruct Foo { bar :Text }\nannotation Bar(struct) :Foo\nstruct Baz {} $(Bar({ bar: 99 }))\n",
				},
			},
			expectCheckError: true,
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

			protobufDescriptor, err := subcompilers[idl.FileKindMicroglot].CompileFile(ctx, r, fs.NewFileString("/protobuf.mgdl", idl.PROTOBUF_IDL, idl.FileKindMicroglot), false, false)
			require.NoError(t, err, "/protobuf.mgdl", r.Reported())

			parsedDescriptors := make([]*proto.Module, 0, len(testCase.files))
			for i, f := range files {
				d, err := subcompilers[f.Kind(ctx)].CompileFile(ctx, r, f, false, false)
				require.NoError(t, err, testCase.files[i].uri, r.Reported())
				parsedDescriptors = append(parsedDescriptors, d)
			}

			symbols := globalSymbolTable{}
			protobufDescriptor = completeUIDs(*protobufDescriptor)
			err = symbols.collect(*protobufDescriptor, r)
			require.NoError(t, err, "/protobuf.mgdl", r.Reported())
			protobufDescriptor, err = link(*protobufDescriptor, &symbols, r)
			require.NoError(t, err, "/protobuf.mgdl", r.Reported())

			completedDescriptors := make([]*proto.Module, 0, len(parsedDescriptors))
			for i, parsedDescriptor := range parsedDescriptors {
				completedDescriptor := completeUIDs(*parsedDescriptor)
				err := symbols.collect(*completedDescriptor, r)
				require.NoError(t, err, testCase.files[i].uri, r.Reported())
				completedDescriptors = append(completedDescriptors, completedDescriptor)
			}

			linkedDescriptors := make([]*proto.Module, 0, len(completedDescriptors))
			for i, completedDescriptor := range completedDescriptors {
				if completedDescriptor != nil {
					linkedDescriptor, err := link(*completedDescriptor, &symbols, r)
					require.NoError(t, err, testCase.files[i].uri, r.Reported())
					linkedDescriptors = append(linkedDescriptors, linkedDescriptor)
				}
			}

			linkedDescriptors = append(linkedDescriptors, protobufDescriptor)

			image := idl.Image{
				Modules: linkedDescriptors,
			}

			optimize(&image)
			check(&image, r)
			if testCase.expectCheckError {
				require.NotEmpty(t, r.Reported())
			} else {
				require.Empty(t, r.Reported())
			}
		})
	}
}
