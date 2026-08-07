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
	ErrKindTimeout
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
	case ErrKindTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// Error is the taxonomy every provider implementation maps its SDK-specific
// errors into at the call boundary.
//
// Message states what went wrong, in sentence case. Hint, when present,
// states what the user should do about it — it is kept separate so the CLI
// can render it as its own indented line under the message, per smartly's
// error vocabulary:
//
//	× No Anthropic API key found.
//	  Set ANTHROPIC_API_KEY, or choose another provider with --provider.
type Error struct {
	Kind    ErrorKind
	Message string
	Hint    string
	Cause   error
}

// Error returns message and hint as one string, so an Error rendered through
// a plain %v or wrapped by another error still carries its next step.
func (e *Error) Error() string {
	if e.Hint == "" {
		return e.Message
	}
	return e.Message + " " + e.Hint
}

func (e *Error) Unwrap() error { return e.Cause }
