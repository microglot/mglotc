// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris

package compiler

import (
	"os"
	"path/filepath"
	"strings"
)

func getDefaultRoots(lookup func(string) (string, bool)) []string {
	xdgDirs, ok := lookup("XDG_DATA_DIRS")
	if !ok {
		xdgDirs = "/usr/local/share/:/usr/share/"
	}
	dataDirs := strings.Split(xdgDirs, ":")
	for offset, dataDir := range dataDirs {
		p := filepath.Join(dataDir, "microglot")
		p = os.Expand(p, func(s string) string {
			v, _ := lookup(s)
			return v
		})
		dataDirs[offset] = p
	}
	return dataDirs
}
