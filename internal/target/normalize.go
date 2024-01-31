package target

import (
	"net/url"
	"path/filepath"
)

// Normalize processes a given import or compile target and converts it into a
// standard form.
//
// The compiler allows targets to be any valid URI or file path. When the target
// is a file path or a file URI then we convert the paths to an absolute form.
// All non-file URIs are left as-is with the expectation that they will be
// handled by some other implementation
func Normalize(target string) string {
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "" && u.Scheme != "file") {
		return target
	}
	if u.Scheme == "file" {
		target = u.Path
	}
	if !filepath.IsAbs(target) {
		return filepath.Join("/", target)
	}
	return target
}
