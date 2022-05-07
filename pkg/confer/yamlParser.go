package confer

import (
	"errors"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// parseYamlFromBytes data source is []byte
func parseYamlFromBytes(orignData []byte) (data scConfS, err error) {
	if len(orignData) == 0 {
		err = errors.New("yaml source data is empty")
		return
	}

	err = yaml.Unmarshal(orignData, &data)

	return
}

// parseYamlFromFile data source is file path.
func parseYamlFromFile(yamlFileURI string) (data scConfS, err error) {
	var fileData []byte
	fileData, err = ioutil.ReadFile(yamlFileURI)
	if err != nil {
		return
	}
	data, err = parseYamlFromBytes(fileData)
	return
}
