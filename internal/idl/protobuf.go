package idl

import (
	"sync"
)

var PROTOBUF_TYPE_UIDS = map[string]uint64{
	"Package":        0,
	"Proto3Optional": 1,
	"NestedTypeInfo": 2,
}

var protobuf_uid_types map[uint64]string = nil

var computeProtobufUidTypes = sync.OnceFunc(func() {
	if protobuf_uid_types == nil {
		protobuf_uid_types = make(map[uint64]string)
		for protobufTypeName, protobufTypeUID := range PROTOBUF_TYPE_UIDS {
			protobuf_uid_types[protobufTypeUID] = protobufTypeName
		}
	}
})

func GetProtobufTypeNameFromUID(uid uint64) (string, bool) {
	computeProtobufUidTypes()
	v, ok := protobuf_uid_types[uid]
	return v, ok
}
