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
	typeKindError typeKind = 0
	// built-in
	typeKindPrimitive typeKind = 1
	typeKindData      typeKind = 2 // not considered primitive because it can't be constant
	typeKindVirtual   typeKind = 3
	// user-defined
	typeKindStruct     typeKind = 4
	typeKindEnum       typeKind = 5
	typeKindAPI        typeKind = 6
	typeKindSDK        typeKind = 7
	typeKindAnnotation typeKind = 8
	typeKindConstant   typeKind = 9
)

func (c *imageChecker) lookup(tr *proto.TypeReference) (typeKind, interface{}) {
	if tr.ModuleUID == 0 {
		name, ok := idl.GetBuiltinTypeNameFromUID(tr.TypeUID)
		if ok {
			var kind typeKind
			if name.Name == "Data" {
				kind = typeKindData
			} else if name.Name == "List" || name.Name == "Presence" {
				kind = typeKindVirtual
			} else {
				kind = typeKindPrimitive
			}
			return kind, &proto.Struct{Name: &name}
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
				case typeKindVirtual:
					typeName = declaration.(*proto.Struct).Name
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
								c.checkTypeSpecifier(parameter, []typeKind{typeKindPrimitive, typeKindData, typeKindVirtual, typeKindStruct, typeKindEnum})
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

func (c *imageChecker) checkConstantValue(value *proto.Value, expectedTypeSpecifier *proto.TypeSpecifier) {
	resolved, ok := expectedTypeSpecifier.Reference.(*proto.TypeSpecifier_Resolved)
	if !ok {
		c.reporter.Report(exc.New(exc.Location{
			// TODO 2023.12.12: location?
		}, exc.CodeUnknownFatal, fmt.Sprintf("Unexpected unresolved reference while type checking")))
	} else {
		expectedKind, expectedDeclaration := c.lookup(resolved.Resolved.Reference)

		if expectedKind != typeKindPrimitive {
			c.reporter.Report(exc.New(exc.Location{
				// TODO 2023.12.12: location?
			}, exc.CodeUnknownFatal, fmt.Sprintf("Constant assignment to non-primitive type %d", expectedKind)))
		} else {
			primitiveTypeName := expectedDeclaration.(*proto.Struct).Name.Name
			switch valueKind := value.Kind.(type) {
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
				if primitiveTypeName != "Int8" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign int8 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Int16:
				if primitiveTypeName != "Int16" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign int16 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_Int32:
				if primitiveTypeName != "Int32" {
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
				if primitiveTypeName != "Uint8" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign uint8 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_UInt16:
				if primitiveTypeName != "Uint16" {
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("attempted to assign uint16 value to constant of type %s", primitiveTypeName)))
				}
			case *proto.Value_UInt32:
				if primitiveTypeName != "UInt32" {
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
				if primitiveTypeName != "Float32" {
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
				switch identifierReference := valueKind.Identifier.Reference.(type) {
				case *proto.ValueIdentifier_Type:
					kind, declaration := c.lookup(identifierReference.Type)
					if kind != typeKindConstant {
						c.reporter.Report(exc.New(exc.Location{
							// TODO 2023.12.12: location?
						}, exc.CodeUnknownFatal, fmt.Sprintf("constants derived from non-constant types are not supported")))
					} else {
						constant := declaration.(*proto.Constant)
						c.checkConstantValue(constant.Value, expectedTypeSpecifier)
					}
				case *proto.ValueIdentifier_Attribute:
					c.reporter.Report(exc.New(exc.Location{
						// TODO 2023.12.12: location?
					}, exc.CodeUnknownFatal, fmt.Sprintf("constants derived from attributes are not supported")))
				}

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
				c.checkConstantValue(valueKind.Unary.Value, expectedTypeSpecifier)
				switch valueKind.Unary.Operation {
				case proto.OperationUnary_OperationUnaryPositive:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationUnary_OperationUnaryNegative:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationUnary_OperationUnaryNot:
					// TODO 2023.12.12: additionally, check operator compat
				}

			case *proto.Value_Binary:
				c.checkConstantValue(valueKind.Binary.Left, expectedTypeSpecifier)
				c.checkConstantValue(valueKind.Binary.Right, expectedTypeSpecifier)
				switch valueKind.Binary.Operation {
				case proto.OperationBinary_OperationBinaryOr:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryAnd:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryEqual:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryNotEqual:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryLessThan:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryLessThanEqual:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryGreaterThan:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryGreaterThanEqual:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryAdd:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinarySubtract:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryBinOr:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryBinAnd:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryShiftLeft:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryShiftRight:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryMultiply:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryDivide:
					// TODO 2023.12.12: additionally, check operator compat
				case proto.OperationBinary_OperationBinaryModulo:
					// TODO 2023.12.12: additionally, check operator compat
				}
			}
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
				c.checkTypeSpecifier(field.Type, []typeKind{typeKindPrimitive, typeKindData, typeKindVirtual, typeKindStruct, typeKindEnum})
				if field.DefaultValue != nil {
					c.checkConstantValue(field.DefaultValue, field.Type)
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
					c.checkTypeSpecifier(sdkMethodInput.Type, []typeKind{typeKindPrimitive, typeKindData, typeKindVirtual, typeKindStruct, typeKindEnum})
				}
				c.checkTypeSpecifier(sdkMethod.Output, []typeKind{typeKindPrimitive, typeKindData, typeKindVirtual, typeKindStruct, typeKindEnum})
			}
		}
		for _, annotation := range module.Annotations {
			// TODO 2023.12.12: I assume, even though it's not in the spec, that annotation
			// application arguments need to be primitive, for the same reasons that
			// constants must be primitive
			c.checkTypeSpecifier(annotation.Type, []typeKind{typeKindPrimitive})
		}
		for _, constant := range module.Constants {
			c.checkAnnotationApplications(constant.AnnotationApplications)
			c.checkTypeSpecifier(constant.Type, []typeKind{typeKindPrimitive})
			c.checkConstantValue(constant.Value, constant.Type)
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
				annotation := declaration.(*proto.Annotation)
				c.checkConstantValue(annotationApplication.Value, annotation.Type)
			}
		}
	}
}

func (c *imageChecker) checkTypeName(typeName *proto.TypeName) {
	for _, parameter := range typeName.Parameters {
		c.checkTypeSpecifier(parameter, []typeKind{typeKindPrimitive, typeKindData, typeKindVirtual, typeKindStruct, typeKindEnum})
	}
}
