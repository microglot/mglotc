// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"bufio"
	"context"
	"io"
	"strings"

	"gopkg.microglot.org/mglotc/internal/idl"
)

// NewFileString wraps static string content in idl.File.
func NewFileString(path string, content string, kind idl.FileKind) idl.File {
	return NewFileFN(path, func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(content)), nil
	}, kind)
}

type fileIOFunc struct {
	path string
	kind idl.FileKind
	body func() (io.ReadCloser, error)
}

// NewFileFN is intended to wrap actual file based content in the idl.File
// interface. The given body function is used each time there is a call to the
// idl.File.Body method so it must return a new io.ReadCloser handle. There is
// no guarantee that only on output of the body function will be used at a time.
func NewFileFN(path string, body func() (io.ReadCloser, error), kind idl.FileKind) idl.File {
	return &fileIOFunc{
		path: path,
		kind: kind,
		body: body,
	}
}

func (f *fileIOFunc) Path(ctx context.Context) string {
	return f.path
}
func (f *fileIOFunc) Kind(ctx context.Context) idl.FileKind {
	return f.kind
}
func (f *fileIOFunc) Body(ctx context.Context) (idl.FileBody, error) {
	rc, err := f.body()
	if err != nil {
		return nil, err
	}
	rcb := bufio.NewReader(rc)
	rcbc := &bufioReaderCloser{
		Reader: rcb,
		Closer: rc,
	}
	return bodyFromIO(rcbc), nil
}

type bufioReaderCloser struct {
	*bufio.Reader
	io.Closer
}
