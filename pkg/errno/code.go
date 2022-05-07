package errno

var (
	// -200 ~ -100
	RequestError  = &Errno{-105, "Request error"}
	ResponseError = &Errno{-106, "Response error"}
	CaCertRevoke  = &Errno{-110, "server certificate error: "}

	FuseError = &Errno{-499, "The fuse error"}

	LimiterError      = &Errno{-429, "Rate limit exceeded"}
	BbrLimiterError   = &Errno{-430, "BBR Rate limit exceeded"}
	CodelLimiterError = &Errno{-431, "codel Rate limit exceeded"}

	TargetResponseWaring = &Errno{-300, "target service response waring"}

	// acl
	RequestForbidden = &Errno{-403, "Request forbidden"}
)
