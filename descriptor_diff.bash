#!/usr/bin/env bash

# © 2023 Microglot LLC
#
# SPDX-License-Identifier: Apache-2.0

set -e

NAME=${1:-descriptor}

echo "comparing protoc vs. main.go compilation of test/$NAME.proto"

IN=$(mktemp -d)
OUT_BASELINE=$(mktemp -d)
OUT_ROUNDTRIP=$(mktemp -d)

cp test/* "${IN}"

protoc --proto_path="${IN}"/ "${NAME}.proto" --go_out=paths=source_relative:"${OUT_BASELINE}" --fatal_warnings
# This test compares both the descriptor and the generated Go code. The Go code
# includes the protoc version so we remove that because because microglot
# generated code doesn't produce the same version value. Also, the Go code
# usually only contains the raw descriptor long enough to use it at init time.
# So, while the bytes are embedded in the Go source the contents are eventually
# erased with something like file___foo_mgdl_rawDesc = nil which then makes it
# inaccessible. We remove the nil assignment so that the injected test code
# can access that content.
sed --in-place --expression "s/^\/\/ \tprotoc .*//" "${OUT_BASELINE}/${NAME}.pb.go"
sed --in-place --expression "s/.*rawDesc = nil//" "${OUT_BASELINE}/${NAME}.pb.go"
# TODO 2024-08-28: Add support for comment headers in IDL files
# The following removes the first 4 lines of the baseline file that represent
# the license header. The compiler doesn't yet handle IDL file headers.
tail -n +5 "${OUT_BASELINE}/${NAME}.pb.go" > "${OUT_BASELINE}/${NAME}.pb.go.trunc" && mv "${OUT_BASELINE}/${NAME}.pb.go.trunc" "${OUT_BASELINE}/${NAME}.pb.go"

## This code unmarshals the "rawDesc" descriptor and partially outputs it in string form, to make
## tracking down subtle differences between the baseline and round-trip descriptor easier.
cat >"${OUT_BASELINE}/main.go" <<EOF
package main

import (
        "fmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func printDescriptorProto(prefix string, m *descriptorpb.DescriptorProto) {
            fmt.Printf("%s.Name: %v\n", prefix, *m.Name)
            for j, f := range m.Field {
                fmt.Printf("%s.Field[%d]: %v\n", prefix, j, f)
            }
            fmt.Printf("%s.Extension: %v\n", prefix, m.Extension)
            for j, t := range m.NestedType {
                        printDescriptorProto(fmt.Sprintf("%s.NestedType[%d]", prefix, j), t)
            }
            fmt.Printf("%s.EnumType: %v\n", prefix, m.EnumType)
            fmt.Printf("%s.ExtensionRange: %v\n", prefix, m.ExtensionRange)
            fmt.Printf("%s.OneofDecl: %v\n", prefix, m.OneofDecl)
            fmt.Printf("%s.Options: %v\n", prefix, m.Options)
         }


func main() {
        var d descriptorpb.FileDescriptorProto
        proto.UnmarshalOptions{}.Unmarshal(file_${NAME}_proto_rawDesc, &d)

        fmt.Printf("Name: %v\n", *d.Name)
        fmt.Printf("Package: %v\n", d.Package)
        fmt.Printf("Dependency: %v\n", d.Dependency)
        fmt.Printf("PublicDependency: %v\n", d.PublicDependency)
        fmt.Printf("WeakDependency: %v\n", d.WeakDependency)
        for i, m := range d.MessageType {
                printDescriptorProto(fmt.Sprintf("MessageType[%d]", i), m)
        }
        fmt.Printf("EnumType: %v\n", d.EnumType)
}
EOF
(
        cd "${OUT_BASELINE}"
        go mod init "${NAME}" && go get google.golang.org/grpc && go run .
) >"${OUT_BASELINE}/descriptor" 2>/dev/null

go run main.go --root="${IN}/" "${NAME}.proto" --output="${OUT_ROUNDTRIP}" --pbplugin=protoc-gen-go:paths=source_relative
sed --in-place --expression "s/^\/\/ \tprotoc .*//" "${OUT_ROUNDTRIP}/${NAME}.pb.go"
sed --in-place --expression "s/.*rawDesc = nil//" "${OUT_ROUNDTRIP}/${NAME}.pb.go"


cp "${OUT_BASELINE}/main.go" "${OUT_ROUNDTRIP}/main.go"
(
        cd "${OUT_ROUNDTRIP}"
        go mod init "${NAME}" && go get google.golang.org/grpc && go run .
) >"${OUT_ROUNDTRIP}/descriptor" 2>/dev/null

DESC_DIFF="$(git --no-pager diff --no-index "${OUT_BASELINE}/descriptor" "${OUT_ROUNDTRIP}/descriptor" || true)"
GO_DIFF="$(git --no-pager diff --no-index "${OUT_BASELINE}/${NAME}.pb.go" "${OUT_ROUNDTRIP}/${NAME}.pb.go" || true)"

if [[ "${DESC_DIFF}" != "" ]]; then
        echo "Found a descriptor diff"
        echo "${DESC_DIFF}"
        exit 1
fi

if [[ "${GO_DIFF}" != "" ]]; then
        echo "Found a protoc-gen-go diff"
        echo "${GO_DIFF}"
        exit 1
fi
