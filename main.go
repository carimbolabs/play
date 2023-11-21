package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
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
	runtimes sync.Map
	bundles  sync.Map
}

var (
	//go:embed index.html
	html []byte
	//go:embed assets
	assets embed.FS
	cache  Cache
)

func getRuntime(runtime string) (Runtime, error) {
	url := fmt.Sprintf("https://github.com/flippingpixels/carimbo/releases/download/v%s/WebAssembly.zip", runtime)

	if cached, ok := cache.runtimes.Load(runtime); ok {
		return cached.(Runtime), nil
	}

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

	if (resp == nil) || (resp.StatusCode != http.StatusOK) {
		return Runtime{}, fmt.Errorf("http status error: %d", resp.StatusCode)
	}

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
	cache.runtimes.Store(runtime, rt)
	return rt, nil
}

func getBundle(org, repo, release string) ([]byte, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/bundle.7z", org, repo, release)

	if cached, ok := cache.bundles.Load(url); ok {
		return cached.([]byte), nil
	}

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

	cache.bundles.Store(url, body)

	return body, nil
}

type Params struct {
	Runtime      string `param:"runtime"`
	Organization string `param:"org"`
	Repository   string `param:"repo"`
	Release      string `param:"release"`
	Format       string `param:"format"`
}

func (p *Params) Sha1() string {
	var sb strings.Builder
	sb.WriteString(p.Runtime)
	sb.WriteString(p.Organization)
	sb.WriteString(p.Repository)
	sb.WriteString(p.Release)

	triplet := sb.String()

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

	var sb strings.Builder
	sb.WriteString("/")
	sb.WriteString(p.Runtime)
	sb.WriteString("/")
	sb.WriteString(p.Organization)
	sb.WriteString("/")
	sb.WriteString(p.Repository)
	sb.WriteString("/")
	sb.WriteString(p.Release)
	sb.WriteString("/")
	sb.WriteString(p.Format)
	sb.WriteString("/")

	var formats = map[string]struct {
		width  int
		height int
	}{
		"480p":  {854, 480},
		"720p":  {1280, 720},
		"1080p": {1920, 1080},
	}

	format, ok := formats[p.Format]
	if !ok {
		return fmt.Errorf("invalid format: %s", p.Format)
	}

	data := struct {
		BaseURL string
		Width   int
		Height  int
	}{
		BaseURL: sb.String(),
		Width:   format.width,
		Height:  format.height,
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

	etag := p.Sha1()

	if c.Request().Header.Get("If-None-Match") == etag {
		return c.NoContent(http.StatusNotModified)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000")
	c.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
	c.Response().Header().Set("ETag", etag)

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

	etag := p.Sha1()

	if c.Request().Header.Get("If-None-Match") == etag {
		return c.NoContent(http.StatusNotModified)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000")
	c.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
	c.Response().Header().Set("ETag", etag)

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

	etag := p.Sha1()

	if c.Request().Header.Get("If-None-Match") == etag {
		return c.NoContent(http.StatusNotModified)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000")
	c.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
	c.Response().Header().Set("ETag", etag)

	return c.Blob(http.StatusOK, "application/octet-stream", bundle)
}

func assetsHandler(static fs.FS) echo.HandlerFunc {
	return func(c echo.Context) error {
		path := c.Param("*")
		f, err := static.Open(fmt.Sprintf("assets/%s", path))
		if err != nil {
			if os.IsNotExist(err) {
				return echo.NotFoundHandler(c)
			}
			return fmt.Errorf("error opening file: %w", err)
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		h := sha1.New()
		if _, err := h.Write(content); err != nil {
			return fmt.Errorf("error computing SHA1: %w", err)
		}

		etag := fmt.Sprintf("%x", h.Sum(nil))

		if c.Request().Header.Get("If-None-Match") == etag {
			return c.NoContent(http.StatusNotModified)
		}

		c.Response().Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000")
		c.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
		c.Response().Header().Set("ETag", etag)

		c.Response().Header().Set(echo.HeaderContentType, http.DetectContentType(content))
		c.Response().WriteHeader(http.StatusOK)
		if _, err = c.Response().Write(content); err != nil {
			return fmt.Errorf("error writing response: %w", err)
		}

		return nil
	}
}

func main() {
	e := echo.New()
	e.Pre(middleware.RemoveTrailingSlash())
	e.Pre(middleware.GzipWithConfig(middleware.GzipConfig{MinLength: 2048}))

	e.GET("/:runtime/:org/:repo/:release/:format", indexHandler)
	e.GET("/:runtime/:org/:repo/:release/:format/carimbo.js", javaScriptHandler)
	e.GET("/:runtime/:org/:repo/:release/:format/carimbo.wasm", webAssemblyHandler)
	e.GET("/:runtime/:org/:repo/:release/:format/bundle.7z", bundleHandler)
	e.GET("/:runtime/:org/:repo/:release/:format/assets/*", assetsHandler(assets))
	e.GET("/favicon.ico", favIconHandler)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", os.Getenv("PORT"))))
}
