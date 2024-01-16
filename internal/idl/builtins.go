package idl

import (
	"gopkg.microglot.org/compiler.go/internal/proto"
)

var BUILTIN_UID_TYPENAMES = map[uint64]proto.TypeName{
	1:  proto.TypeName{Name: "Bool"},
	2:  proto.TypeName{Name: "Text"},
	3:  proto.TypeName{Name: "Data"},
	4:  proto.TypeName{Name: "Int8"},
	5:  proto.TypeName{Name: "Int16"},
	6:  proto.TypeName{Name: "Int32"},
	7:  proto.TypeName{Name: "Int64"},
	8:  proto.TypeName{Name: "UInt8"},
	9:  proto.TypeName{Name: "UInt16"},
	10: proto.TypeName{Name: "UInt32"},
	11: proto.TypeName{Name: "UInt64"},
	12: proto.TypeName{Name: "Float32"},
	13: proto.TypeName{Name: "Float64"},
	14: proto.TypeName{
		Name: "Presence",
		Parameters: []*proto.TypeSpecifier{
			&proto.TypeSpecifier{},
		},
	},
	15: proto.TypeName{
		Name: "List",
		Parameters: []*proto.TypeSpecifier{
			&proto.TypeSpecifier{},
		},
	},
	16: proto.TypeName{
		Name: "Map",
		Parameters: []*proto.TypeSpecifier{
			&proto.TypeSpecifier{},
			&proto.TypeSpecifier{},
		},
	},
}

func GetBuiltinTypeNameFromUID(uid uint64) (proto.TypeName, bool) {
	v, ok := BUILTIN_UID_TYPENAMES[uid]
	return v, ok
}
