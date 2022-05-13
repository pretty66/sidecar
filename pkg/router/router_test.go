package router

import (
	"fmt"
	util "github.com/openmsp/sidecar/utils"
	"log"
	"testing"
	"time"
)

func TestGetUniqueIDFromKey(t *testing.T) {
	uniqueID := "sdfsdfsdfsdf"
	key := fmt.Sprintf(RouterConfigKey, uniqueID)
	r := &Rule{}
	if r.getUniqueIDFromKey(key) != uniqueID {
		t.Fail()
	}
}

func TestWithRecover(t *testing.T) {
	var f func()
	f = func() {
		util.GoWithRecover(func() {
			testpanic()
		}, func(i interface{}) {
			f()
		})
	}
	f()
	time.Sleep(time.Minute)
}

func testpanic() {
	log.Println("into test panic, sleep 1")
	time.Sleep(time.Second)
	panic("panic test")
}
