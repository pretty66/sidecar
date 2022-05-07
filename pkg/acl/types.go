package acl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/utils"
)

type HeaderFunc func(key string) (string, bool)

var _supportKind = []string{"HTTPRouteGroup"}

type AclRules struct {
	Rules   []Rule      `json:"rules"`
	Sources SourceMatch `json:"sources"`
}

type Rule struct {
	Kind    string       `json:"kind"`
	Name    string       `json:"name"`
	Matches []*HTTPMatch `json:"matches"`
}

func NewACLRuleByConfig(conf []byte) (*AclRules, error) {
	var arule AclRules
	err := json.Unmarshal(conf, &arule)
	if err != nil {
		return nil, fmt.Errorf("acl rule resolution error：%s, %v", string(conf), err)
	}
	if len(arule.Sources) == 0 {
		return nil, fmt.Errorf("The ACL rule is incorrectly configured: Source must be configured！=> %s", string(conf))
	}
	for k := range arule.Rules {
		if !utils.InArray(arule.Rules[k].Kind, _supportKind) {
			return nil, fmt.Errorf("The ACL rule is incorrectly configured: KIND is not supported！=> %s", string(conf))
		}
		for i := range arule.Rules[k].Matches {
			hm, err := NewHTTPMatch(arule.Rules[k].Matches[i])
			if err != nil {
				return nil, err
			}
			arule.Rules[k].Matches[i] = hm
		}
	}
	return &arule, nil
}

type HTTPMatch struct {
	Name      string            `json:"name"`
	Methods   []string          `json:"methods"`
	PathRegex string            `json:"pathRegex"`
	Headers   map[string]string `json:"headers"`
	pRegex    *regexp.Regexp
	headRegex map[string]*regexp.Regexp
}

func NewHTTPMatch(hm *HTTPMatch) (*HTTPMatch, error) {
	if len(hm.Methods) == 0 && len(hm.PathRegex) == 0 && len(hm.Headers) == 0 {
		return nil, fmt.Errorf("The ACL matching rule is incorrectly configured, and all configuration items are empty！")
	}
	if len(hm.Methods) == 1 && hm.Methods[0] == "*" {
		hm.Methods = hm.Methods[:0]
	}
	for k := range hm.Methods {
		hm.Methods[k] = strings.ToUpper(hm.Methods[k])
	}
	if len(hm.PathRegex) > 0 {
		regex, err := regexp.Compile(hm.PathRegex)
		if err != nil {
			return nil, fmt.Errorf("the acl path regular expression is incorrect, %s, %v", hm, err)
		}
		hm.pRegex = regex
	}
	if len(hm.Headers) > 0 {
		hm.headRegex = make(map[string]*regexp.Regexp)
		for k := range hm.Headers {
			regex, err := regexp.Compile(hm.Headers[k])
			if err != nil {
				return nil, fmt.Errorf("the acl header regular expression is incorrect, %s: %s, %v", k, hm.Headers[k], err)
			}
			hm.headRegex[strings.ToLower(k)] = regex
		}
	}
	return hm, nil
}

func (hm *HTTPMatch) String() string {
	return string(utils.JSONEncode(hm))
}

func (hm *HTTPMatch) Matches(requestPath, method string, header HeaderFunc) error {
	if len(hm.Methods) > 0 && !utils.InArray(strings.ToUpper(method), hm.Methods) {
		return errno.RequestForbidden
	}
	if hm.pRegex != nil && !hm.pRegex.MatchString(requestPath) {
		return errno.RequestForbidden
	}
	if len(hm.headRegex) > 0 {
		for k := range hm.headRegex {
			val, ok := header(strings.ToLower(k))
			if !ok || !hm.headRegex[k].MatchString(val) {
				return errno.RequestForbidden
			}
		}
	}
	return nil
}

type SourceMatch []string

func (sm SourceMatch) CheckAllow(source string) bool {
	return utils.InArray(source, sm)
}

func (ar *AclRules) Verify(fromUniqueID, method, path string, head HeaderFunc) error {
	if !ar.Sources.CheckAllow(fromUniqueID) {
		return errno.RequestForbidden
	}
	if len(ar.Rules) == 0 {
		return nil
	}
	for k := range ar.Rules {
		for _, match := range ar.Rules[k].Matches {
			if match.Matches(path, method, head) == nil {
				return nil
			}
		}
	}
	return errno.RequestForbidden
}
