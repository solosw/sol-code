package workflowui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solosw/solcode/internal/workflow"
)

func TestServerListAndSave(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "project-workflows")
	userDir := filepath.Join(dir, "user-workflows")

	var saved []workflow.Definition
	srv, url, err := Start(Config{
		WorkDir:    dir,
		UserDir:    userDir,
		ProjectDir: projectDir,
		List: func() []workflow.Definition {
			return saved
		},
		Save: func(def workflow.Definition, scope workflow.SaveScope, layout *workflow.Layout) (string, error) {
			path, err := workflow.SaveToDirWithLayout(def, projectDir, layout)
			if err != nil {
				return "", err
			}
			def.Path = path
			// replace or append
			found := false
			for i := range saved {
				if saved[i].Name == def.Name {
					saved[i] = def
					found = true
					break
				}
			}
			if !found {
				saved = append(saved, def)
			}
			return path, nil
		},
		Delete: func(name string) error {
			for i, def := range saved {
				if def.Name == name {
					if err := workflow.Delete(def, []string{projectDir, userDir}); err != nil {
						return err
					}
					saved = append(saved[:i], saved[i+1:]...)
					return nil
				}
			}
			return fmt.Errorf("workflow not found")
		},
		OpenBrowser: false,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	res, err := http.Get(url + "api/meta")
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("meta status %d", res.StatusCode)
	}
	var meta map[string]any
	if err := json.NewDecoder(res.Body).Decode(&meta); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	tools, _ := meta["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("expected tools in meta, got %#v", meta)
	}

	body := `{
	  "name": "node-demo",
	  "description": "from web",
	  "execution_mode": "auto",
	  "scope": "project",
	  "tasks": [
	    {"id":"a","description":"A","prompt":"do a","difficulty":"easy"},
	    {"id":"b","description":"B","prompt":"do b","depends_on":["a"]}
	  ],
	  "layout": {"nodes":{"a":{"x":10,"y":20},"b":{"x":200,"y":20}}}
	}`
	resp, err := http.Post(url+"api/save", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("save status %d: %s", resp.StatusCode, raw)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode save: %v", err)
	}
	if out["path"] == nil || out["path"] == "" {
		t.Fatalf("missing path: %#v", out)
	}

	listResp, err := http.Get(url + "api/workflows")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer listResp.Body.Close()
	var list []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0]["name"] != "node-demo" {
		t.Fatalf("list = %#v", list)
	}

	// static index
	indexResp, err := http.Get(url)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	defer indexResp.Body.Close()
	raw, _ := io.ReadAll(indexResp.Body)
	if indexResp.StatusCode != 200 || len(raw) < 100 {
		t.Fatalf("index status=%d len=%d", indexResp.StatusCode, len(raw))
	}

	req, err := http.NewRequest(http.MethodDelete, url+"api/workflows/node-demo", nil)
	if err != nil {
		t.Fatal(err)
	}
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != 200 {
		body, _ := io.ReadAll(delResp.Body)
		t.Fatalf("delete status %d: %s", delResp.StatusCode, body)
	}
	listResp2, err := http.Get(url + "api/workflows")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp2.Body.Close()
	var list2 []map[string]any
	if err := json.NewDecoder(listResp2.Body).Decode(&list2); err != nil {
		t.Fatal(err)
	}
	if len(list2) != 0 {
		t.Fatalf("expected empty list after delete, got %#v", list2)
	}
}
