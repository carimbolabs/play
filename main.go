package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type Message struct {
	Text string `json:"message"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	message := Message{
		Text: "Hello, world!",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil)
}
