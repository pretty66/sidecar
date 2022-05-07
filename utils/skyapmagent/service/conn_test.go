package service

import (
	"bytes"
	"testing"
)

func TestByte(t *testing.T) {
	target := []byte(`aa
bbb
ccc`)
	index := bytes.IndexByte(target, '\n')
	if index != 2 {
		t.Fatal(index)
	}
}
