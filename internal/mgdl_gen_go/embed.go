package mgdl_gen_go

import (
	"fmt"

	"google.golang.org/protobuf/types/pluginpb"

	"gopkg.microglot.org/compiler.go/internal/idl"
)

func Embed(parameters string, image *idl.Image, targets []string) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	// the use of CodeGeneratorResponse_File is just for short-term convenience, here; don't hesitate to
	// switch it out for something better.
	files := []*pluginpb.CodeGeneratorResponse_File{}
	for _, target := range targets {
		targetURI := fmt.Sprintf("/%s", target)
		for _, module := range image.Modules {
			fmt.Printf("%s vs %s\n", targetURI, module.URI)
			if module.URI == targetURI {
				name := "mgdl.go"
				content := "hello, world"

				files = append(files, &pluginpb.CodeGeneratorResponse_File{
					Name:    &name,
					Content: &content,
				})
				// emit constants
				// emit sdks
				// append to files
			}
		}
	}
	return files, nil
}
