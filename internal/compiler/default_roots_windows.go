// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package compiler

import (
	"path/filepath"
)

func getDefaultRoots(lookup func(string) (string, bool)) []string {
	userprofile, _ := lookup("USERPROFILE")
	systemdrive, _ := lookup("SystemDrive")

	dataDirs := []string{
		filepath.Join(userprofile, "AppData", "Local", "microglot", "idl"),
		filepath.Join(systemdrive, "ProgramData", "microglot", "idl"),
	}

	return dataDirs
}
