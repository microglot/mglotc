package idl

var BUILTIN_TYPE_UIDS = map[string]uint64{
	"Bool":    0,
	"Text":    1,
	"Data":    2,
	"Int8":    3,
	"Int16":   4,
	"Int32":   5,
	"Int64":   6,
	"UInt8":   7,
	"UInt16":  8,
	"UInt32":  9,
	"UInt64":  10,
	"Float32": 11,
	"Float64": 12,
}

var builtin_uid_types map[uint64]string = nil

func computeBuiltinUidTypes() {
	if builtin_uid_types == nil {
		builtin_uid_types = make(map[uint64]string)
		for builtinTypeName, builtinTypeUID := range BUILTIN_TYPE_UIDS {
			builtin_uid_types[builtinTypeUID] = builtinTypeName
		}
	}
}

func GetBuiltinTypeNameFromUID(uid uint64) (string, bool) {
	computeBuiltinUidTypes()
	v, ok := builtin_uid_types[uid]
	return v, ok
}
