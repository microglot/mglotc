// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package idl

import (
	"fmt"
)

var PROTOBUF_TYPE_UIDS = map[string]uint64{
	"Package":              1,
	"NestedTypeInfo":       2,
	"FileOptionsGoPackage": 3,
	"JsonName":             4,
	"Proto3Optional":       5,
	"EnumFromProto":        6,
}

var PROTOBUF_IDL = fmt.Sprintf(`
syntax = "mglot0"

module = @2 $(Protobuf.FileOptionsGoPackage("not.importable"))

struct NestedType {
   From :Text @1
   To :Text @2
}

struct NestedTypes {
   NestedTypes :List<:NestedType> @1
}

annotation Package(module) :Text @%d
annotation NestedTypeInfo(struct, enum) :NestedTypes @%d
annotation FileOptionsGoPackage(module) :Text @%d
annotation JsonName(field) :Text @%d
annotation Proto3Optional(field) :Bool @%d
annotation EnumFromProto(enum) :Bool @%d
`, PROTOBUF_TYPE_UIDS["Package"],
	PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
	PROTOBUF_TYPE_UIDS["FileOptionsGoPackage"],
	PROTOBUF_TYPE_UIDS["JsonName"],
	PROTOBUF_TYPE_UIDS["Proto3Optional"],
	PROTOBUF_TYPE_UIDS["EnumFromProto"],
)
