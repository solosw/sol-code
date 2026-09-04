package acp

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolOutputContentImagePath(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "out.png")
	// 1x1 png
	raw, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
	)
	if err := os.WriteFile(pngPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	caption := "Generated image saved.\npath: " + pngPath + "\nprompt: a cat\n"
	content, locations := toolOutputContent("ImageGenerate", caption, false)
	if len(content) < 2 {
		t.Fatalf("content len=%d want >=2 text+resource_link", len(content))
	}
	if content[0].Content == nil || content[0].Content.Type != "text" {
		t.Fatalf("first = %+v", content[0])
	}
	if content[0].Content.Text != caption {
		t.Fatalf("text mismatch")
	}
	rl := content[1].Content
	if rl == nil || rl.Type != "resource_link" {
		t.Fatalf("second = %+v", content[1])
	}
	if !strings.HasPrefix(rl.URI, "file://") {
		t.Fatalf("uri = %q", rl.URI)
	}
	if rl.MimeType != "image/png" {
		t.Fatalf("mime = %q", rl.MimeType)
	}
	if rl.Text != pngPath {
		t.Fatalf("link text path = %q", rl.Text)
	}
	// small file → inline image
	if len(content) < 3 || content[2].Content == nil || content[2].Content.Type != "image" {
		t.Fatalf("expected inline image, got %+v", content)
	}
	if content[2].Content.Data == "" {
		t.Fatal("empty image data")
	}
	if len(locations) != 1 || locations[0].Path != pngPath {
		t.Fatalf("locations = %+v", locations)
	}
}

func TestToolOutputContentNoPath(t *testing.T) {
	content, locations := toolOutputContent("Bash", "hello\n", false)
	if len(content) != 1 || content[0].Content.Type != "text" {
		t.Fatalf("%+v", content)
	}
	if len(locations) != 0 {
		t.Fatalf("locations = %+v", locations)
	}
}

func TestToolOutputContentErrorSkipsMedia(t *testing.T) {
	content, locations := toolOutputContent("ImageGenerate", "path: /tmp/x.png\n", true)
	if len(content) != 1 || len(locations) != 0 {
		t.Fatalf("content=%+v locations=%+v", content, locations)
	}
}

func TestPathToFileURI(t *testing.T) {
	uri := pathToFileURI(`C:\tmp\a.png`)
	if !strings.HasPrefix(uri, "file:///") {
		t.Fatalf("uri = %q", uri)
	}
	if !strings.Contains(uri, "C:") && !strings.Contains(strings.ToUpper(uri), "C:") {
		// slash form C:/
		if !strings.Contains(uri, "C:/") && !strings.Contains(uri, "c:/") {
			t.Fatalf("uri missing drive: %q", uri)
		}
	}
}

func TestExtractImagePathViewImageCaption(t *testing.T) {
	out := "Image: shot.png\nSize: 10x10\nPath: /work/shot.png\n"
	if got := extractImagePathFromToolOutput(out); got != "/work/shot.png" {
		t.Fatalf("got %q", got)
	}
}
