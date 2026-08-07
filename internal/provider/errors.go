package provider

// ErrorKind classifies a provider failure so callers can print an actionable
// message without knowing which SDK's error types produced it.
type ErrorKind int

const (
	ErrKindAuth ErrorKind = iota
	ErrKindRateLimit
	ErrKindOverloaded
	ErrKindNetwork
	ErrKindInvalid
	ErrKindUnknown
)

// String returns the stable name used in log entries.
func (k ErrorKind) String() string {
	switch k {
	case ErrKindAuth:
		return "auth"
	case ErrKindRateLimit:
		return "rate_limit"
	case ErrKindOverloaded:
		return "overloaded"
	case ErrKindNetwork:
		return "network"
	case ErrKindInvalid:
		return "invalid"
	default:
		return "unknown"
	}
}

// Error is the taxonomy every provider implementation maps its SDK-specific
// errors into at the call boundary.
type Error struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }
