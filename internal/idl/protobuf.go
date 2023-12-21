package idl

import (
	"fmt"
)

var PROTOBUF_TYPE_UIDS = map[string]uint64{
	"Package":        1,
	"NestedTypeInfo": 2,
}

var PROTOBUF_IDL = fmt.Sprintf(`
syntax = "microglot0"

module = @2

annotation Package(module) :Text @%d
annotation NestedTypeInfo(struct, enum) :Text @%d
`, PROTOBUF_TYPE_UIDS["Package"],
	PROTOBUF_TYPE_UIDS["NestedTypeInfo"],
)
