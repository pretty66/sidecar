package confer

import "time"

type RemoteApp struct {
	AppID       string
	Scheme      string
	Host        string
	Path        string
	Hostname    string
	IsRetry     bool
	IsKnative   bool
	ExtraHeader map[string]string
	Timeout     time.Duration
}
