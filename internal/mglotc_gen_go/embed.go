// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package mglotc_gen_go

import (
	"bytes"
	"fmt"
	"go/token"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/types/pluginpb"

	"gopkg.microglot.org/mglotc/internal/idl"
	"gopkg.microglot.org/mglotc/internal/proto"
	"gopkg.microglot.org/mglotc/internal/target"
)

// the generatedFile struct and its interface are very closely derived from protobuf/compiler/protogen;
// those types are unfortunately too tightly tied to protobuf's plugin system to be reused here.
type generatedFile struct {
	filename string
	buf      bytes.Buffer
	imports  map[string]gopkg
	pkgname  string
}

func (g *generatedFile) PackageName(v string) {
	g.pkgname = v
}

func (g *generatedFile) Import(v ...gopkg) {
	for _, x := range v {
		if _, ok := g.imports[x.importPath]; ok {
			continue
		}
		g.imports[x.importPath] = x
	}
}

func (g *generatedFile) P(v ...interface{}) {
	for _, x := range v {
		switch x := x.(type) {
		default:
			fmt.Fprint(&g.buf, x)
		}
	}
	fmt.Fprintln(&g.buf)
}

func (g *generatedFile) String() string {
	outBuf := new(bytes.Buffer)
	_, _ = outBuf.WriteString("// Code generated by mglot-gen-go. DO NOT EDIT.")
	_, _ = outBuf.WriteString("\n\n")
	fmt.Fprintf(outBuf, "package %s\n\n", g.pkgname)
	for _, x := range g.imports {
		fmt.Fprintf(outBuf, "import %s \"%s\"\n\n", x.localName, x.importPath)
	}
	_, _ = g.buf.WriteTo(outBuf)
	fmt.Fprint(outBuf, g.buf.String())
	return outBuf.String()
}

type gopkg struct {
	importPath string
	localName  string
}

type Generator struct {
	opts     opts
	image    *idl.Image
	gopkgMap map[uint64]gopkg
}

func NewGenerator(parameters string, image *idl.Image) (*Generator, error) {
	op, err := parseOpts(parameters)
	if err != nil {
		return nil, err
	}
	gopkgMap := make(map[uint64]gopkg)
	for _, module := range image.Modules {
		an := idl.GetProtobufAnnotation(module.AnnotationApplications, "FileOptionsGoPackage")
		if an == nil {
			// TODO: 2024-03-01: Add a native option for Go package definition.
			return nil, fmt.Errorf("unable to determine Go import path for %q. Please add a $(Protobuf.FileOptionsGoPackage()) annotation.\n", module.URI)
		}
		importPath := an.Kind.(*proto.Value_Text).Text.Value
		localName := ""
		if strings.Contains(importPath, ";") {
			parts := strings.Split(importPath, ";")
			importPath = parts[0]
			localName = parts[1]
		}
		if localName == "" {
			localName = path.Base(importPath)
		}
		gopkgMap[module.UID] = gopkg{
			importPath: importPath,
			localName:  GoSanitized(localName),
		}
	}
	return &Generator{
		opts:     op,
		image:    image,
		gopkgMap: gopkgMap,
	}, nil
}

