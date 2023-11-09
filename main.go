package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
)

type Message struct {
	Text string `json:"message"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("cmake", "--build", ".")

	var (
		out    bytes.Buffer
		output string
	)

	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	output = out.String()

	message := Message{
		Text: output,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil)
}
