package cpu

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func remoteGetSourceString(directory string) (data string, err error) {
	client := &http.Client{Timeout: time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", os.Getenv("REMOTE_RESOURCE_URL"), directory), nil)
	if err != nil {
		return data, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()
	ret, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	data = strings.TrimSpace(string(ret))
	return data, nil
}

func remoteGetSourceByte(directory string) (ret []byte, err error) {
	client := &http.Client{Timeout: time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", os.Getenv("REMOTE_RESOURCE_URL"), directory), nil)
	if err != nil {
		return ret, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()
	ret, err = ioutil.ReadAll(resp.Body)
	return
}

func remoteGetSourceReadLines(directory string) ([]string, error) {
	return remoteReadLinesOffsetN(directory, 0, -1)
}

func remoteReadLinesOffsetN(filename string, offset uint, n int) ([]string, error) {
	data, err := remoteGetSourceByte(filename)
	if err != nil {
		return nil, err
	}
	var r bytes.Buffer
	r.Write(data)
	var ret []string
	var line string
	for i := 0; i < n+int(offset) || n < 0; i++ {
		line, err = r.ReadString('\n')
		if err != nil {
			break
		}
		if i < int(offset) {
			continue
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}
	return ret, err
}
