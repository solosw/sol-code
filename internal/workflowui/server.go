package workflowui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/solosw/solcode/internal/workflow"
)

// Config configures the local workflow node editor server.
type Config struct {
	WorkDir     string
	UserDir     string
	ProjectDir  string
	List        func() []workflow.Definition
	Save        func(def workflow.Definition, scope workflow.SaveScope, layout *workflow.Layout) (string, error)
	Delete      func(name string) error
	Reload      func()
	Tools       func() []string
	Addr        string // empty → 127.0.0.1:0
	OpenBrowser bool
}

// Server is a local HTTP server hosting the node editor UI.
type Server struct {
	cfg    Config
	server *http.Server
	url    string
}

type workflowDTO struct {
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	ExecutionMode string              `json:"execution_mode"`
	Tasks         []workflow.TaskSpec `json:"tasks"`
	Path          string              `json:"path,omitempty"`
	Source        string              `json:"source,omitempty"`
	Layout        workflow.Layout     `json:"layout"`
}

type saveRequest struct {
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	ExecutionMode string              `json:"execution_mode"`
	Tasks         []workflow.TaskSpec `json:"tasks"`
	Scope         string              `json:"scope"`
	Layout        *workflow.Layout    `json:"layout,omitempty"`
}

type metaResponse struct {
	WorkDir    string   `json:"work_dir"`
	UserDir    string   `json:"user_dir"`
	ProjectDir string   `json:"project_dir"`
	Tools      []string `json:"tools"`
}

// Start launches the editor server and optionally opens a browser.
func Start(cfg Config) (*Server, string, error) {
	if cfg.List == nil || cfg.Save == nil {
		return nil, "", fmt.Errorf("workflowui: List and Save callbacks are required")
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", fmt.Errorf("workflowui listen: %w", err)
	}
	s := &Server{cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/meta", s.handleMeta)
	mux.HandleFunc("/api/workflows", s.handleWorkflows)
	mux.HandleFunc("/api/workflows/", s.handleWorkflowByName)
	mux.HandleFunc("/api/save", s.handleSave)
	staticRoot, err := fs.Sub(staticFS, "static")
	if err != nil {
		_ = ln.Close()
		return nil, "", err
	}
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	url := "http://" + ln.Addr().String() + "/"
	s.url = url

	go func() {
		_ = s.server.Serve(ln)
	}()

	if cfg.OpenBrowser {
		_ = openBrowser(url)
	}
	return s, url, nil
}

// URL returns the base URL of the running server.
func (s *Server) URL() string {
	if s == nil {
		return ""
	}
	return s.url
}

// Close shuts down the server.
func (s *Server) Close() error {
	if s == nil || s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, metaResponse{
		WorkDir:    s.cfg.WorkDir,
		UserDir:    s.cfg.UserDir,
		ProjectDir: s.cfg.ProjectDir,
		Tools:      s.availableTools(),
	})
}

func (s *Server) availableTools() []string {
	if s.cfg.Tools != nil {
		if tools := uniqueSorted(s.cfg.Tools()); len(tools) > 0 {
			return tools
		}
	}
	return defaultEditorTools()
}

func defaultEditorTools() []string {
	return []string{
		"AskUser", "Bash", "Diff", "Edit", "Fetch", "Glob", "Grep",
		"LS", "LSP", "Patch", "Skill", "Task", "TodoWrite",
		"View", "ViewImage", "WebSearch", "Write",
	}
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defs := s.cfg.List()
	out := make([]workflowDTO, 0, len(defs))
	for _, def := range defs {
		out = append(out, toDTO(def))
	}
	writeJSON(w, out)
}

func (s *Server) handleWorkflowByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	name = strings.Trim(name, "/")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		for _, def := range s.cfg.List() {
			if def.Name == name {
				writeJSON(w, toDTO(def))
				return
			}
		}
		http.Error(w, "workflow not found", http.StatusNotFound)
	case http.MethodDelete:
		if s.cfg.Delete == nil {
			http.Error(w, "delete not configured", http.StatusNotImplemented)
			return
		}
		if err := s.cfg.Delete(name); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if s.cfg.Reload != nil {
			s.cfg.Reload()
		}
		writeJSON(w, map[string]any{"ok": true, "name": name})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	scope := workflow.SaveScope(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = workflow.SaveScopeProject
	}
	def := workflow.Definition{
		Name:          req.Name,
		Description:   req.Description,
		ExecutionMode: req.ExecutionMode,
		Tasks:         req.Tasks,
	}
	path, err := s.cfg.Save(def, scope, req.Layout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.cfg.Reload != nil {
		s.cfg.Reload()
	}
	writeJSON(w, map[string]any{
		"ok":   true,
		"path": path,
		"name": def.Name,
	})
}

func toDTO(def workflow.Definition) workflowDTO {
	layout := workflow.Layout{Nodes: map[string]workflow.NodePos{}}
	if def.Path != "" {
		if loaded, err := workflow.LoadLayout(def.Path); err == nil {
			layout = loaded
		}
	}
	if len(layout.Nodes) == 0 {
		layout = workflow.DefaultLayout(def)
	}
	return workflowDTO{
		Name:          def.Name,
		Description:   def.Description,
		ExecutionMode: def.ExecutionMode,
		Tasks:         def.Tasks,
		Path:          def.Path,
		Source:        def.Source,
		Layout:        layout,
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// OpenBrowser opens url with the platform default browser.
func OpenBrowser(url string) error {
	return openBrowser(url)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
