package tool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageGenerateSavesPNG(t *testing.T) {
	png := minimalPNG()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("auth = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["prompt"] != "a cat" {
			t.Fatalf("prompt = %#v", body["prompt"])
		}
		if body["response_format"] != "b64_json" {
			t.Fatalf("response_format = %#v", body["response_format"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(png)}},
		})
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	tool := NewImageGenerateTool(ImageAPIConfig{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		Model:     "dall-e-3",
		OutputDir: "out-images",
	})
	input, _ := json.Marshal(map[string]any{"prompt": "a cat"})
	res, err := tool.Invoke(context.Background(), &UseContext{WorkDir: dir}, input)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("error: %s", res.Text)
	}
	if !strings.Contains(res.Text, "Generated image saved") {
		t.Fatalf("text: %s", res.Text)
	}
	// Find written file under out-images
	entries, err := os.ReadDir(filepath.Join(dir, "out-images"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("files = %d", len(entries))
	}
	got, err := os.ReadFile(filepath.Join(dir, "out-images", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(png) {
		t.Fatalf("png mismatch len got=%d want=%d", len(got), len(png))
	}
}

func TestImageEditMultipart(t *testing.T) {
	png := minimalPNG()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "multipart/form-data") {
			t.Fatalf("content-type = %s", ct)
		}
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("prompt") != "make blue" {
			t.Fatalf("prompt = %q", r.FormValue("prompt"))
		}
		f, _, err := r.FormFile("image")
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(png)}},
		})
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	if err := os.WriteFile(src, png, 0o644); err != nil {
		t.Fatal(err)
	}
	tool := NewImageEditTool(ImageAPIConfig{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		EditModel: "dall-e-2",
		OutputDir: "edits",
	})
	input, _ := json.Marshal(map[string]any{
		"image":  "src.png",
		"prompt": "make blue",
	})
	res, err := tool.Invoke(context.Background(), &UseContext{WorkDir: dir}, input)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("error: %s", res.Text)
	}
	if !strings.Contains(res.Text, "Edited image saved") {
		t.Fatalf("text: %s", res.Text)
	}
}

func TestImageAPIEndpointJoinsV1(t *testing.T) {
	c := newImageAPIClient(ImageAPIConfig{BaseURL: "https://api.example.com", APIKey: "k"})
	if got := c.endpoint("/images/generations"); got != "https://api.example.com/v1/images/generations" {
		t.Fatalf("got %s", got)
	}
	c2 := newImageAPIClient(ImageAPIConfig{BaseURL: "https://api.example.com/v1", APIKey: "k"})
	if got := c2.endpoint("/images/edits"); got != "https://api.example.com/v1/images/edits" {
		t.Fatalf("got %s", got)
	}
}

// minimalPNG is a valid 1x1 PNG.
func minimalPNG() []byte {
	// precomputed 1x1 transparent PNG
	b, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
	)
	if err != nil {
		panic(err)
	}
	return b
}
