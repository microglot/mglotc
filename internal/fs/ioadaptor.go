// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"io"

	"gopkg.microglot.org/mglotc/internal/exc"
	"gopkg.microglot.org/mglotc/internal/idl"
)

func bodyFromIO(v io.ReadCloser) idl.FileBody {
	return &ioFileBody{rc: v}
}

type ioFileBody struct {
	rc io.ReadCloser
	b  []byte
}

func (self *ioFileBody) Read(ctx context.Context, size int32) ([]byte, error) {
	if len(self.b) < int(size) {
		self.b = make([]byte, size)
	}
	count, err := self.rc.Read(self.b[:size])
	if err != nil && err != io.EOF {
		return nil, exc.WrapUnknown(exc.Location{}, err)
	}
	if err == io.EOF {
		return self.b[:count], exc.Wrap(exc.Location{}, exc.CodeEOF, err)
	}
	return self.b[:count], nil
}

func (self *ioFileBody) Close(ctx context.Context) error {
	return self.rc.Close()
}
