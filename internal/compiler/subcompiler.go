// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package compiler

import (
	"context"

	"gopkg.microglot.org/mglotc/internal/exc"
	"gopkg.microglot.org/mglotc/internal/idl"
	"gopkg.microglot.org/mglotc/internal/proto"
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
