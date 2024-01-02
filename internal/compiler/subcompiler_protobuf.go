package compiler

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/bufbuild/protocompile/options"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"

	"gopkg.microglot.org/compiler.go/internal/compiler/protobuf"
	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

type SubCompilerProtobuf struct{}

func (self *SubCompilerProtobuf) CompileFile(ctx context.Context, r exc.Reporter, file idl.File, dumpTokens bool, dumpTree bool) (*proto.Module, error) {
	b, err := file.Body(ctx)
	if dumpTokens {
		return nil, errors.New("token stream dumping isn't implemented for protobuf, sorry")
	}
	if err != nil {
		return nil, r.Report(exc.WrapUnknown(exc.Location{URI: file.Path(ctx)}, err))
	}
	defer b.Close(ctx)
	h := reporter.NewHandler(&protoReporter{Reporter: r})
	node, err := parser.Parse(file.Path(ctx), &fileBodyIO{ctx: ctx, body: b}, h)
	if err != nil {
		return nil, err
	}
	if dumpTree {
		// TODO 2023.08.14: no stringer implementation for the protoc AST; this output is not useful.
		fmt.Println(node)
	}
	result, err := parser.ResultFromAST(node, true, h)
	if err != nil {
		return nil, err
	}

	_, err = options.InterpretUnlinkedOptions(result)
	if err != nil {
		return nil, err
	}

	module, err := protobuf.FromFileDescriptorProto(result.FileDescriptorProto())
	if err != nil {
		return nil, err
	}
	return module, nil
}

type protoReporter struct {
	Reporter exc.Reporter
}

func (self *protoReporter) Error(e reporter.ErrorWithPos) error {
	pos := e.GetPosition()
	loc := exc.Location{
		URI: pos.Filename,
		Location: idl.Location{
			Line:   int32(pos.Line),
			Column: int32(pos.Col),
			Offset: int64(pos.Offset),
		},
	}
	return self.Reporter.Report(exc.Wrap(loc, exc.CodeProtobufParseError, e))
}

func (self *protoReporter) Warning(e reporter.ErrorWithPos) {
	_ = self.Error(e)
}

type fileBodyIO struct {
	ctx  context.Context
	body idl.FileBody
}

func (self *fileBodyIO) Read(p []byte) (int, error) {
	b, err := self.body.Read(self.ctx, int32(len(p)))
	if err != nil && !errors.Is(err, io.EOF) {
		return len(b), err
	}
	copy(p, b)
	if errors.Is(err, io.EOF) {
		return len(b), io.EOF
	}
	return len(b), nil
}

func (self *fileBodyIO) Close() error {
	return self.body.Close(self.ctx)
}
