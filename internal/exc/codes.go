package exc

const (
	CodeUnknownFatal                  = "M0000"
	CodeFileNotFound                  = "M0001"
	CodeUnsuportedFileSystemOperation = "M0002"
	CodePermissionDenied              = "M0003"
	CodeUnsupportedFileFormat         = "M0004"
	CodeUnexpectedEOF                 = "M0005"
	CodeProtobufParseError            = "M0006"
	CodeInvalidNumber                 = "M0007"
	CodeUnexpectedToken               = "M0008"
	CodeInvalidLiteral                = "M0009"
	CodeUIDCollision                  = "M0010"
	CodeNameCollision                 = "M0011"
	CodeUnknownImport                 = "M0012"
	CodeUnknownType                   = "M0013"
	CodeUnknownIdentifier             = "M0014"
	CodeUnknownReference              = "M0015"
	CodeUnresolvedReference           = "M0016"
	CodeTypeParameterError            = "M0017"
	CodeWrongTypeKind                 = "M0018"
	CodeWrongTypeForAPI               = "M0019"
	CodeWrongTypeValue                = "M0020"
	CodeUnimplemented                 = "M0021"
	CodeUnknownFieldInStructLiteral   = "M0022"
)

const (
	CodeEOF = "_EOF_"
)

var (
	defaultNonFatal = map[string]bool{}
)
