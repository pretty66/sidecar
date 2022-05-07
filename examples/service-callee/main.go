package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

const port = 3000

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Status: "OK"})
}

func defaultHandler(w http.ResponseWriter, _ *http.Request) {
	log.Printf("defaultHandler is called\n")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Status: "OK", Data: "response form callee"})
}
func main() {
	r := mux.NewRouter()
	r.HandleFunc("healthz", healthHandler)
	r.HandleFunc("call", defaultHandler)
	log.Println(http.ListenAndServe(fmt.Sprintf(":%d", port), r))
}
