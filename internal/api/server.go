// Package api provides the HTTP API server for the Flutter GUI.
//
// It exposes a REST API on 127.0.0.1:9876 that mirrors the CLI commands.
// Only listens on loopback — no external network access.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/harness"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/pipeline"
)

// Server wraps the HTTP API server.
type Server struct {
	h      *harness.Harness
	port   int
	server *http.Server
}

// NewServer creates an HTTP API server on the given port.
func NewServer(h *harness.Harness, port int) *Server {
	s := &Server{h: h, port: port}
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	// Skills
	mux.HandleFunc("/api/skills", s.handleSkills)

	// Pipeline — run full pipeline
	mux.HandleFunc("/api/pipeline", s.handlePipeline)

	// Stage — run single stage
	mux.HandleFunc("/api/stage", s.handleStage)

	// Global rules query
	mux.HandleFunc("/api/global_rules", s.handleGlobalRules)
	mux.HandleFunc("/api/global_rules/network", s.handleNetworkPermission)

	// Tasks — list outputs
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskDetail)

	// Cors for Flutter Web dev
	handler := corsMiddleware(mux)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // AI generation can be slow
		IdleTimeout:  60 * time.Second,
	}
	return s
}

// Run starts the server and blocks until SIGINT/SIGTERM.
func (s *Server) Run() error {
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}()

	log.Printf("AI Novel Agent API listening on http://%s", s.server.Addr)
	log.Printf("Flutter GUI can connect to this endpoint")
	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("api: serve: %w", err)
	}
	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonResp(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	jsonResp(w, status, map[string]string{"error": msg})
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, http.StatusOK, map[string]string{
		"status": "ok",
		"skills": fmt.Sprintf("%d loaded", len(s.h.ListSkills())),
	})
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	names := s.h.ListSkills()
	skills := make([]map[string]any, 0, len(names))
	for _, name := range names {
		sk := s.h.GetSkill(name)
		if sk != nil {
			skills = append(skills, map[string]any{
				"name":        sk.Name,
				"version":     sk.Version,
				"description": sk.Description,
				"stages":      sk.Stages,
			})
		}
	}
	jsonResp(w, http.StatusOK, map[string]any{"skills": skills})
}

type pipelineReq struct {
	SkillName string `json:"skill_name"`
	TrendData string `json:"trend_data"`
	TaskID    string `json:"task_id,omitempty"`
}

func (s *Server) handlePipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req pipelineReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.SkillName == "" || req.TrendData == "" {
		jsonErr(w, http.StatusBadRequest, "skill_name and trend_data are required")
		return
	}
	if req.TaskID == "" {
		req.TaskID = fmt.Sprintf("task-%d", time.Now().Unix())
	}

	ctx := r.Context()
	outputs, err := s.h.RunPipeline(ctx, req.TaskID, req.SkillName, req.TrendData)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, map[string]any{
		"task_id": req.TaskID,
		"outputs": outputs,
	})
}

type stageReq struct {
	SkillName string `json:"skill_name"`
	Stage     string `json:"stage"`
	TaskID    string `json:"task_id,omitempty"`
	Input     map[string]string `json:"input"`
}

func (s *Server) handleStage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req stageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.TaskID == "" {
		req.TaskID = fmt.Sprintf("task-%d", time.Now().Unix())
	}

	out, err := s.h.RunStage(r.Context(), req.TaskID, req.SkillName, req.Stage,
		pipeline.StageInput{
			TrendData:      req.Input["trend_data"],
			Topic:          req.Input["topic"],
			ChapterOutline: req.Input["chapter_outline"],
			PrevContext:    req.Input["prev_context"],
			Content:        req.Input["content"],
		})
	if err != nil {
		// Check for network permission request
		var permErr *pipeline.NetworkPermissionRequired
		if errors.As(err, &permErr) {
			jsonResp(w, http.StatusForbidden, map[string]any{
				"needs_network_permission": true,
				"skill_name":               permErr.Permission.SkillName,
				"reason":                   permErr.Permission.Reason,
				"action":                   "Call POST /api/global_rules/network with {'enable': true} to grant access",
			})
			return
		}
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, out)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	taskID := r.URL.Query().Get("task_id")
	format := r.URL.Query().Get("format")
	if taskID == "" {
		jsonErr(w, http.StatusBadRequest, "task_id is required")
		return
	}
	if format == "" {
		format = "txt"
	}
	// Read all outputs for the task
	entries, err := os.ReadDir(s.h.Root + "/outputs/" + taskID)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "task not found: "+taskID)
		return
	}
	type file struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	var files []file
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".txt" {
			continue
		}
		data, err := os.ReadFile(s.h.Root + "/outputs/" + taskID + "/" + e.Name())
		if err != nil {
			continue
		}
		files = append(files, file{Name: e.Name(), Content: string(data)})
	}
	jsonResp(w, http.StatusOK, map[string]any{
		"task_id": taskID,
		"format":  format,
		"files":   files,
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.h.Root + "/outputs")
	if err != nil {
		jsonResp(w, http.StatusOK, map[string]any{"tasks": []string{}})
		return
	}
	var tasks []string
	for _, e := range entries {
		if e.IsDir() {
			tasks = append(tasks, e.Name())
		}
	}
	jsonResp(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	// /api/tasks/<task_id>
	taskID := r.URL.Path[len("/api/tasks/"):]
	if taskID == "" {
		jsonErr(w, http.StatusBadRequest, "task_id required")
		return
	}
	entries, err := os.ReadDir(s.h.Root + "/outputs/" + taskID)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "task not found")
		return
	}
	type stageFile struct {
		Stage   string `json:"stage"`
		Content string `json:"content"`
	}
	var stages []stageFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".txt" {
			continue
		}
		data, _ := os.ReadFile(s.h.Root + "/outputs/" + taskID + "/" + e.Name())
		stages = append(stages, stageFile{
			Stage:   strings.TrimSuffix(e.Name(), ".txt"),
			Content: string(data),
		})
	}
	jsonResp(w, http.StatusOK, map[string]any{
		"task_id": taskID,
		"stages":  stages,
	})
}

// ---- Global Rules ----

func (s *Server) handleGlobalRules(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, http.StatusOK, map[string]any{
		"language": s.h.GlobalRules.Language,
		"rules":    s.h.GlobalRules.Rules,
		"network": map[string]any{
			"enabled":         s.h.GlobalRules.Network.Enabled,
			"ask_permission":  s.h.GlobalRules.Network.AskPermission,
		},
	})
}

func (s *Server) handleNetworkPermission(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Toggle network access
		var req struct {
			Enable bool `json:"enable"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			s.h.GlobalRules.Network.Enabled = req.Enable
			s.h.Pipe.GlobalRules.Network.Enabled = req.Enable
			jsonResp(w, http.StatusOK, map[string]any{
				"network_enabled": s.h.GlobalRules.Network.Enabled,
			})
			return
		}
	}
	// GET: read current network status
	jsonResp(w, http.StatusOK, map[string]any{
		"network_enabled":  s.h.GlobalRules.Network.Enabled,
		"ask_permission":   s.h.GlobalRules.Network.AskPermission,
		"allowed_domains":  s.h.GlobalRules.Network.AllowedDomains,
	})
}