func (gen *Generator) Generate(targets []string) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	// the use of CodeGeneratorResponse_File is just for short-term convenience, here; don't hesitate to
	// switch it out for something better.
	files := []*pluginpb.CodeGeneratorResponse_File{}
	for _, tgt := range targets {
		targetURI := target.Normalize(tgt)
		for _, module := range gen.image.Modules {
			if module.URI == targetURI {
				// TODO 2024.01.03: what about package names in .mglot directly? Currently this
				//  forces the $(Protobuf.FileOptionsGoPackage()) annotation, which is definitely
				//  weird when the source file is .mglot
				// TODO 2024.01.03: support -M flag, not only the go package
				packagePath, packageName := gen.gopkgMap[module.UID].importPath, gen.gopkgMap[module.UID].localName

				var filename string
				switch gen.opts.pathMode {
				case pathModeRelative:
					prefix := module.URI
					prefix = removeProtoExt(prefix)
					filename = prefix + ".mglot.go"
				case pathModeImport:
					file := path.Base(module.URI)
					file = removeProtoExt(file)
					filename = path.Join(packagePath, file) + ".mglot.go"
					if gen.opts.modulePrefix != "" {
						filename = strings.TrimPrefix(filename, gen.opts.modulePrefix)
					}
				}

				g := &generatedFile{
					filename: filename,
					imports:  make(map[string]gopkg),
				}
				g.PackageName(packageName)
				g.Import(gopkg{importPath: "context", localName: "context"})

				// emit constants
				for _, constant := range module.Constants {
					g.P("// const ", constant.Name)
					g.P("const ", constant.Name, " ", gen.genType(module.UID, g, gen.image, constant.Type), " = ", genLiteral(constant.Value))
					g.P()
				}

				if gen.opts.renderAPIs {
					// emit apis
					for _, api := range module.APIs {
						g.P("// type ", api.Name.Name, " is the interface for ", api.Name.Name, "API.")
						g.P("type ", api.Name.Name, " interface {")
						for _, ext := range api.Extends {
							g.P("    ", gen.genType(module.UID, g, gen.image, ext))
						}
						for _, method := range api.Methods {
							g.P("    ", method.Name, "(ctx context.Context, req ", gen.genType(module.UID, g, gen.image, method.Input), ") (", gen.genType(module.UID, g, gen.image, method.Output), ", error)")
						}
						g.P("}")
						g.P()
					}
				}

				// emit sdks
				for _, sdk := range module.SDKs {
					ifName := sdk.Name.Name
					g.P("// type ", ifName, " is the interface for ", sdk.Name.Name, "SDK.")
					g.P("type ", ifName, " interface {")
					for _, ext := range sdk.Extends {
						g.P("    ", gen.genType(module.UID, g, gen.image, ext))
					}
					for _, method := range sdk.Methods {
						arguments := "ctx context.Context"
						for _, input := range method.Input {
							arguments += ", "
							arguments += input.Name + " "
							arguments += gen.genType(module.UID, g, gen.image, input.Type)
						}
						rtype := ""
						if method.Output != nil {
							rtype = gen.genType(module.UID, g, gen.image, method.Output)
						}
						if !method.NoThrows {
							if rtype == "" {
								rtype = "error"
							} else {
								rtype = "(" + rtype + ", error" + ")"
							}
						}

						g.P("    ", method.Name, "(", arguments, ") ", rtype)
					}
					g.P("}")
				}

				content := g.String()
				files = append(files, &pluginpb.CodeGeneratorResponse_File{
					Name:    &g.filename,
					Content: &content,
				})
			}
		}
	}
	return files, nil
}

// generate a golang type name from a proto.TypeSpecifier
func (gen *Generator) genType(mod uint64, g *generatedFile, image *idl.Image, t *proto.TypeSpecifier) string {
	resolved := t.Reference.(*proto.TypeSpecifier_Resolved).Resolved
	kind, declaration := image.Lookup(resolved.Reference)
	switch kind {
	case idl.TypeKindPrimitive:
		switch declaration.(*proto.Struct).Name.Name {
		case "Bool":
			return "bool"
		case "Text":
			return "string"
		case "Int8":
			return "int8"
		case "Int16":
			return "int16"
		case "Int32":
			return "int32"
		case "Int64":
			return "int64"
		case "UInt8":
			return "uint8"
		case "UInt16":
			return "uint16"
		case "UInt32":
			return "uint32"
		case "UInt64":
			return "uint64"
		case "Float32":
			return "float32"
		case "Float64":
			return "float64"
		default:
			// type checking should prevent this from ever happening
			panic("unknown primitive type in mglot-gen-go")
		}
	case idl.TypeKindStruct:
		name := declaration.(*proto.Struct).Name.Name
		if declaration.(*proto.Struct).Reference.ModuleUID != mod {
			imp := gen.gopkgMap[declaration.(*proto.Struct).Reference.ModuleUID]
			g.Import(imp)
			name = imp.localName + "." + name
		}
		return "*" + name
	case idl.TypeKindAPI:
		if gen.opts.renderAPIs {
			// direct name of the API as generated from this plugin
			name := declaration.(*proto.API).Name.Name
			if declaration.(*proto.API).Reference.ModuleUID != mod {
				imp := gen.gopkgMap[declaration.(*proto.API).Reference.ModuleUID]
				g.Import(imp)
				name = imp.localName + "." + name
			}
			return name
		}
		// name of the server interface generated by protoc_gen_go_grpc
		name := fmt.Sprintf("%sServer", declaration.(*proto.API).Name.Name)
		if declaration.(*proto.API).Reference.ModuleUID != mod {
			imp := gen.gopkgMap[declaration.(*proto.API).Reference.ModuleUID]
			g.Import(imp)
			name = imp.localName + "." + name
		}
		return name
	case idl.TypeKindSDK:
		// name of the SDK as specified in the .mglot
		name := declaration.(*proto.SDK).Name.Name
		if declaration.(*proto.SDK).Reference.ModuleUID != mod {
			imp := gen.gopkgMap[declaration.(*proto.SDK).Reference.ModuleUID]
			g.Import(imp)
			name = imp.localName + "." + name
		}
		return name
	case idl.TypeKindVirtual:
		switch declaration.(*proto.Struct).Name.Name {
		case "List":
			return fmt.Sprintf("[]%s", gen.genType(mod, g, image, resolved.Parameters[0]))
		case "Presence":
			return fmt.Sprintf("*%s", gen.genType(mod, g, image, resolved.Parameters[0]))
		case "Map":
			return fmt.Sprintf("map[%s]%s", gen.genType(mod, g, image, resolved.Parameters[0]), gen.genType(mod, g, image, resolved.Parameters[1]))
		default:
			panic("unsupported virtual type in mglot-gen-go")
		}
	case idl.TypeKindEnum:
		name := declaration.(*proto.Enum).Name
		if declaration.(*proto.Enum).Reference.ModuleUID != mod {
			imp := gen.gopkgMap[declaration.(*proto.Enum).Reference.ModuleUID]
			g.Import(imp)
			name = imp.localName + "." + name
		}
		return name
	default:
		// TODO 2024.01.02: data, enum, annotation
		panic("unsupported type in mglot-gen-go")
	}
}

