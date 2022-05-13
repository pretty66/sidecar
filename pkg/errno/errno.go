package errno

import (
	"encoding/json"
	"log"
)

type SCError struct {
	State int    `json:"state"`
	Msg   string `json:"msg"`
}

func (e SCError) Error() string {
	return e.Msg
}

func (e SCError) Add(s string) *SCError {
	e.Msg += ": " + s
	return &e
}

func (e *SCError) Encode() []byte {
	b, err := json.Marshal(e)
	if err != nil {
		log.Println(err)
		return nil
	}
	return b
}

func (e *SCError) String() string {
	return string(e.Encode())
}
