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

func (c *imageChecker) lookup(tr *proto.TypeReference) (idl.TypeKind, interface{}) {
	kind, declaration := c.image.Lookup(tr)
	if kind == idl.TypeKindError {
		c.reporter.Report(exc.New(exc.Location{
			// TODO 2023.11.26: location?
		}, exc.CodeUnknownFatal, fmt.Sprintf("Resolved reference (ModuleUID=%d, TypeUID=%d) points to a type outside the current Image", tr.ModuleUID, tr.TypeUID)))
	}
	return kind, declaration
}

func (c *imageChecker) checkTypeSpecifier(ts *proto.TypeSpecifier, expectedKinds []idl.TypeKind) {
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
				case idl.TypeKindVirtual:
					typeName = declaration.(*proto.Struct).Name
				case idl.TypeKindStruct:
					typeName = declaration.(*proto.Struct).Name
				case idl.TypeKindAPI:
					typeName = declaration.(*proto.API).Name
				case idl.TypeKindSDK:
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
								c.checkTypeSpecifier(parameter, []idl.TypeKind{idl.TypeKindPrimitive, idl.TypeKindData, idl.TypeKindVirtual, idl.TypeKindStruct, idl.TypeKindEnum})
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

func (c *imageChecker) checkValue(value *proto.Value, expectedTypeSpecifier *proto.TypeSpecifier) {
	resolved, ok := expectedTypeSpecifier.Reference.(*proto.TypeSpecifier_Resolved)
	if !ok {
		c.reporter.Report(exc.New(exc.Location{
			// TODO 2023.12.12: location?
		}, exc.CodeUnknownFatal, fmt.Sprintf("Unexpected unresolved reference while type checking")))
	} else {
		expectedKind, expectedDeclaration := c.lookup(resolved.Resolved.Reference)

		switch expectedKind {
		case idl.TypeKindPrimitive:
			primitiveTypeName := expectedDeclaration.(*proto.Struct).Name.Name
			switch value.Kind.(type) {
			case *proto.Value_Bool:
				if primitiveTypeName != "Bool" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign boolean value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Text:
				if primitiveTypeName != "Text" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign text value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Int8:
				if primitiveTypeName != "Int8" && primitiveTypeName != "Int16" && primitiveTypeName != "Int32" && primitiveTypeName != "Int64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign int8 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Int16:
				if primitiveTypeName != "Int16" && primitiveTypeName != "Int32" && primitiveTypeName != "Int64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign int16 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Int32:
				if primitiveTypeName != "Int32" && primitiveTypeName != "Int64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign int32 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Int64:
				if primitiveTypeName != "Int64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign int64 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_UInt8:
				if primitiveTypeName != "UInt8" && primitiveTypeName != "UInt16" && primitiveTypeName != "UInt32" && primitiveTypeName != "UInt64" && primitiveTypeName != "Int16" && primitiveTypeName != "Int32" && primitiveTypeName != "Int64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign uint8 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_UInt16:
				if primitiveTypeName != "UInt16" && primitiveTypeName != "UInt32" && primitiveTypeName != "UInt64" && primitiveTypeName != "Int32" && primitiveTypeName != "Int64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign uint16 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_UInt32:
				if primitiveTypeName != "UInt32" && primitiveTypeName != "UInt64" && primitiveTypeName != "Int64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign uint32 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_UInt64:
				if primitiveTypeName != "UInt64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign uint64 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Float32:
				if primitiveTypeName != "Float32" && primitiveTypeName != "Float64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign float32 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Float64:
				if primitiveTypeName != "Float64" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign float64 value to constant of type %s", primitiveTypeName)))
				}

			case *proto.Value_Identifier:
				c.reporter.Report(exc.New(exc.Location{
					// TODO 2023.12.12: location?
				}, exc.CodeUnknownFatal, fmt.Sprintf("constants derived from non-constant identifiers are not supported")))

			case *proto.Value_Data:
				c.reporter.Report(exc.New(exc.Location{
					// TODO 2023.12.12: location?
				}, exc.CodeUnknownFatal, fmt.Sprintf("data constants are not supported")))
			case *proto.Value_List:
				c.reporter.Report(exc.New(exc.Location{
					// TODO 2023.12.12: location?
				}, exc.CodeUnknownFatal, fmt.Sprintf("list constants are not supported")))
			case *proto.Value_Struct:
				c.reporter.Report(exc.New(exc.Location{
					// TODO 2023.12.12: location?
				}, exc.CodeUnknownFatal, fmt.Sprintf("struct constants are not supported")))

			case *proto.Value_Unary:
				c.reporter.Report(exc.New(exc.Location{
					// TODO 2023.12.12: location?
				}, exc.CodeUnknownFatal, fmt.Sprintf("constants can't be composed from unary operations on non-constant values")))

			case *proto.Value_Binary:
				c.reporter.Report(exc.New(exc.Location{
					// TODO 2023.12.12: location?
				}, exc.CodeUnknownFatal, fmt.Sprintf("constants can't be composed from binary operations on non-constant values")))
			}
		case idl.TypeKindVirtual:
			// TODO 2023.12.20: type check literal virtuals (lists, presences)
		case idl.TypeKindStruct:
			// TODO 2023.12.20: type check literal structs
		default:
			c.reporter.Report(exc.New(exc.Location{
				// TODO 2023.12.12: location?
			}, exc.CodeUnknownFatal, fmt.Sprintf("Constant assignment to unsupported type %d", expectedKind)))
		}
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
				c.checkTypeSpecifier(field.Type, []idl.TypeKind{idl.TypeKindPrimitive, idl.TypeKindData, idl.TypeKindVirtual, idl.TypeKindStruct, idl.TypeKindEnum})
				if field.DefaultValue != nil {
					c.checkValue(field.DefaultValue, field.Type)
				}
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
				c.checkTypeSpecifier(extends, []idl.TypeKind{idl.TypeKindAPI})
			}
			for _, apiMethod := range api.Methods {
				c.checkAnnotationApplications(apiMethod.AnnotationApplications)
				c.checkTypeSpecifier(apiMethod.Input, []idl.TypeKind{idl.TypeKindStruct})
				c.checkTypeSpecifier(apiMethod.Output, []idl.TypeKind{idl.TypeKindStruct})
			}
		}
		for _, sdk := range module.SDKs {
			c.checkAnnotationApplications(sdk.AnnotationApplications)
			c.checkTypeName(sdk.Name)
			for _, extends := range sdk.Extends {
				c.checkTypeSpecifier(extends, []idl.TypeKind{idl.TypeKindSDK})
			}
			for _, sdkMethod := range sdk.Methods {
				c.checkAnnotationApplications(sdkMethod.AnnotationApplications)
				for _, sdkMethodInput := range sdkMethod.Input {
					c.checkTypeSpecifier(sdkMethodInput.Type, []idl.TypeKind{idl.TypeKindPrimitive, idl.TypeKindData, idl.TypeKindVirtual, idl.TypeKindStruct, idl.TypeKindEnum})
				}
				c.checkTypeSpecifier(sdkMethod.Output, []idl.TypeKind{idl.TypeKindPrimitive, idl.TypeKindData, idl.TypeKindVirtual, idl.TypeKindStruct, idl.TypeKindEnum})
			}
		}
		for _, annotation := range module.Annotations {
			c.checkTypeSpecifier(annotation.Type, []idl.TypeKind{idl.TypeKindPrimitive, idl.TypeKindData, idl.TypeKindVirtual, idl.TypeKindStruct, idl.TypeKindEnum})
		}
		for _, constant := range module.Constants {
			c.checkAnnotationApplications(constant.AnnotationApplications)
			c.checkTypeSpecifier(constant.Type, []idl.TypeKind{idl.TypeKindPrimitive})
			c.checkValue(constant.Value, constant.Type)
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
			if kind != idl.TypeKindAnnotation {
				c.reporter.Report(exc.New(exc.Location{
					// TODO 2023.11.26: location?
				}, exc.CodeUnknownFatal, fmt.Sprintf("unexpected %d (expecting annotation)", kind)))
			} else {
				annotation := declaration.(*proto.Annotation)
				c.checkValue(annotationApplication.Value, annotation.Type)
			}
		}
	}
}

func (c *imageChecker) checkTypeName(typeName *proto.TypeName) {
	for _, parameter := range typeName.Parameters {
		c.checkTypeSpecifier(parameter, []idl.TypeKind{idl.TypeKindPrimitive, idl.TypeKindData, idl.TypeKindVirtual, idl.TypeKindStruct, idl.TypeKindEnum})
	}
}
