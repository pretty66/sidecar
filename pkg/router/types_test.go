package router

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJsonRouterConfig(t *testing.T) {
	conf := &Config{
		ToUniqueID: "sidecar.proxy",
		Timestamp:  time.Now().Unix() / 1e6,
		Routers: []Router{
			{
				Match: Match{
					Prefix: "*",
					Path:   "1",
					Regex:  "2",
					Headers: []HeaderMatcher{
						{
							Name:  "a",
							Value: "c",
							Regex: true,
						},
					},
				},
				Routes: []Route{
					{
						Metadata: map[string]string{
							"runtime": "production",
						},
						Weight: 10,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(conf)
	t.Log(string(b))
}
