// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package idl

import (
	"gopkg.microglot.org/compiler.go/internal/proto"
)

type TypeKind uint16

const (
	TypeKindError TypeKind = 0
	// built-in
	TypeKindPrimitive TypeKind = 1
	TypeKindData      TypeKind = 2 // not considered primitive because it can't be constant
	TypeKindVirtual   TypeKind = 3
	// user-defined
	TypeKindStruct     TypeKind = 4
	TypeKindEnum       TypeKind = 5
	TypeKindAPI        TypeKind = 6
	TypeKindSDK        TypeKind = 7
	TypeKindAnnotation TypeKind = 8
	TypeKindConstant   TypeKind = 9
)

func (i *Image) Lookup(tr *proto.TypeReference) (TypeKind, interface{}) {
	if tr.ModuleUID == 0 {
		name, ok := GetBuiltinTypeNameFromUID(tr.TypeUID)
		if ok {
			var kind TypeKind
			if name.Name == "Data" {
				kind = TypeKindData
			} else if name.Name == "List" || name.Name == "Presence" || name.Name == "Map" {
				kind = TypeKindVirtual
			} else {
				kind = TypeKindPrimitive
			}
			return kind, &proto.Struct{Name: &name}
		}
	}
	for _, module := range i.Modules {
		if module.UID == tr.ModuleUID {
			for _, struct_ := range module.Structs {
				if struct_.Reference.TypeUID == tr.TypeUID {
					return TypeKindStruct, struct_
				}
			}
			for _, enum := range module.Enums {
				if enum.Reference.TypeUID == tr.TypeUID {
					return TypeKindEnum, enum
				}
			}
			for _, api := range module.APIs {
				if api.Reference.TypeUID == tr.TypeUID {
					return TypeKindAPI, api
				}
			}
			for _, sdk := range module.SDKs {
				if sdk.Reference.TypeUID == tr.TypeUID {
					return TypeKindSDK, sdk
				}
			}
			for _, annotation := range module.Annotations {
				if annotation.Reference.TypeUID == tr.TypeUID {
					return TypeKindAnnotation, annotation
				}
			}
			for _, constant := range module.Constants {
				if constant.Reference.TypeUID == tr.TypeUID {
					return TypeKindConstant, constant
				}
			}
		}
	}
	return TypeKindError, nil
}
