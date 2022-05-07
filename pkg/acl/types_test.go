package acl

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/openmsp/sidecar/pkg/errno"
)

func TestNewAclRuleByConfig(t *testing.T) {
	conf := AclRules{
		Rules: []Rule{
			{
				Kind: "HTTPRouteGroup",
				Name: "rule",
				Matches: []*HTTPMatch{
					{
						Name:      "test_http",
						Methods:   []string{"gEt", "Post"},
						PathRegex: ".*index",
					},
					{
						Name:      "test_http2",
						Methods:   []string{"put"},
						PathRegex: "/put.*",
					},
					{
						Name:    "test_http3",
						Methods: []string{"get"},
						Headers: map[string]string{
							"cookie":          `\d{3,9}`,
							"Accept-Encoding": "gzip",
						},
						PathRegex: "/header",
					},
				},
			},
		},
		Sources: SourceMatch{"a"},
	}
	b, err := json.Marshal(conf)
	if err != nil {
		t.Fatal(err)
	}
	acl, err := NewACLRuleByConfig(b)
	if err != nil {
		t.Fatal(err)
	}
	testArg := []struct {
		fromUniqueID string
		method       string
		path         string
		head         HeaderFunc
		err          error
	}{
		{"a", "get", "/index", nil, nil},
		{"b", "get", "/index", nil, errno.RequestForbidden},
		{"a", "post", "/index", nil, nil},
		{"a", "delete", "/index", nil, errno.RequestForbidden},
		{"a", "put", "/index", nil, errno.RequestForbidden},
		{"a", "put", "putwqe", nil, errno.RequestForbidden},
		{"a", "put", "/putwqe", nil, nil},
		{"a", "put", "/header", nil, errno.RequestForbidden},
		{"a", "get", "/header", func(key string) (string, bool) {
			header := map[string]string{
				"cookie":          "1312123",
				"accept-encoding": "gzip",
			}
			val, ok := header[key]
			return val, ok
		}, nil},
	}

	for k := range testArg {
		err := acl.Verify(testArg[k].fromUniqueID, testArg[k].method, testArg[k].path, testArg[k].head)
		if !errors.Is(err, testArg[k].err) {
			t.Fatal(testArg[k].fromUniqueID, testArg[k].method, testArg[k].path, err)
		}
	}
}
