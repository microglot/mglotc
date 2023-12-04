package compiler

import (
	"fmt"

	"gopkg.microglot.org/compiler.go/internal/exc"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/proto"
)

// check() applies type-checking logic to an Image of linked Module descriptors.
// reports: type mismatches
func check(image *idl.Image, reporter exc.Reporter) {
	checker := imageChecker{
		image:    image,
		reporter: reporter,
	}
	checker.check()
}

type imageChecker struct {
	image    *idl.Image
	reporter exc.Reporter
}

type typeKind uint16

const (
	typeKindError      typeKind = 0
	typeKindPrimitive  typeKind = 1
	typeKindStruct     typeKind = 2
	typeKindEnum       typeKind = 3
	typeKindAPI        typeKind = 4
	typeKindSDK        typeKind = 5
	typeKindAnnotation typeKind = 6
	typeKindConstant   typeKind = 7
)

func (c *imageChecker) lookup(tr *proto.TypeReference) (typeKind, interface{}) {
	if tr.ModuleUID == 0 {
		name, ok := idl.GetBuiltinTypeNameFromUID(tr.TypeUID)
		if ok {
			if name == "Presence" {
				return typeKindStruct, &proto.Struct{
					Name: &proto.TypeName{
						Name: "Presence",
						Parameters: []*proto.TypeSpecifier{
							&proto.TypeSpecifier{},
						},
					},
				}
			}
			if name == "List" {
				return typeKindStruct, &proto.Struct{
					Name: &proto.TypeName{
						Name: "List",
						Parameters: []*proto.TypeSpecifier{
							&proto.TypeSpecifier{},
						},
					},
				}
			}
			return typeKindPrimitive, nil
		}
	}
	if tr.ModuleUID == 1 {
		// TODO 2023.11.26: parameterized
		name, ok := idl.GetProtobufTypeNameFromUID(tr.TypeUID)
		if ok {
			if name == "Package" {
				return typeKindAnnotation, &proto.Annotation{}
			}
			if name == "NestedTypeInfo" {
				return typeKindAnnotation, &proto.Annotation{}
			}
		}
	}
	for _, module := range c.image.Modules {
		if module.UID == tr.ModuleUID {
			for _, struct_ := range module.Structs {
				if struct_.Reference.TypeUID == tr.TypeUID {
					return typeKindStruct, struct_
				}
			}
			for _, enum := range module.Enums {
				if enum.Reference.TypeUID == tr.TypeUID {
					return typeKindEnum, enum
				}
			}
			for _, api := range module.APIs {
				if api.Reference.TypeUID == tr.TypeUID {
					return typeKindAPI, api
				}
			}
			for _, sdk := range module.SDKs {
				if sdk.Reference.TypeUID == tr.TypeUID {
					return typeKindSDK, sdk
				}
			}
			for _, annotation := range module.Annotations {
				if annotation.Reference.TypeUID == tr.TypeUID {
					return typeKindAnnotation, annotation
				}
			}
			for _, constant := range module.Constants {
				if constant.Reference.TypeUID == tr.TypeUID {
					return typeKindConstant, constant
				}
			}
		}
	}
	c.reporter.Report(exc.New(exc.Location{
		// TODO 2023.11.26: location?
	}, exc.CodeUnknownFatal, fmt.Sprintf("Resolved reference (ModuleUID=%d, TypeUID=%d) points to a type outside the current Image", tr.ModuleUID, tr.TypeUID)))
	return typeKindError, nil
}

func (c *imageChecker) checkTypeSpecifier(ts *proto.TypeSpecifier, expectedKinds []typeKind) {
	resolved, ok := ts.Reference.(*proto.TypeSpecifier_Resolved)
	if !ok {
		c.reporter.Report(exc.New(exc.Location{
			// TODO 2023.11.26: location?
		}, exc.CodeUnknownFatal, fmt.Sprintf("Unexpected unresolved reference while type checking")))
	} else {
		kind, declaration := c.lookup(resolved.Resolved.Reference)
		for _, expectedKind := range expectedKinds {
			if kind == expectedKind {
				var typeName *proto.TypeName = nil
				switch kind {
				case typeKindStruct:
					typeName = declaration.(*proto.Struct).Name
				case typeKindAPI:
					typeName = declaration.(*proto.API).Name
				case typeKindSDK:
					typeName = declaration.(*proto.SDK).Name
				}

				if len(resolved.Resolved.Parameters) > 0 {
					if typeName == nil {
						c.reporter.Report(exc.New(exc.Location{}, exc.CodeUnknownFatal, fmt.Sprintf("type can't be parameterized")))
					} else {
						if len(typeName.Parameters) != len(resolved.Resolved.Parameters) {
							c.reporter.Report(exc.New(exc.Location{}, exc.CodeUnknownFatal, fmt.Sprintf("wrong number of parameters")))
						} else {
							for _, parameter := range resolved.Resolved.Parameters {
								c.checkTypeSpecifier(parameter, []typeKind{typeKindPrimitive, typeKindStruct, typeKindEnum})
							}
						}
					}
				}

				return
			}
		}
		c.reporter.Report(exc.New(exc.Location{
			// TODO 2023.11.26: location?
		}, exc.CodeUnknownFatal, fmt.Sprintf("unexpected %d (expecting %v)", kind, expectedKinds)))
	}
}

