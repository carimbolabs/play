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
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("cmake", "--build", ".")
	cmd.Dir = "/opt/carimbo/build"
	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run()
	// err := cmd.Run()
	// if err != nil {
	// 	w.Header().Set("Content-Type", "application/json")
	// 	json.NewEncoder(w).Encode(message)

	// 	return
	// }

	message := Message{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil)
}
