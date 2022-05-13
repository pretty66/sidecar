package errno

var (
	// -200 ~ -100
	ErrRequest      = &SCError{-105, "Request error"}
	ErrResponse     = &SCError{-106, "Response error"}
	ErrCaCertRevoke = &SCError{-110, "server certificate error: "}
	ErrFuse         = &SCError{-499, "The fuse error"}
	ErrLimiter      = &SCError{-429, "Rate limit exceeded"}
	ErrBBRLimiter   = &SCError{-430, "BBR Rate limit exceeded"}
	ErrCodelLimiter = &SCError{-431, "codel Rate limit exceeded"}

	ErrTargetResponseWaring = &SCError{-300, "target service response waring"}

	// acl
	ErrRequestForbidden = &SCError{-403, "Request forbidden"}
)
