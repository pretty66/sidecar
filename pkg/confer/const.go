package confer

import "net/http"

const (
	REQUEST_HEADER_FROM_UNIQUEID = "x-from-uniqueid"
	REQUEST_HEADER_CONTENT_TYPE  = "content-type"

	REQUEST_BODY_KEY = "request_body"

	MSP_SERVICE_RULE_REDIS_KEY = "msp:service_rule:%s"

	// trace Data size limit
	MaxTraceDataSize = 512 * 1024
	// Flow traction label
	HTTPRouterFilter = "x-router-filter"
	// Traffic Mirroring Identifier
	HTTPRequestTypeKey = "x-request-type"
	HTTPShadowRequest  = "shadow"
	// Request life cycle Key: List of functions executed after the agent requests a response
	RespCacheProxyRespFunc = "_resp_cache_proxy_response_func"

	HeaderAppID     = "msp-app-id"
	AppIDKey        = "appIDKey"
	AppNamespaceKey = "appNamespaceKey"
	AppInfoKey      = "appInfoKey"
)

var HopHeaders = []string{
	"Connection",          // Connection
	"Proxy-Connection",    // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",          // Keep-Alive
	"Proxy-Authenticate",  // Proxy-Authenticate
	"Proxy-Authorization", // Proxy-Authorization
	"Te",                  // canonicalized version of "TE"
	"Trailer",             // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",   // Transfer-Encoding
	"Upgrade",             // Upgrade
	// "Content-Length",
}

// before the proxy request is sent
type BeforeProxyRequest func(req *http.Request) error

// after the proxy request is sent
type AfterProxyResponse func(response *http.Response) error
