package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

const port = 3001

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Status: "OK"})
}

func callHandler(w http.ResponseWriter, _ *http.Request) {
	log.Printf("defaultHandler is called\n")

	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse("http://localhost:8080/call")
	req.Header.Set("msp-app-id", "callee")

	transport := &http.Transport{DialContext: net.Dialer{Timeout: 5 * time.Second}.DialContext}
	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Status: "Request Err", Message: err.Error()})
		return
	}
	res, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Status: "Read Response Err", Message: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Status: "OK", Data: string(res)})
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("healthz", healthHandler)
	r.HandleFunc("call", callHandler)
	log.Println(http.ListenAndServe(fmt.Sprintf(":%d", port), r))
}
