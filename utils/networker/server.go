package networker

import (
	"errors"
	util "github.com/openmsp/sidecar/utils"
	"net"
	"net/http"
	"net/http/httputil"
	"time"
)

// networks
type ReverseProxyServerOption struct {
	ProtocolType string // eg:  "HTTP" "RESP" "HTTPS" "MTLS"
	BindNetWork  string // eg : unix,tcp
	BindAddress  string // eg: /tme/sidecar.sock 127.0.0.1:9090
	BindIP       string
	BindPort     string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	TargetProtocolType        string
	TargetAddress             string
	TargetDialTimeout         time.Duration
	TargetKeepAlive           time.Duration
	TargetIdleConnTimeout     time.Duration
	TargetMaxIdleConnsPerHost int
}

type HTTPReverseProxy struct {
	Option *ReverseProxyServerOption
	Server *http.Server
	Proxy  *httputil.ReverseProxy
}

var (
	ErrProtocolType  = errors.New("application layer protocol is not supported")
	ErrBindNetWork   = errors.New("network monitoring category is not supported")
	ErrBindAddress   = errors.New("bind address not invalid")
	ErrTargetAddress = errors.New("target address not invalid")
)

func CheckOption(option *ReverseProxyServerOption) error {
	if !util.InArray(option.ProtocolType, []string{"http", "https", "mtls"}) {
		return ErrProtocolType
	}
	if option.TargetProtocolType != "" && !util.InArray(option.TargetProtocolType, []string{"http", "https", "mtls"}) {
		return ErrProtocolType
	}
	if !util.InArray(option.BindNetWork, []string{"tcp", "unix"}) {
		return ErrBindNetWork
	}
	if option.BindAddress == "" {
		return ErrBindAddress
	}

	var err error
	option.BindIP, option.BindPort, err = net.SplitHostPort(option.BindAddress)
	if err != nil {
		return ErrBindAddress
	}

	if option.TargetAddress != "" {
		_, _, err = net.SplitHostPort(option.TargetAddress)
		if err != nil {
			return ErrTargetAddress
		}
	}
	return nil
}

func NewHTTPReverseProxyServer(option *ReverseProxyServerOption) (*HTTPReverseProxy, error) {
	err := CheckOption(option)
	if err != nil {
		return nil, err
	}
	if option.ReadTimeout.Seconds() <= 0 {
		option.ReadTimeout = 60 * time.Second
	}
	if option.WriteTimeout.Seconds() <= 0 {
		option.WriteTimeout = 60 * time.Second
	}
	hp := &HTTPReverseProxy{
		Option: option,
		Server: &http.Server{
			ReadTimeout:  option.ReadTimeout,
			WriteTimeout: option.WriteTimeout,
			IdleTimeout:  option.IdleTimeout,
			Addr:         option.BindAddress,
		},
		Proxy: &httputil.ReverseProxy{
			Director: func(request *http.Request) {
				return
			},
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: option.TargetKeepAlive,
				}).DialContext,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: option.TargetMaxIdleConnsPerHost,
			},
			BufferPool: NewBufferPool(),
		},
	}
	return hp, nil
}
