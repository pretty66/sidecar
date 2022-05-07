package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/openmsp/cilog"
)

func initLog() {
	configLogData := &cilog.ConfigLogData{
		OutPut: "stdout",
		Debug:  false,
		Key:    "sc_test_log",
	}
	configAppData := &cilog.ConfigAppData{
		AppName:    "test",
		AppID:      "test-appid",
		AppVersion: "0.0.0",
	}
	cilog.ConfigLogInit(configLogData, configAppData)
}

func TestNetHttpResponseHandle(t *testing.T) {
	initLog()
	e := echo.New()

	e.Use(NetHTTPResponseHandle)

	e.GET("/4k", func(ctx echo.Context) error {
		ctx.Response().WriteHeader(http.StatusOK)
		ctx.Response().Write(bytes.Repeat([]byte("a"), 1024*4))
		return nil
	})

	e.GET("/65k", func(ctx echo.Context) error {
		ctx.Response().WriteHeader(http.StatusOK)
		ctx.Response().Write(bytes.Repeat([]byte("a"), 1024*65))
		return nil
	})

	e.GET("/error", func(ctx echo.Context) error {
		ctx.Response().WriteHeader(http.StatusInternalServerError)
		ctx.Response().Write(bytes.Repeat([]byte("a"), 1024*1))
		return nil
	})

	go func() {
		select {
		case <-time.After(time.Second * 1):
			e.Close()
		}
	}()

	_ = e.Start(":40000")
}

func TestJsonError(t *testing.T) {
	s := struct {
		Type string `json:"type"`
		Err  error  `json:"err"`
	}{"asdasd", errors.New("1231231231")}
	b, _ := json.Marshal(s)
	t.Log(string(b))
}
