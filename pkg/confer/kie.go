package confer

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/go-chassis/kie-client"
)

const defaultKey = "basic"

type RemoteBasicConfig struct{}

func GetRemoteConfig(project, addr, version string) (map[string]string, error) {
	c, err := kie.NewClient(kie.Config{
		Endpoint: addr,
	})
	if err != nil {
		panic(err)
	}
	var optional map[string]string
	if len(version) > 0 {
		optional = loadKV(c, project, version)
	}
	basic := loadKV(c, project, defaultKey)
	if basic == nil && optional == nil {
		return nil, nil
	}
	if basic == nil {
		return optional, nil
	}
	for k, v := range optional {
		basic[k] = v
	}
	return basic, nil
}

func loadKV(cli *kie.Client, project, key string) map[string]string {
	kvresp, _, err := cli.List(context.TODO(),
		kie.WithGetProject(project),
		kie.WithKey(key),
		kie.WithRevision(-1),
	)
	if errors.Is(err, kie.ErrNoChanges) {
		return nil
	}
	if err != nil {
		panic(err)
	}
	if kvresp.Total == 0 {
		return nil
	}
	kv := make(map[string]string)
	for k := range kvresp.Data {
		if kvresp.Data[k].Key != key {
			continue
		}

		err = json.Unmarshal([]byte(kvresp.Data[k].Value), &kv)
		if err != nil {
			log.Printf("error: remote configuration resolution failed procedure，key: %s, val: %s", kvresp.Data[k].Key, kvresp.Data[k].Value)
			return nil
		}
		log.Println("induction remote configuration：", key, kvresp.Data[k].Value)
		break
	}
	return kv
}
