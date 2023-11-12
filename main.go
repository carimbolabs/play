package main

import (
	"archive/zip"
	_ "embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

//go:embed index.html
var html []byte

type Runtime struct {
	Script string
	Binary string
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

func runtime(release string, r chan Runtime, errCh chan error) {
	url := fmt.Sprintf("https://github.com/carimbolabs/carimbo/releases/download/v%s/WebAssembly.zip", release)

	// Baixar o arquivo ZIP
	fmt.Println("Baixando o arquivo ZIP...")
	resp, err := http.Get(url)
	if err != nil {
		errCh <- err
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errCh <- err
		return
	}
	// Ler o conteúdo do arquivo ZIP
	fmt.Println("Lendo o conteúdo do arquivo ZIP...")
	// Criar um leitor para o corpo da resposta
	readerAt := strings.NewReader(string(body))

	// Criar o leitor ZIP usando io.ReaderAt
	zr, err := zip.NewReader(readerAt, int64(len(body)))
	if err != nil {
		errCh <- err
		return
	}

	// Procurar pelos arquivos desejados no ZIP
	var scriptContent, binaryContent []byte
	for _, file := range zr.File {
		switch file.Name {
		case "carimbo.js":
			scriptContent, err = readZipFile(file)
			if err != nil {
				errCh <- err
				return
			}
		case "carimbo.wasm":
			binaryContent, err = readZipFile(file)
			if err != nil {
				errCh <- err
				return
			}
		}
	}

	r <- Runtime{
		Script: string(scriptContent),
		Binary: string(binaryContent),
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	pattern := regexp.MustCompile(`/(?P<runtime>[^/]+)/(?P<org>[^/]+)/(?P<repo>[^/]+)/(?P<version>[^/]+)`)

	match := pattern.FindStringSubmatch(r.URL.Path)
	if len(match) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	matches := make(map[string]string)
	for i, name := range pattern.SubexpNames() {
		if i != 0 && name != "" {
			matches[name] = match[i]
		}
	}

	if len(matches) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	runtimeCh := make(chan Runtime)
	errCh := make(chan error)
	go runtime(matches["runtime"], runtimeCh, errCh)

	var runtime Runtime
	select {
	case runtime = <-runtimeCh:
	case err := <-errCh:
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Printf("Runtime: %+v\n", runtime.Script)
	html = []byte(strings.ReplaceAll(string(html), "{{script}}", runtime.Script))

	w.Write(html)
}

func main() {
	http.HandleFunc("/", handler)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil))
}