// generate a golang literal from a proto.Value
func genLiteral(value *proto.Value) string {
	switch v := value.Kind.(type) {
	case *proto.Value_Bool:
		return fmt.Sprintf("%#v", v.Bool.Value)
	case *proto.Value_Text:
		return fmt.Sprintf("%#v", v.Text.Value)
	case *proto.Value_Int8:
		return fmt.Sprintf("%#v", v.Int8.Value)
	case *proto.Value_Int16:
		return fmt.Sprintf("%#v", v.Int16.Value)
	case *proto.Value_Int32:
		return fmt.Sprintf("%#v", v.Int32.Value)
	case *proto.Value_Int64:
		return fmt.Sprintf("%#v", v.Int64.Value)
	case *proto.Value_UInt8:
		return fmt.Sprintf("%#v", v.UInt8.Value)
	case *proto.Value_UInt16:
		return fmt.Sprintf("%#v", v.UInt16.Value)
	case *proto.Value_UInt32:
		return fmt.Sprintf("%#v", v.UInt32.Value)
	case *proto.Value_UInt64:
		return fmt.Sprintf("%#v", v.UInt64.Value)
	case *proto.Value_Float32:
		return fmt.Sprintf("%#v", v.Float32.Value)
	case *proto.Value_Float64:
		return fmt.Sprintf("%#v", v.Float64.Value)
	default:
		// type checking should prevent this from ever happening
		panic("unsupported value kind in mglot-gen-go")
	}
}

type pathMode string

const (
	pathModeImport   pathMode = "IMPORT"
	pathModeRelative pathMode = "RELATIVE"
)
const (
	paramKeyPaths            = "paths"
	paramKeyModule           = "module"
	paramKeyAPIs             = "apis"
	paramValueSourceRelative = "source_relative"
	paramValueImport         = "import"
	paramValueTrue           = "true"
)

type opts struct {
	pathMode     pathMode
	modulePrefix string
	renderAPIs   bool
}

func parseOpts(parameters string) (opts, error) {
	opts := opts{
		pathMode: pathModeImport,
	}
	for _, p := range strings.Split(parameters, ";") {
		parts := strings.Split(p, "=")
		if len(parts) != 2 {
			return opts, fmt.Errorf("invalid parameter: %s", p)
		}
		key, value := parts[0], parts[1]
		switch key {
		case paramKeyPaths:
			switch value {
			case paramValueSourceRelative:
				opts.pathMode = pathModeRelative
			case paramValueImport:
				opts.pathMode = pathModeImport
			}
		case paramKeyModule:
			opts.pathMode = pathModeImport
			opts.modulePrefix = value
		case paramKeyAPIs:
			opts.renderAPIs = value == paramValueTrue
		}
	}
	return opts, nil
}

func removeProtoExt(s string) string {
	if ext := path.Ext(s); ext == ".proto" || ext == ".protodevel" {
		return s[:len(s)-len(ext)]
	}
	return s
}

// COPY/PASTE FROM google.golang.org/protobuf/internal/strs!
// GoSanitized converts a string to a valid Go identifier.
func GoSanitized(s string) string {
	// Sanitize the input to the set of valid characters,
	// which must be '_' or be in the Unicode L or N categories.
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '_'
	}, s)

	// Prepend '_' in the event of a Go keyword conflict or if
	// the identifier is invalid (does not start in the Unicode L category).
	r, _ := utf8.DecodeRuneInString(s)
	if token.Lookup(s).IsKeyword() || !unicode.IsLetter(r) {
		return "_" + s
	}
	return s
}
