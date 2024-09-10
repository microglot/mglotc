// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package compiler

import (
	"path/filepath"

	"gopkg.microglot.org/compiler.go/internal/fs"
	"gopkg.microglot.org/compiler.go/internal/idl"
)

func NewDefaultFS(lookup func(string) (string, bool)) (idl.FileSystem, error) {
	roots := getDefaultRoots(lookup)
	f := make(fs.FileSystemMulti, 0, len(roots))
	for _, root := range roots {
		absRoot, errAbs := filepath.Abs(root)
		if errAbs != nil {
			return nil, errAbs
		}
		rf, err := fs.NewFileSystemLocal(absRoot)
		if err != nil {
			return nil, err
		}
		f = append(f, rf)
	}
	return f, nil
}
