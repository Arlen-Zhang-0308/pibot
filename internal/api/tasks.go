package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pibot/pibot/internal/scheduler"
)

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.scheduler.List()
	jsonResponse(w, http.StatusOK, tasks)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var task scheduler.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := s.scheduler.Add(&task); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, &task)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	task, err := s.scheduler.Get(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var updated scheduler.Task
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := s.scheduler.Update(id, &updated); err != nil {
		if err.Error() == "task not found: "+id {
			errorResponse(w, http.StatusNotFound, err.Error())
		} else {
			errorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	task, _ := s.scheduler.Get(id)
	jsonResponse(w, http.StatusOK, task)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.scheduler.Remove(id); err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.scheduler.RunNow(id); err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "triggered"})
}

func (s *Server) handleTaskHistory(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := s.scheduler.Get(id); err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	history := s.scheduler.GetHistory(id)
	jsonResponse(w, http.StatusOK, history)
}
