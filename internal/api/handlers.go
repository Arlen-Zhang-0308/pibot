package api

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/pibot/pibot/internal/ai"
	"github.com/pibot/pibot/internal/config"
)

// Response helpers
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	log.Printf("[api] ERROR %d: %s", status, message)
	jsonResponse(w, status, map[string]string{"error": message})
}

// Config handlers

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, s.config.Public())
}

// ConfigUpdateRequest represents a config update request
type ConfigUpdateRequest struct {
	DefaultProvider string `json:"default_provider,omitempty"`
	OpenAIKey       string `json:"openai_key,omitempty"`
	OpenAIModel     string `json:"openai_model,omitempty"`
	AnthropicKey    string `json:"anthropic_key,omitempty"`
	AnthropicModel  string `json:"anthropic_model,omitempty"`
	GoogleKey       string `json:"google_key,omitempty"`
	GoogleModel     string `json:"google_model,omitempty"`
	OllamaBaseURL   string `json:"ollama_base_url,omitempty"`
	OllamaModel     string `json:"ollama_model,omitempty"`
	QwenKey         string `json:"qwen_key,omitempty"`
	QwenBaseURL     string `json:"qwen_base_url,omitempty"`
	QwenModel       string `json:"qwen_model,omitempty"`
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Printf("[api] config update requested from %s", r.RemoteAddr)

	err := s.config.Update(func(c *config.Config) {
		if req.DefaultProvider != "" {
			c.AI.DefaultProvider = req.DefaultProvider
		}
		if req.OpenAIKey != "" {
			c.AI.OpenAI.APIKey = req.OpenAIKey
		}
		if req.OpenAIModel != "" {
			c.AI.OpenAI.Model = req.OpenAIModel
		}
		if req.AnthropicKey != "" {
			c.AI.Anthropic.APIKey = req.AnthropicKey
		}
		if req.AnthropicModel != "" {
			c.AI.Anthropic.Model = req.AnthropicModel
		}
		if req.GoogleKey != "" {
			c.AI.Google.APIKey = req.GoogleKey
		}
		if req.GoogleModel != "" {
			c.AI.Google.Model = req.GoogleModel
		}
		if req.OllamaBaseURL != "" {
			c.AI.Ollama.BaseURL = req.OllamaBaseURL
		}
		if req.OllamaModel != "" {
			c.AI.Ollama.Model = req.OllamaModel
		}
		if req.QwenKey != "" {
			c.AI.Qwen.APIKey = req.QwenKey
		}
		if req.QwenBaseURL != "" {
			c.AI.Qwen.BaseURL = req.QwenBaseURL
		}
		if req.QwenModel != "" {
			c.AI.Qwen.Model = req.QwenModel
		}
	})

	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to update config")
		return
	}

	log.Printf("[api] config updated successfully")
	jsonResponse(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Chat handlers

// ChatRequest represents a chat request
type ChatRequest struct {
	Messages []ai.Message `json:"messages"`
	Provider string       `json:"provider,omitempty"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
	Response string `json:"response"`
	Provider string `json:"provider"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Messages) == 0 {
		errorResponse(w, http.StatusBadRequest, "No messages provided")
		return
	}

	providerName := req.Provider
	if providerName == "" {
		providerName = s.config.GetAI().DefaultProvider
	}

	log.Printf("[api] chat request from %s provider=%s messages=%d", r.RemoteAddr, providerName, len(req.Messages))

	response, err := s.chatSession.ChatWithTools(r.Context(), providerName, req.Messages)
	if err != nil {
		log.Printf("[api] chat request ERROR provider=%s: %v", providerName, err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("[api] chat request completed provider=%s response_len=%d", providerName, len(response))
	jsonResponse(w, http.StatusOK, ChatResponse{
		Response: response,
		Provider: providerName,
	})
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providers := s.aiManager.ListProviders()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"providers": providers,
		"default":   s.config.GetAI().DefaultProvider,
	})
}

// Execution handlers

// ExecRequest represents a command execution request
type ExecRequest struct {
	Command string `json:"command"`
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Printf("[api] exec request from %s: %s", r.RemoteAddr, req.Command)

	result, err := s.executor.Execute(r.Context(), req.Command)
	if err != nil {
		log.Printf("[api] exec request ERROR: %v", err)
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

func (s *Server) handleExecConfirm(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pendingID := vars["id"]

	log.Printf("[api] exec confirm from %s pending_id=%s", r.RemoteAddr, pendingID)

	result, err := s.executor.ExecuteConfirmed(r.Context(), pendingID)
	if err != nil {
		log.Printf("[api] exec confirm ERROR pending_id=%s: %v", pendingID, err)
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

func (s *Server) handleExecCancel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pendingID := vars["id"]

	log.Printf("[api] exec cancel from %s pending_id=%s", r.RemoteAddr, pendingID)

	if err := s.executor.CancelPending(pendingID); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) handleListPending(w http.ResponseWriter, r *http.Request) {
	pending := s.executor.ListPending()
	jsonResponse(w, http.StatusOK, map[string]interface{}{"pending": pending})
}

// File operation handlers

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	
	files, err := s.fileOps.List(path)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"files":         files,
		"base_directory": s.fileOps.GetBaseDirectory(),
	})
}

func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := vars["path"]
	
	// Combine with base directory
	fullPath := filepath.Join(s.fileOps.GetBaseDirectory(), path)
	
	content, err := s.fileOps.Read(fullPath)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"path":    fullPath,
		"content": content,
	})
}

// WriteFileRequest represents a file write request
type WriteFileRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleWriteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := vars["path"]
	
	var req WriteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Combine with base directory
	fullPath := filepath.Join(s.fileOps.GetBaseDirectory(), path)
	
	if err := s.fileOps.Write(fullPath, req.Content); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status": "written",
		"path":   fullPath,
	})
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := vars["path"]
	
	// Combine with base directory
	fullPath := filepath.Join(s.fileOps.GetBaseDirectory(), path)
	
	recursive := r.URL.Query().Get("recursive") == "true"
	
	var err error
	if recursive {
		err = s.fileOps.DeleteRecursive(fullPath)
	} else {
		err = s.fileOps.Delete(fullPath)
	}
	
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"path":   fullPath,
	})
}
