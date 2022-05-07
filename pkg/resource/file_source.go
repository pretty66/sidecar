package resource

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

func ReadNumberFromRemote(remoteURL, name string) (n uint64, err error) {
	data, err := RemoteGetSourceString(remoteURL, name)
	if err != nil {
		return 0, err
	}
	n, err = strconv.ParseUint(strings.TrimSpace(data), 10, 64)
	if err != nil {
		return n, err
	}
	return n, nil
}

func ReadNumberFromFile(name string) (n uint64, err error) {
	file, err := os.Open(name)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return n, err
	}

	n, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return n, err
	}
	return n, nil
}

func ReadIntFromRemote(remoteURL, name string) (n int64, err error) {
	data, err := RemoteGetSourceString(remoteURL, name)
	if err != nil {
		return 0, err
	}
	n, err = strconv.ParseInt(strings.TrimSpace(data), 10, 64)
	if err != nil {
		return n, err
	}
	return n, nil
}

func ReadIntFromFile(name string) (n int64, err error) {
	file, err := os.Open(name)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return n, err
	}

	n, err = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return n, err
	}
	return n, nil
}

func ReadMapFromRemote(remoteURL, name string) (m map[string]uint64, err error) {
	ret, err := RemoteGetSourceByte(remoteURL, name)
	if err != nil {
		return nil, err
	}
	m = make(map[string]uint64)
	reader := bytes.NewReader(ret)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}
		m[parts[0]] = v
	}

	return m, nil
}

func ReadMapFromFile(name string) (m map[string]uint64, err error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	m = make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}
		m[parts[0]] = v
	}

	return m, nil
}
