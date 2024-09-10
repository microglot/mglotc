#!/usr/bin/env bash

# © 2023 Microglot LLC
#
# SPDX-License-Identifier: Apache-2.0

set -e -x

FILE=${1:-test.mgdl}

IN=$(mktemp -d)
OUT=$(mktemp -d)

cp test/* $IN

go run main.go --root=$IN/ $FILE --output=$OUT --pbplugin=protoc-gen-go:paths=source_relative --pbplugin=protoc-gen-go-grpc:paths=source_relative --plugin=mgdl-gen-go

(cd $OUT; go mod init asdf_asdf && go get google.golang.org/grpc && go build)
