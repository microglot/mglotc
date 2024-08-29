// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
)

const (
	fileExt          = ".mgdl"      // Typical microglot IDL content
	fileDescExt      = ".mgdlbin"   // An IDL image in microglot binary format
	fileDescJSONExt  = ".mgdljson"  // An IDL image in microglot JSON format
	fileDescProtoExt = ".mgdlproto" // An IDL image in protobuf format
	protoExt         = ".proto"     // Typical protobuf IDL content
	protoDescExt     = ".protoset"  // A protobuf descriptor set in protobuf format
)

var knownExts = map[string]idl.FileKind{
	fileExt:          idl.FileKindMicroglot,
	fileDescExt:      idl.FileKindMicroglotDescBinary,
	fileDescJSONExt:  idl.FileKindMicroglotDescJSON,
	fileDescProtoExt: idl.FileKindMicroglotDescProto,
	protoExt:         idl.FileKindProtobuf,
	protoDescExt:     idl.FileKindProtobufDesc,
}

var _ idl.FileSystem = FileSystemMulti{}

// FileSystemMulti is an ordered set of FileSystem implementations that are
// tried in order. Note that this type does not implement write operations.
// Those must be performed on individual backends.
type FileSystemMulti []idl.FileSystem

func (r FileSystemMulti) Open(ctx context.Context, uri string) ([]idl.File, error) {
	for _, fs := range r {
		files, err := fs.Open(ctx, uri)
		if err != nil {
			continue
		}
		return files, nil
	}
	return nil, exc.New(exc.Location{URI: uri}, exc.CodeFileNotFound, fmt.Sprintf("could not open %s from any file system", uri))
}

func (r FileSystemMulti) Write(ctx context.Context, uri string, content string) error {
	return exc.New(exc.Location{URI: uri}, exc.CodeUnsuportedFileSystemOperation, "cannot write to a composite file system")
}

// FileFilter is a filter function type used to select which files to open when
// the path being opened is a directory. Implementations should return true if
// the file should be opened, false otherwise.
type FileFilter func(ctx context.Context, fname string) bool

type FileSystemLocalOption func(*fileSystemLocal)

// WithOptionFSFactory installs a custom factory function used to generate the
// underlying file system handle. The default value is os.DirFS. The string
// value provided to the factory function is the root directory of the file
// system. All paths given to open or write are considered relative to this
// root.
func WithOptionFSFactory(v func(root string) fs.FS) FileSystemLocalOption {
	return func(rfs *fileSystemLocal) {
		rfs.fsFactory = v
	}
}

// WithOptionFileFilter installs a custom filter function used to select files
// when a target is a directory. The default value check against a list of known
// file formats that are supported by the compiler.
func WithOptionFileFilter(v FileFilter) FileSystemLocalOption {
	return func(rfs *fileSystemLocal) {
		rfs.fileFilter = v
	}
}

type fileSystemLocal struct {
	root       string
	fsFactory  func(string) fs.FS
	fileFilter FileFilter
}

// NewFileSystemLocal creates a new FileSystem that uses the local file system.
func NewFileSystemLocal(root string, options ...FileSystemLocalOption) (idl.FileSystem, error) {
	absroot, err := filepath.Abs(root)
	if err != nil {
		return nil, exc.WrapUnknown(exc.Location{URI: root}, err)
	}
	result := &fileSystemLocal{
		root:      absroot,
		fsFactory: os.DirFS,
		fileFilter: func(ctx context.Context, fname string) bool {
			return knownExts[filepath.Ext(fname)] != idl.FileKindNone
		},
	}
	for _, option := range options {
		option(result)
	}
	return result, nil
}

func (r *fileSystemLocal) Open(ctx context.Context, uri string) ([]idl.File, error) {
	path := uri
	u, err := url.Parse(uri)
	if err == nil {
		path = u.Path
	}
	path = filepath.Join("/", path)

	dir := r.fsFactory(r.root)
	p := filepath.Clean(path)
	if p == "" || p == "/" {
		// If the entire path was a root then set to '.' to satisfy the
		// fs.ValidPath method which only allows, and requires, '.' when
		// it is expressing the root path.
		p = "."
	}
	p = strings.TrimPrefix(p, "/")
	// Trim the first slash character if present because fs.FS requires an
	// un-rooted path.
	d, err := dir.Open(p)
	if err != nil {
		return nil, fsErr(p, err)
	}
	defer d.Close()
	stat, _ := d.Stat()
	if !stat.IsDir() {
		f := NewFileFN(path, func() (io.ReadCloser, error) {
			return dir.Open(p)
		}, knownExts[filepath.Ext(p)])
		return []idl.File{f}, nil
	}
	dfs, err := d.(fs.ReadDirFile).ReadDir(0)
	if err != nil {
		return nil, fsErr(p, err)
	}
	files := make([]idl.File, 0, len(dfs))
	for _, df := range dfs {
		if df.IsDir() {
			continue
		}
		if !r.fileFilter(ctx, df.Name()) {
			continue
		}
		dfPath := filepath.Join(p, df.Name())
		rc, err := dir.Open(dfPath)
		if err != nil {
			return nil, fsErr(dfPath, err)
		}
		defer rc.Close()
		f := NewFileFN(dfPath, func() (io.ReadCloser, error) {
			return dir.Open(dfPath)
		}, knownExts[filepath.Ext(dfPath)])
		files = append(files, f)
	}
	if len(files) < 1 {
		return nil, exc.New(exc.Location{URI: path}, exc.CodeFileNotFound, fmt.Sprintf("found directory %s but it is empty", path))
	}
	return files, nil
}

func (r *fileSystemLocal) Write(ctx context.Context, uri string, content string) error {
	path := uri
	u, err := url.Parse(uri)
	if err == nil {
		path = u.Path
	}
	path = filepath.Join(r.root, "/", path)
	p := filepath.Clean(path)

	d := filepath.Dir(p)
	if err = os.MkdirAll(d, os.ModeDir|0o755); err != nil {
		return fsErr(d, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return fsErr(p, err)
	}
	return nil
}

func fsErr(path string, err error) error {
	if errT, ok := err.(*fs.PathError); ok {
		switch errT.Err {
		case fs.ErrInvalid:
			return exc.WrapUnknown(exc.Location{URI: errT.Path}, errT)
		case fs.ErrNotExist:
			return exc.Wrap(exc.Location{URI: errT.Path}, exc.CodeFileNotFound, errT)
		case fs.ErrPermission:
			return exc.Wrap(exc.Location{URI: errT.Path}, exc.CodePermissionDenied, errT)
		default:
			return exc.WrapUnknown(exc.Location{URI: errT.Path}, errT)
		}
	}
	return exc.WrapUnknown(exc.Location{URI: path}, err)
}
