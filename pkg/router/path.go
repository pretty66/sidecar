package router

import (
	"github.com/openmsp/sidecar/utils"
	"regexp"
	"strings"
)

// traffic matching rule
type BaseMatch struct {
	commonHeaderMatcherImpl
	AllowMethods []string
}

func (p *BaseMatch) Matches(_, method string, header HeaderFunc) bool {
	if len(p.AllowMethods) > 0 && !utils.InArray(strings.ToUpper(method), p.AllowMethods) {
		return false
	}
	return p.Match(header)
}

func createBaseMatch(methods []string, header []HeaderMatcher) *BaseMatch {
	for k := range methods {
		methods[k] = strings.ToUpper(methods[k])
	}
	return &BaseMatch{createHTTPHeaderMatcher(header), methods}
}

// ---------- prefix match
type PrefixMatch struct {
	*BaseMatch
	Prefix string
}

func (p *PrefixMatch) Matches(requestPath, method string, header HeaderFunc) bool {
	if len(p.AllowMethods) > 0 && !utils.InArray(strings.ToUpper(method), p.AllowMethods) {
		return false
	}
	if p.Prefix != "*" && strings.Index(requestPath, p.Prefix) != 0 {
		return false
	}
	return p.Match(header)
}

// -----------------------

// +++++++++++++++++ path match
type PathMatch struct {
	*BaseMatch
	Path string
}

func (p *PathMatch) Matches(requestPath, method string, header HeaderFunc) bool {
	if requestPath != p.Path {
		return false
	}
	if len(p.AllowMethods) > 0 && !utils.InArray(strings.ToUpper(method), p.AllowMethods) {
		return false
	}
	return p.Match(header)
}

// +++++++++++++++++++++++++++++++++

// *************** regex match
type RegexMatch struct {
	*BaseMatch
	RegexPattern *regexp.Regexp
}

func (p *RegexMatch) Matches(requestPath, method string, header HeaderFunc) bool {
	if len(p.AllowMethods) > 0 && !utils.InArray(strings.ToUpper(method), p.AllowMethods) {
		return false
	}
	if !p.RegexPattern.MatchString(requestPath) {
		return false
	}
	return p.Match(header)
}

// ************************
