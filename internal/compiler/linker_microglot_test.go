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

func TestLinker(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name  string
		files []struct {
			uri                string
			contents           string
			expectCollectError bool
			expectLinkError    bool
		}
	}{
		{
			name: "nothing",
			files: []struct {
				uri                string
				contents           string
				expectCollectError bool
				expectLinkError    bool
			}{
				{
					uri:                "/test.mgdl",
					contents:           `syntax = "microglot0"`,
					expectCollectError: false,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "duplicate declaration",
			files: []struct {
				uri                string
				contents           string
				expectCollectError bool
				expectLinkError    bool
			}{
				{
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst foo :Boolean = true\nconst foo :String = \"asdf\"\n",
					expectCollectError: true,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "unknown type",
			files: []struct {
				uri                string
				contents           string
				expectCollectError bool
				expectLinkError    bool
			}{
				{
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst boo :Boolean = bar",
					expectCollectError: false,
					expectLinkError:    true,
				},
			},
		},
		{
			name: "unknown import",
			files: []struct {
				uri                string
				contents           string
				expectCollectError bool
				expectLinkError    bool
			}{
				{
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nimport \"/nonexistent.mgdl\" as n",
					expectCollectError: false,
					expectLinkError:    true,
				},
			},
		},
		{
			name: "module UID collision",
			files: []struct {
				uri                string
				contents           string
				expectCollectError bool
				expectLinkError    bool
			}{
				{
					uri:                "/one.mgdl",
					contents:           "syntax = \"microglot0\"\nmodule = @10",
					expectCollectError: false,
					expectLinkError:    false,
				},
				{
					uri:                "/two.mgdl",
					contents:           "syntax = \"microglot0\"\nmodule = @10",
					expectCollectError: true,
					expectLinkError:    false,
				},
			},
		},
		{
			name: "type UID collision",
			files: []struct {
				uri                string
				contents           string
				expectCollectError bool
				expectLinkError    bool
			}{
				{
					uri:                "/test.mgdl",
					contents:           "syntax = \"microglot0\"\nconst x :Bool = true @10\nconst y :Bool = false @10",
					expectCollectError: true,
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
				files = append(files, fs.NewFileString(f.uri, f.contents, idl.FileKindMicroglot))
			}

			parsed_descriptors := make([]*proto.Module, 0, len(testCase.files))
			for i, f := range files {
				d, err := subcompilers[f.Kind(ctx)].CompileFile(ctx, r, f, false, false)
				require.NoError(t, err, testCase.files[i].uri, r.Reported())
				parsed_descriptors = append(parsed_descriptors, d)
			}

			symbols := globalSymbolTable{}
			collected_descriptors := make([]*proto.Module, 0, len(parsed_descriptors))
			for i, parsed_descriptor := range parsed_descriptors {
				d, err := symbols.collect(*parsed_descriptor, r)
				if testCase.files[i].expectCollectError {
					require.Error(t, err, testCase.files[i].uri)
				} else {
					require.NoError(t, err, testCase.files[i].uri, r.Reported())
				}
				collected_descriptors = append(collected_descriptors, d)
			}

			if len(r.Reported()) == 0 {
				for i, collected_descriptor := range collected_descriptors {
					if collected_descriptor != nil {
						_, err := link(*collected_descriptor, &symbols, r)
						if testCase.files[i].expectLinkError {
							require.Error(t, err, testCase.files[i].uri)
						} else {
							require.NoError(t, err, testCase.files[i].uri, r.Reported())
						}
					}
				}
			}
		})
	}
}
