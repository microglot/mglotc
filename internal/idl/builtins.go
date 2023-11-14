package idl

import (
	"sync"
)

var BUILTIN_TYPE_UIDS = map[string]uint64{
	"Bool":     1,
	"Text":     2,
	"Data":     3,
	"Int8":     4,
	"Int16":    5,
	"Int32":    6,
	"Int64":    7,
	"UInt8":    8,
	"UInt16":   9,
	"UInt32":   10,
	"UInt64":   11,
	"Float32":  12,
	"Float64":  13,
	"Presence": 14,
	"List":     15,
}

var builtin_uid_types map[uint64]string = nil

var computeBuiltinUidTypes = sync.OnceFunc(func() {
	if builtin_uid_types == nil {
		builtin_uid_types = make(map[uint64]string)
		for builtinTypeName, builtinTypeUID := range BUILTIN_TYPE_UIDS {
			builtin_uid_types[builtinTypeUID] = builtinTypeName
		}
	}
})

func GetBuiltinTypeNameFromUID(uid uint64) (string, bool) {
	computeBuiltinUidTypes()
	v, ok := builtin_uid_types[uid]
	return v, ok
}
