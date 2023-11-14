package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Runtime struct {
	Script string
	Binary string
}

type Cache struct {
	sync.Mutex
	runtimes map[string]Runtime
}

var (
	//go:embed index.html
	html  []byte
	cache Cache
	// toCache = regexp.MustCompile(`\.(zip|wasm|js|ico)$`)
)

func init() {
	cache.runtimes = make(map[string]Runtime)
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

func removeRootDirFromZip(zipData []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	var modifiedZipBuffer bytes.Buffer
	writer := zip.NewWriter(&modifiedZipBuffer)

	for _, file := range reader.File {
		file.Name = strings.Join(strings.Split(file.Name, "/")[1:], "/")

		destFile, err := writer.Create(file.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip file: %w", err)
		}

		srcFile, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open zip file: %w", err)
		}
		defer srcFile.Close()

		if _, err = io.Copy(destFile, srcFile); err != nil {
			return nil, fmt.Errorf("failed to copy file contents: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return modifiedZipBuffer.Bytes(), nil
}

func fetchRuntime(runtime string) (Runtime, error) {
	cache.Lock()
	defer cache.Unlock()

	if cached, ok := cache.runtimes[runtime]; ok {
		return cached, nil
	}

	url := fmt.Sprintf("https://github.com/carimbolabs/carimbo/releases/download/v%s/WebAssembly.zip", runtime)

	resp, err := http.Get(url)
	if err != nil {
		return Runtime{}, fmt.Errorf("HTTP GET error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Runtime{}, fmt.Errorf("readAll error: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return Runtime{}, fmt.Errorf("zip.NewReader error: %w", err)
	}

	var scriptContent, binaryContent []byte
	for _, file := range zr.File {
		switch file.Name {
		case "carimbo.js":
			scriptContent, err = readZipFile(file)
			if err != nil {
				return Runtime{}, fmt.Errorf("readZipFile error: %w", err)
			}
		case "carimbo.wasm":
			binaryContent, err = readZipFile(file)
			if err != nil {
				return Runtime{}, fmt.Errorf("readZipFile error: %w", err)
			}
		}
	}

	rt := Runtime{Script: string(scriptContent), Binary: string(binaryContent)}
	cache.runtimes[runtime] = rt
	return rt, nil
}

func fetchBundle(org, repo, release string) ([]byte, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/v%s.zip", org, repo, release)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("readAll error: %w", err)
	}

	body, err = removeRootDirFromZip(body)
	if err != nil {
		return nil, fmt.Errorf("removeRootDirFromZip error: %w", err)
	}

	return body, nil
}

func getOrgRepoReleaseFromURL(urlPath string) (string, string, string, string) {
	pattern := regexp.MustCompile(`/play/(?P<runtime>[^/]+)/(?P<org>[^/]+)/(?P<repo>[^/]+)/(?P<release>[^/]+)`)
	match := pattern.FindStringSubmatch(urlPath)

	var runtime, org, repo, release string
	for i, name := range pattern.SubexpNames() {
		if i != 0 && name != "" {
			switch name {
			case "runtime":
				runtime = match[i]
			case "org":
				org = match[i]
			case "repo":
				repo = match[i]
			case "release":
				release = match[i]
			}
		}
	}

	return runtime, org, repo, release
}

func getRuntimeFromURL(urlPath string) string {
	runtime, _, _, _ := getOrgRepoReleaseFromURL(urlPath)
	return runtime
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)

	_, err := w.Write(data)
	if err != nil {
		log.Printf("Error writing response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func javaScriptHandler(w http.ResponseWriter, r *http.Request) {
	runtime, err := fetchRuntime(getRuntimeFromURL(r.URL.Path))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching runtime: %v", err), http.StatusInternalServerError)
		return
	}

	serveStaticFile(w, r, "application/javascript", []byte(runtime.Script))
}

func webAssemblyHandler(w http.ResponseWriter, r *http.Request) {
	runtime, err := fetchRuntime(getRuntimeFromURL(r.URL.Path))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching runtime: %v", err), http.StatusInternalServerError)
		return
	}

	serveStaticFile(w, r, "application/wasm", []byte(runtime.Binary))
}

func bundleHandler(w http.ResponseWriter, r *http.Request) {
	_, org, repo, release := getOrgRepoReleaseFromURL(r.URL.Path)
	bundle, err := fetchBundle(org, repo, release)
	if err != nil {
		log.Printf("[fetchBundle]: error %v", err)
		http.Error(w, fmt.Sprintf("Error fetching bundle: %v", err), http.StatusInternalServerError)
		return
	}

	serveStaticFile(w, r, "application/zip", bundle)
}

func favIconHandler(w http.ResponseWriter, r *http.Request) {
	serveStaticFile(w, r, "image/x-icon", []byte{})
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("index").Parse(string(html))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing template: %v", err), http.StatusInternalServerError)
		return
	}

	baseURL := fmt.Sprintf("%s/", strings.TrimRight(filepath.Join("/", path.Clean(r.URL.Path)), "/"))

	data := struct {
		BaseURL string
	}{
		BaseURL: baseURL,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
		return
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")

		// if toCache.MatchString(r.URL.Path) {
		// 	w.Header().Set("Cache-Control", "public, max-age=31536000")
		// 	w.Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
		// 	w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		// } else {
		// 	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		// }

		gzipWriter := gzip.NewWriter(w)
		defer gzipWriter.Close()

		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gzipWriter}, r)
	})
}

func main() {
	server := &http.Server{
		Addr: fmt.Sprintf(":%s", os.Getenv("PORT")),
		Handler: Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			switch {
			case strings.HasSuffix(path, ".js"):
				javaScriptHandler(w, r)
			case strings.HasSuffix(path, ".wasm"):
				webAssemblyHandler(w, r)
			case strings.HasSuffix(path, ".zip"):
				bundleHandler(w, r)
			case strings.HasSuffix(path, ".ico"):
				favIconHandler(w, r)
			default:
				rootHandler(w, r)
			}
		})),
	}

	log.Fatal(server.ListenAndServe())
}
