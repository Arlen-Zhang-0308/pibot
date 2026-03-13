package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pibot/pibot/internal/ai"
	"github.com/pibot/pibot/internal/config"
	"github.com/pibot/pibot/internal/executor"
	"github.com/pibot/pibot/internal/fileops"
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
	wsHub       *Hub
	router      *mux.Router
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, aiMgr *ai.Manager, exec *executor.Executor, fops *fileops.FileOps) *Server {
	// Create skills registry
	skillsRegistry := skills.NewRegistry()

	// Register skills
	skillsRegistry.Register(skills.NewExecuteCommandSkill(exec))
	skillsRegistry.Register(skills.NewReadFileSkill(fops))
	skillsRegistry.Register(skills.NewWriteFileSkill(fops))
	skillsRegistry.Register(skills.NewListDirectorySkill(fops))
	skillsRegistry.Register(skills.NewSystemInfoSkill(fops))

	// Create chat session with tools
	chatSession := ai.NewChatSession(cfg, skillsRegistry, aiMgr)

	s := &Server{
		config:      cfg,
		aiManager:   aiMgr,
		chatSession: chatSession,
		executor:    exec,
		fileOps:     fops,
		skills:      skillsRegistry,
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
