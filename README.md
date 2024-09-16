<!--
© 2024 Microglot LLC

SPDX-License-Identifier: Apache-2.0
-->

# Microglot IDL Compiler

This project is an experimental IDL that has:

- Compatibility with proto2 and proto3 syntaxes for Protocol Buffers
- Compatibility with existing protoc code generation plugins
- An optional, alternative syntax for Protocol Buffers messages, enums, and
  services
- Support for defining constant values
- Support for defining non-networked APIs

These features will be expanded to include better tools for designing and
specifying the _implementations_ of APIs and not just the APIs themselves. This
includes structured language for describing the behaviors and constraints of an
implementation, operational details such as SLOs, defining implementation
dependencies on other APIs or implementations, generating diagrams, and more.

The overall goal of the Microglot IDL is expand today's code generation tooling
into a more complete software design suite.

## Project Status

In short: alpha/early-access.

The project is in early access for those interested in the idea or giving
feedback. The compiler supports the majority of features in both proto2 and
proto3 syntaxes and you can quickly validate it as a replacement for protoc
by following the usage instructions below.

The native IDL syntax that overlaps with the features of proto2 and proto3 are
not likely to change but the extended features are still experimental.

## Installation

Find your system architecture in the
[latest release](https://github.com/microglot/mglotc/releases), download
and expand the archive, and copy the `mglotc` binary to a directory on your
path.

## Usage

The compiler usage looks like:
```sh
mglotc --root DIR_CONTAINING_IDL -pbplugin PROTOC_PLUGIN IDL_FILE1 IDL_FILE2
```

### Content Roots

The `--root` flag may be used multiple times and each entry, in the order given,
represents a root directory used to search for imported IDL files. This flag is
equivalent to the `-I` flag in protoc.

To illustrate, let's work with the following directory structure:
```
- /
    - project1/
        - proto/
            - foo.proto
    - /project2
        - idl/
            - bar.mglot
    - baz.mglot
```

In this setup the `/project1/proto` directory contains IDL using either the
proto2 or proto3 syntax and the `/project2/idl` directory contains IDL using
the mglot syntax. The top level `baz.mglot` is an IDL file that imports
both `foo.proto` and `bar.mglot`.

Just like imports handled by protoc, all import statements are interpreted as
being relative to some root directory on the file system. For example, an import
path of either `project1/proto/foo.proto` in proto2/3 or
`/project2/idl/bar.mglot` in mglot are searched for by looking for those paths
within one of the defined roots. The compiler `--root` flag establishes an
ordered set of roots to search. Using the above directory structure we could
run the compiler with `mglotc --root . baz.mglot` and the import paths would
resolve because both resolve when evaluated from the project root containing the
`baz.mglot` file. For convenience, the `--root .` flag is set by default unless
other, explicit roots are defined.

### Plugins

Most protoc plugins should be compatible with the mglotc. The CLI syntax is
slightly different: `mglotc --pbplugin protoc-gen-go:arg1=v1,arg2=v2`. The
`--pbplugin` flag is used for identifying protoc plugins. The minimum value to
give with `--pbplugin` is the name of the plugin binary. This is slightly
different to protoc because protoc assumes the prefix `protoc-gen-` for plugins
and mglotc does not. If the protoc plugin accepts additional arguments then they
can be placed after a colon (`:`) and in exactly the same format as with protoc.
The arguments are passed through to the plugin unmodified.

The compiler is designed to handle multi-package inputs but some of the older
protoc plugins fail if given IDL content from multiple packages. The
`--per-package-mode` flag may be added to the compiler flags in order to force
batching of IDL content by package and calling protoc plugins once for each
package.

The compiler currently has only one native plugin called `mglotc-gen-go` and it
is currently embedded in the compiler itself. It is activated with `--plugin
mglotc-gen-go` and can be used in conjunction with protoc plugins. This plugin
generates constants, SDKs, and optionally APIs. The plugin supports the
following arguments:

- `paths=source_relative`
    - Identical to the protoc argument.
- `module=github.com/myproject/foo`
    - Defines the Go package path of the output directory and sets the `paths`
      parameter to `imports`.
- `apis=true`
    - Toggles rendering APIs.

The embedded Go plugin is not yet stable and provided only for experimentation
right now.

## Protocol Buffers Compatibility

The majority of existing proto2 and proto3 syntax IDL files should work without
issues but there are a few features that we don't yet or have only limited or
partial support:

- No support for plugin defined options. Only the global protoc options are
  supported right now.
- No support for `import weak`.
- Limited support for reserved IDs and names. These are currently parsed by the
  compiler but not enforced. We plan to improve support for reservations over
  time.

## Native IDL Syntax

The native IDL syntax is still a work in progress. The features that overlap
with proto2 and proto3 may receive minor syntax changes but the basic feature
set is generally stabilized on what proto2/3 support. Any feature that is not
native to proto2 or proto3 may be more volatile.

Setting the `syntax` property of an IDL file to `mglot0` instead of `proto2` or
`proto3` engages an alternate syntax with an extended feature set compared to
Protocol Buffers. The native syntax can import `proto2` and `proto3` files, has
an equivalent base feature set to Protocol Buffers, and will be expanded over
time to incorporate new features and ideas.

### Built-In Types

The built-in scalar and virtual types mirror those from Protocol Buffers. Here
is a table showing the available scalar types, their equivalent in the proto2/3
syntaxes, range of values, and example literal syntax.

| Type | proto2/3 | Value Range | Example Literal |
|------|----------|-------|---------|
| Bool | bool | true/false | true, false |
| Text | string | \<=2^32 UTF-8 bytes | "example", "☃" |
| Data | bytes | \<=2^32 bytes | 0x"F0123456789ABCDEF", 0x"DEAD BEEF" |
| Int8 | int32 | -2^7 to (2^7)-1 | 1, -100 |
| Int16 | int32 | -2^15 to (2^15)-1 | 1, -100, 1000, -1_000 |
| Int32 | int32 | -2^31 to (2^31)-1 | 1, -100, 1000, -1_000 |
| Int64 | int64 | -2^63 to (2^63)-1 | 1, -100, 1000, -1_000 |
| UInt8 | uint32 | 0 to (2^8)-1 | 1, 100 |
| UInt16 | uint32 | 0 to (2^16)-1 | 1, 100, 1000, 1_000 |
| UInt32 | uint32 | 0 to (2^32)-1 | 1, 100, 1000, 1_000 |
| UInt64 | uint64 | 0 to (2^64)-1 | 1, 100, 1000, 1_000 |
| Float32 | float | 32bit IEEE 754 | 0.0, -1E6, 0x2.p10 |
| Float64 | double | 64bit IEEE 754 | 0.0, -1E6, 0x2.p10 |

In addition to the scalar types, the mglot0 syntax supports the following
virtual types:

| Type | proto2/3 | Constraints | Example Usage |
|------|----------|-------|-----------|
| List\<T> | repeated | Value type cannot be List or Map types | :List\<:Text> |
| Map\<K,V> | map | Keys cannot be Data or Floats, Values cannot be List or Map types | :Map\<:Text, :MyStruct> |
| Presence\<T> | optional | Limited to scalar types | :Presence\<:Bool> |

### Modules

By default, IDL files using the mglot syntax are considered independent
namespaces, equivalent to a package in proto2/3. There is no implicit sharing of
a namespace unless an explicit proto2/3 package name is added using annotations.
Namespace sharing is only currently supported for backwards compatible
translation of proto2/3 syntax to mglot.

### Structs

Structs are equivalent to proto2/3 messages:
```
struct Foo {
    Bar :UInt32 @1
    Baz :Presence<:Bool> @2
} @1
```

Each field must have a name and type definition. The `@` notation assigns an
integer value as the UID, or field number. If no UID is provided then one is
generated for the field. Similarly, structs may be assigned a custom UID or one
will be generated. Field UIDs must be unique within the struct and struct UIDs
must be unique across all user defined types. The range of field UID values is
currently limited to the range of field numbers allowed in proto2/3 for
compatibility reasons.

Struct definitions also support a `union` feature which is equivalent to the
proto2/3 `oneof` feature:
```
struct Foo {
    union {
        Bar :UInt32 @2
        Baz :List<:Bool> @3
    } @1
}
```

Unions may be named or unnamed. If no name is given then the name `Union` is
assumed. The union itself is a field-like concept and has its own UID that must
not conflict with other fields.

### Enums

Enums are equivalent to the same feature in proto2/3:
```
enum Foo {
    Bar @1
    Baz @2
}
```
All enums have an implicit zero value of `None` that is equivalent to:
```
enum Foo {
    None @0
    Bar @1
    Baz @2
}
```
You can rename the zero/undefined value by explicitly adding it:
```
enum Foo {
    Invalid @0
    Bar @1
    Baz @2
}
```

Unlike proto2/3, the enumerant names are encapsulated within the enum namespace
and do not need to be unique across all enum definitions in the module.

### APIs

APIs are equivalent to proto2/3 services:
```
api Foo {
    Bar(:BarInput) returns (:BarOutput)
    Baz(:BazInput) returns (:BazOutput)
}
```

The inputs and outputs of API methods are limited to struct types. Unlike
proto2/3, the mglot syntax does not support streaming API methods.

APIs offer an extension mechanism for sharing methods:
```
api HealthChecker {
    HealthCheck(:Empty) returns (:HealthStatus)
}
api Foo extends (:HealthChecker) {
    Bar(:BarInput) returns (:BarOutput)
    Baz(:BazInput) returns (:BazOutput)
}
```
In the above example, the `Foo` API is considered to contain three methods:
`Bar`, `Baz`, and `HealthCheck`. APIs may extend multiple other APIs and if an
API included in the `extends` list has its own extensions then they are included
as well. Cycles are not allowed. All defined and extended method names must be
unique across the entire set.

### Constants

Constant values may be defined for any scalar type except `Data`:
```
const Foo :Text = "a constant value"
const Bar :UInt16 = 111
const Baz :Float32 = 1.2
const Yes :Bool = true
```

### SDKs

SDKs are a mirror of the API syntax intended for designing in-process code
bindings that are not hosted over a network. These are for building common,
cross-language SDKs or standard libraries.

```
sdk Foo {
    Bar() nothrows
    Baz(v :Int32) returns (:List<:Foo>)
}
```

SDK methods have more flexibility than API definitions because they are not
bound by the same statelessness requirement of APIs. Methods return nothing by
default unless a `returns` clause is added. Methods may optionally add a
`nothrows` clause to indicate that the method cannot fail. Method inputs are
always named and typed, similar to most programming language method signatures.
Finally, SDK methods can accept and return stateful objects such as APIs and
SDKs.

### Language Specification

The complete language and compiler specification is available at
https://microglot.org/docs/idl/specification.

## Roadmap

The initial development roadmap will focus on Go as the target language for code
generation and will expand to other target languages once the IDL is more
stable. The rough outline and order of features we're thinking about are:

- An `impl` type for defining implementation details
- An embedded scripting language to describe implementation logic that generates
  to code
- Ability to describe implementation dependencies
- Syntax for defining operational constraints such as SLOs and idempotency
- Generation of contract tests
- Native compiler plugin protocol
- Language expansion (Python, JS/TS, Java)
- Dev tooling (language server, syntax highlighting, etc.)

We don't have any timeline to offer for when these will be done or a guarantee
that any individual item will actually be worked on.

## Contributing

If you're interested in the project, what we need most is feedback on our ideas
and suggestions for what IDL related problems to solve. What do you think is
lacking in the current generation of IDLs? What aspects of system or API design
do you wish you could express? These are the sorts of questions we're asking
ourselves and would appreciate your thoughts.

For now, use https://github.com/microglot/mglotc/discussions to pitch ideas or
ask questions. We'll put an update there when we have other ways to engage.

## License

> © 2023 Microglot LLC

All content in this repository is licensed under Apache 2.0 except for the
dev container configuration which is under CC0-1.0. See the `LICENSES` directory
for the full text of either license. Each file in the repo is annotated with an
SPDX header or `.license` file identifying which license applies.
