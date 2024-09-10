// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package iter

import (
	"bufio"
	"context"
	"errors"
	"io"
	"unicode/utf8"

	"gopkg.microglot.org/mglotc/internal/idl"
	"gopkg.microglot.org/mglotc/internal/optional"
)

// NewUnicodeFileBody converts a FileBody into an iterator of code points.
func NewUnicodeFileBody(b idl.FileBody) idl.Iterator[idl.CodePoint] {
	return NewUnicodeFileBodyCtx(context.Background(), b)
}

// NewUnicodeFileBodyCtx is the same as NewUnicodeFileBody but uses the given
// context for all read operations for cancellation or other purposes.
func NewUnicodeFileBodyCtx(ctx context.Context, b idl.FileBody) idl.Iterator[idl.CodePoint] {
	return newFileBody(ctx, b)
}

type fileBody struct {
	readCloser io.ReadCloser
	scanner    *bufio.Scanner
}

func newFileBody(ctx context.Context, r idl.FileBody) *fileBody {
	rc := &fileBodyIO{
		ctx:  ctx,
		body: r,
	}
	scanner := bufio.NewScanner(rc)
	scanner.Split(bufio.ScanRunes)
	return &fileBody{
		readCloser: rc,
		scanner:    scanner,
	}
}

func (f *fileBody) Next(ctx context.Context) optional.Optional[idl.CodePoint] {
	ok := f.scanner.Scan()
	if !ok {
		return optional.None[idl.CodePoint]()
	}
	r, _ := utf8.DecodeRune(f.scanner.Bytes())
	return optional.Some(idl.CodePoint(r))
}

func (f *fileBody) Close(context.Context) error {
	_ = f.readCloser.Close()
	err := f.scanner.Err()
	if err != nil {
		return err
	}
	return nil
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
