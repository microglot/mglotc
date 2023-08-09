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
)

const (
	CodeEOF = "_EOF_"
)

var (
	defaultNonFatal = map[string]bool{}
)
