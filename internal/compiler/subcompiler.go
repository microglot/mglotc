package compiler

import (
	"context"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

type SubCompiler interface {
	CompileFile(ctx context.Context, r exc.Reporter, file idl.File, dumpTokens bool, dumpTree bool) (*proto.Module, error)
}

func DefaultSubCompilers() map[idl.FileKind]SubCompiler {
	scmicroglot := &SubCompilerMicroglot{}
	scproto := &SubCompilerProtobuf{}
	scidl := &SubCompilerIDL{
		Microglot: scmicroglot,
		Protobuf:  scproto,
	}
	return map[idl.FileKind]SubCompiler{
		idl.FileKindMicroglot: scidl,
		idl.FileKindProtobuf:  scproto,
		// TODO: Add deserializer support for the encoded file formats below.
		idl.FileKindMicroglotDescBinary: nil,
		idl.FileKindMicroglotDescJSON:   nil,
		idl.FileKindMicroglotDescProto:  nil,
		idl.FileKindProtobufDesc:        nil,
	}
}
