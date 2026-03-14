package api

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pibot/pibot/internal/ai"
	"github.com/pibot/pibot/internal/config"
	"github.com/pibot/pibot/internal/executor"
	"github.com/pibot/pibot/internal/fileops"
	"github.com/pibot/pibot/internal/scheduler"
	"github.com/pibot/pibot/internal/skills"
)

// Server holds all dependencies for the API server
type Server struct {
	config      *config.Config
	aiManager   *ai.Manager
	chatSession *ai.ChatSession
	executor    *executor.Executor
	fileOps     *fileops.FileOps
	skills      *skills.Registry
	scheduler   *scheduler.Scheduler
	wsHub       *Hub
	router      *mux.Router
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, aiMgr *ai.Manager, exec *executor.Executor, fops *fileops.FileOps, sched *scheduler.Scheduler) *Server {
	// Create skills registry
	skillsRegistry := skills.NewRegistry()

	// Register built-in skills
	skillsRegistry.Register(skills.NewExecuteCommandSkill(exec))
	skillsRegistry.Register(skills.NewReadFileSkill(fops))
	skillsRegistry.Register(skills.NewWriteFileSkill(fops))
	skillsRegistry.Register(skills.NewListDirectorySkill(fops))
	skillsRegistry.Register(skills.NewSystemInfoSkill(fops))

	// Load external skills from ~/.pibot_skills
	skillsPath := cfg.GetSkillsPath()
	if err := skills.LoadExternalSkills(skillsRegistry, skillsPath); err != nil {
		log.Printf("Warning: failed to load external skills from %s: %v", skillsPath, err)
	}

	// Create chat session with tools
	chatSession := ai.NewChatSession(cfg, skillsRegistry, aiMgr)

	// Wire chat session into scheduler for AI action support
	if sched != nil {
		sched.SetChatSession(chatSession)
	}

	s := &Server{
		config:      cfg,
		aiManager:   aiMgr,
		chatSession: chatSession,
		executor:    exec,
		fileOps:     fops,
		skills:      skillsRegistry,
		scheduler:   sched,
		wsHub:       NewHub(),
		router:      mux.NewRouter(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// API routes
	api := s.router.PathPrefix("/api").Subrouter()
	
	// Config endpoints
	api.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	api.HandleFunc("/config", s.handleUpdateConfig).Methods("POST")
	
	// Chat endpoints
	api.HandleFunc("/chat", s.handleChat).Methods("POST")
	api.HandleFunc("/providers", s.handleListProviders).Methods("GET")
	
	// WebSocket endpoint
	api.HandleFunc("/ws", s.handleWebSocket)
	
	// Command execution endpoints
	api.HandleFunc("/exec", s.handleExec).Methods("POST")
	api.HandleFunc("/exec/confirm/{id}", s.handleExecConfirm).Methods("POST")
	api.HandleFunc("/exec/cancel/{id}", s.handleExecCancel).Methods("POST")
	api.HandleFunc("/exec/pending", s.handleListPending).Methods("GET")
	
	// File operations endpoints
	api.HandleFunc("/files", s.handleListFiles).Methods("GET")
	api.HandleFunc("/files/{path:.*}", s.handleReadFile).Methods("GET")
	api.HandleFunc("/files/{path:.*}", s.handleWriteFile).Methods("POST")
	api.HandleFunc("/files/{path:.*}", s.handleDeleteFile).Methods("DELETE")

	// Scheduled task endpoints
	api.HandleFunc("/tasks", s.handleListTasks).Methods("GET")
	api.HandleFunc("/tasks", s.handleCreateTask).Methods("POST")
	api.HandleFunc("/tasks/{id}", s.handleGetTask).Methods("GET")
	api.HandleFunc("/tasks/{id}", s.handleUpdateTask).Methods("PUT")
	api.HandleFunc("/tasks/{id}", s.handleDeleteTask).Methods("DELETE")
	api.HandleFunc("/tasks/{id}/run", s.handleRunTask).Methods("POST")
	api.HandleFunc("/tasks/{id}/history", s.handleTaskHistory).Methods("GET")
	
	// Static files - serve from embedded filesystem
	s.router.PathPrefix("/").Handler(http.FileServer(http.FS(staticFiles)))
}

// Handler returns the HTTP handler
func (s *Server) Handler() http.Handler {
	return s.router
}

// StartWebSocketHub starts the WebSocket hub
func (s *Server) StartWebSocketHub() {
	go s.wsHub.Run()
}
