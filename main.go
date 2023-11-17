package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Runtime struct {
	Script []byte
	Binary []byte
}

type Cache struct {
	sync.RWMutex
	runtimes map[string]Runtime
}

var (
	//go:embed index.html
	html  []byte
	cache Cache
)

func init() {
	cache.runtimes = make(map[string]Runtime)
}

func getRuntime(runtime string) (Runtime, error) {
	cache.Lock()
	defer cache.Unlock()

	if cached, ok := cache.runtimes[runtime]; ok {
		return cached, nil
	}

	url := fmt.Sprintf("https://github.com/carimbolabs/carimbo/releases/download/v%s/WebAssembly.zip", runtime)
	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Runtime{}, fmt.Errorf("http request error: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Runtime{}, fmt.Errorf("http get error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Runtime{}, fmt.Errorf("read all error: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return Runtime{}, fmt.Errorf("zip reader error: %w", err)
	}

	readFile := func(file *zip.File) ([]byte, error) {
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open zip file: %w", err)
		}
		defer rc.Close()

		return io.ReadAll(rc)
	}

	var scriptContent, binaryContent []byte
	for _, file := range zr.File {
		switch file.Name {
		case "carimbo.js":
			scriptContent, err = readFile(file)
			if err != nil {
				return Runtime{}, fmt.Errorf("readfile error: %w", err)
			}
		case "carimbo.wasm":
			binaryContent, err = readFile(file)
			if err != nil {
				return Runtime{}, fmt.Errorf("readfile error: %w", err)
			}
		}
	}

	rt := Runtime{Script: scriptContent, Binary: binaryContent}
	cache.runtimes[runtime] = rt
	return rt, nil
}

func getBundle(org, repo, release string) ([]byte, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/bundle.7z", org, repo, release)

	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http request error: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read all error: %w", err)
	}

	return body, nil
}

type Params struct {
	Runtime      string `param:"runtime"`
	Organization string `param:"org"`
	Repository   string `param:"repo"`
	Release      string `param:"release"`
}

func (p *Params) Sha1() string {
	triplet := fmt.Sprintf("v1%s%s%s%s", p.Runtime, p.Organization, p.Repository, p.Release)

	hash := sha1.New()
	//nolint:errcheck
	io.WriteString(hash, triplet)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func indexHandler(c echo.Context) error {
	p := Params{}
	if err := c.Bind(&p); err != nil {
		return fmt.Errorf("parse parameters error: %w", err)
	}

	data := struct {
		BaseURL string
	}{
		BaseURL: fmt.Sprintf("/%s/%s/%s/%s/", p.Runtime, p.Organization, p.Repository, p.Release),
	}

	tmpl, err := template.New("index").Parse(string(html))
	if err != nil {
		return fmt.Errorf("parse template error: %w", err)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=300, s-maxage=300")

	if err := tmpl.Execute(c.Response().Writer, data); err != nil {
		return fmt.Errorf("execute template error: %w", err)
	}

	return nil
}

func favIconHandler(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000")
	c.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
	return c.Blob(http.StatusOK, "image/x-icon", []byte{})
}

func javaScriptHandler(c echo.Context) error {
	p := Params{}
	if err := c.Bind(&p); err != nil {
		return fmt.Errorf("parse parameters error: %w", err)
	}

	runtime, err := getRuntime(p.Runtime)
	if err != nil {
		return fmt.Errorf("get runtime error: %w", err)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000")
	c.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
	c.Response().Header().Set("ETag", p.Sha1())

	return c.Blob(http.StatusOK, "application/javascript", runtime.Script)
}

func webAssemblyHandler(c echo.Context) error {
	p := Params{}
	if err := c.Bind(&p); err != nil {
		return fmt.Errorf("parse parameters error: %w", err)
	}

	runtime, err := getRuntime(p.Runtime)
	if err != nil {
		return fmt.Errorf("get runtime error: %w", err)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000")
	c.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
	c.Response().Header().Set("ETag", p.Sha1())

	return c.Blob(http.StatusOK, "application/wasm", runtime.Binary)
}

func bundleHandler(c echo.Context) error {
	p := Params{}
	if err := c.Bind(&p); err != nil {
		return fmt.Errorf("parse parameters error: %w", err)
	}

	bundle, err := getBundle(p.Organization, p.Repository, p.Release)
	if err != nil {
		return fmt.Errorf("get bundle error: %w", err)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000")
	c.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
	c.Response().Header().Set("ETag", p.Sha1())

	return c.Blob(http.StatusOK, "application/octet-stream", bundle)
}

func main() {
	e := echo.New()
	e.Pre(middleware.RemoveTrailingSlash())
	e.Pre(middleware.GzipWithConfig(middleware.GzipConfig{MinLength: 2048}))

	e.GET("/:runtime/:org/:repo/:release", indexHandler)
	e.GET("/:runtime/:org/:repo/:release/carimbo.js", javaScriptHandler)
	e.GET("/:runtime/:org/:repo/:release/carimbo.wasm", webAssemblyHandler)
	e.GET("/:runtime/:org/:repo/:release/bundle.7z", bundleHandler)
	e.GET("/favicon.ico", favIconHandler)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", os.Getenv("PORT"))))
}
