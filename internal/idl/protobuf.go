package idl

import (
	"fmt"
)

var PROTOBUF_TYPE_UIDS = map[string]uint64{
	"Package":              1,
	"NestedTypeInfo":       2,
	"FileOptionsGoPackage": 3,
}

var PROTOBUF_IDL = fmt.Sprintf(`
syntax = "microglot0"

module = @2 $(Protobuf.FileOptionsGoPackage("not.importable"))

annotation Package(module) :Text @%d
annotation NestedTypeInfo(struct, enum) :List<:Text> @%d
annotation FileOptionsGoPackage(module) :Text @%d
`, PROTOBUF_TYPE_UIDS["Package"],
	PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
	PROTOBUF_TYPE_UIDS["FileOptionsGoPackage"],
)
