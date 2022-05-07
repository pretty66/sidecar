package router

import (
	"errors"
	"regexp"

	util "github.com/openmsp/sidecar/utils"
)

type HeaderFunc func(key string) (string, bool)

// matching
type Matchable interface {
	Matches(path, method string, headers HeaderFunc) bool
}

// control plane configuration parameters
type RouterConfig struct {
	ToUniqueID string   `json:"to_unique_id"` // can not be empty
	Timestamp  int64    `json:"timestamp"`    // Last update time, 13-digit millisecond timestamp
	Routers    []Router `json:"routers"`      // Internal at least one router configuration
}

// A specific url rule matches a specific target
type Router struct {
	// can not be empty
	Match RouterMatch `json:"match"`
	// Traffic pulling at least one route
	Routes []Route `json:"routes"`
	// Traffic mirroring
	Mirror []Mirror `json:"mirror"`
}

// Specific target instance configuration
type Route struct {
	// The attribute metadata contained in the target instance, if it does not match the target,
	// an error will be reported in the request; the configuration cannot be empty
	Metadata map[string]string `json:"metadata"`
	// The weight of the load to the specific instance,
	// the default is 10, and it can be set from 1 to 10.
	Weight int `json:"weight,omitempty"`
}

// request url matching rules
// When the priority prefix > path > regex path matching configuration exists at the same time,
// only the configuration with the higher priority will take effect.
// There is an and relationship between attributes,
// which must be satisfied at the same time to be considered a successful match
type RouterMatch struct {
	Methods []string        `json:"methods"`
	Prefix  string          `json:"prefix,omitempty"`
	Path    string          `json:"path,omitempty"`
	Regex   string          `json:"regex,omitempty"`
	Headers []HeaderMatcher `json:"headers,omitempty"`
}

// header
type HeaderMatcher struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Regex bool   `json:"regex,omitempty"`
}

// KeyValueMatchType
type KeyValueMatchType uint32

// Key value match patterns
const (
	ValueExact KeyValueMatchType = iota
	ValueRegex
)

type StringMatch struct {
	Value        string
	IsRegex      bool
	RegexPattern *regexp.Regexp
}

type KeyValueData struct {
	Name  string // name should be lower case in router headerdata
	Value StringMatch
}

func (r *RouterConfig) String() string {
	return string(util.JSONEncode(r))
}

func (sm StringMatch) Matches(s string) bool {
	if !sm.IsRegex {
		return s == sm.Value
	}
	if sm.RegexPattern != nil {
		return sm.RegexPattern.MatchString(s)
	}
	return false
}

func (k *KeyValueData) Key() string {
	return k.Name
}

func (k *KeyValueData) MatchType() KeyValueMatchType {
	if k.Value.IsRegex {
		return ValueRegex
	}
	return ValueExact
}

func (k *KeyValueData) Matcher() string {
	return k.Value.Value
}

func NewKeyValueData(header HeaderMatcher) (*KeyValueData, error) {
	kvData := &KeyValueData{
		Name: header.Name,
		Value: StringMatch{
			Value:   header.Value,
			IsRegex: header.Regex,
		},
	}
	if header.Regex {
		p, err := regexp.Compile(header.Value)
		if err != nil {
			return nil, err
		}
		kvData.Value.RegexPattern = p
	}
	return kvData, nil
}

type commonHeaderMatcherImpl []*KeyValueData

func (m commonHeaderMatcherImpl) Match(headers HeaderFunc) bool {
	for _, headerData := range m {
		cfgName := headerData.Name
		// if a condition is not matched, return false
		// ll condition matched, return true
		value, exists := headers(cfgName)
		if !exists {
			return false
		}
		if !headerData.Value.Matches(value) {
			return false
		}
	}
	return true
}

func createHTTPHeaderMatcher(header []HeaderMatcher) commonHeaderMatcherImpl {
	match := make(commonHeaderMatcherImpl, 0, len(header))
	for k := range header {
		if kv, err := NewKeyValueData(header[k]); err == nil {
			match = append(match, kv)
		}
	}
	return match
}

func NewMatches(match RouterMatch) (Matchable, error) {
	var matchable Matchable
	switch {
	case match.Prefix != "":
		matchable = &PrefixMatch{
			BaseMatch: createBaseMatch(match.Methods, match.Headers),
			Prefix:    match.Prefix,
		}
	case match.Path != "":
		matchable = &PathMatch{
			BaseMatch: createBaseMatch(match.Methods, match.Headers),
			Path:      match.Path,
		}
	case match.Regex != "":
		regPattern, err := regexp.Compile(match.Regex)
		if err != nil {
			return nil, errors.New("the routing regular rule is incorrect" + err.Error())
		}
		matchable = &RegexMatch{
			BaseMatch:    createBaseMatch(match.Methods, match.Headers),
			RegexPattern: regPattern,
		}
	default:
		if len(match.Headers) == 0 {
			return nil, errors.New("the route configuration rule is empty")
		}
		matchable = createBaseMatch(match.Methods, match.Headers)
	}
	return matchable, nil
}

type Mirror struct {
	Host     string            `json:"host"`
	Metadata map[string]string `json:"metadata"`
	Percent  int               `json:"percent"`
}