func (c *imageChecker) check() {
	for _, module := range c.image.Modules {
		// TODO 2023.11.26: DotImport.Reference?
		c.checkAnnotationApplications(module.AnnotationApplications)
		for _, struct_ := range module.Structs {
			c.checkAnnotationApplications(struct_.AnnotationApplications)
			c.checkTypeName(struct_.Name)
			for _, field := range struct_.Fields {
				c.checkAnnotationApplications(field.AnnotationApplications)
				c.checkTypeSpecifier(field.Type, []typeKind{typeKindPrimitive, typeKindStruct, typeKindEnum})
			}
			for _, union := range struct_.Unions {
				c.checkAnnotationApplications(union.AnnotationApplications)
			}
		}
		for _, enum := range module.Enums {
			c.checkAnnotationApplications(enum.AnnotationApplications)
			for _, enumerant := range enum.Enumerants {
				c.checkAnnotationApplications(enumerant.AnnotationApplications)
			}
		}
		for _, api := range module.APIs {
			c.checkAnnotationApplications(api.AnnotationApplications)
			c.checkTypeName(api.Name)
			for _, extends := range api.Extends {
				c.checkTypeSpecifier(extends, []typeKind{typeKindAPI})
			}
			for _, apiMethod := range api.Methods {
				c.checkAnnotationApplications(apiMethod.AnnotationApplications)
				c.checkTypeSpecifier(apiMethod.Input, []typeKind{typeKindStruct})
				c.checkTypeSpecifier(apiMethod.Output, []typeKind{typeKindStruct})
			}
		}
		for _, sdk := range module.SDKs {
			c.checkAnnotationApplications(sdk.AnnotationApplications)
			c.checkTypeName(sdk.Name)
			for _, extends := range sdk.Extends {
				c.checkTypeSpecifier(extends, []typeKind{typeKindSDK})
			}
			for _, sdkMethod := range sdk.Methods {
				c.checkAnnotationApplications(sdkMethod.AnnotationApplications)
				for _, sdkMethodInput := range sdkMethod.Input {
					c.checkTypeSpecifier(sdkMethodInput.Type, []typeKind{typeKindPrimitive, typeKindStruct, typeKindEnum})
				}
				c.checkTypeSpecifier(sdkMethod.Output, []typeKind{typeKindPrimitive, typeKindStruct, typeKindEnum})
			}
		}
		for _, annotation := range module.Annotations {
			c.checkTypeSpecifier(annotation.Type, []typeKind{typeKindPrimitive, typeKindStruct, typeKindEnum})
		}
		for _, constant := range module.Constants {
			c.checkAnnotationApplications(constant.AnnotationApplications)
			c.checkTypeSpecifier(constant.Type, []typeKind{typeKindPrimitive, typeKindStruct, typeKindEnum})

			// TODO 2023.11.26: constant.Value vs. constant.Type
		}
	}
}

func (c *imageChecker) checkAnnotationApplications(annotationApplications []*proto.AnnotationApplication) {
	// TODO 2023.11.26: check that the annotation's scope matches the application
	for _, annotationApplication := range annotationApplications {
		resolved, ok := annotationApplication.Annotation.Reference.(*proto.TypeSpecifier_Resolved)
		if !ok {
			c.reporter.Report(exc.New(exc.Location{
				// TODO 2023.11.26: location?
			}, exc.CodeUnknownFatal, fmt.Sprintf("Unexpected unresolved reference while type checking")))
		} else {
			kind, declaration := c.lookup(resolved.Resolved.Reference)
			if kind != typeKindAnnotation {
				c.reporter.Report(exc.New(exc.Location{
					// TODO 2023.11.26: location?
				}, exc.CodeUnknownFatal, fmt.Sprintf("unexpected %d (expecting annotation)", kind)))
			} else {
				_ = declaration.(*proto.Annotation)
				// TODO 2023.11.26: annotationApplication.Value vs. annotation.Type
			}
		}
	}
}

func (c *imageChecker) checkTypeName(typeName *proto.TypeName) {
	for _, parameter := range typeName.Parameters {
		c.checkTypeSpecifier(parameter, []typeKind{typeKindPrimitive, typeKindStruct, typeKindEnum})
	}
}
