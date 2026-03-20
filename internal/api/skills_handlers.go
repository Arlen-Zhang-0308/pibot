package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/pibot/pibot/internal/skills"
)

// handleListInstalledSkills returns all skills installed in the skills directory.
func (s *Server) handleListInstalledSkills(w http.ResponseWriter, r *http.Request) {
	skillsPath := s.config.GetSkillsPath()
	infos, err := skills.ListInstalledSkills(skillsPath)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to list skills: "+err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"skills": infos})
}

// handleSearchClawHub proxies a search query to the ClawHub API.
func (s *Server) handleSearchClawHub(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		errorResponse(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	results, err := skills.SearchClawHub(query)
	if err != nil {
		log.Printf("[api/skills] ClawHub search error: %v", err)
		errorResponse(w, http.StatusBadGateway, "ClawHub search failed: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"results": results})
}

// handleGetClawHubSkill proxies a skill detail request to the ClawHub API.
func (s *Server) handleGetClawHubSkill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	slug := vars["slug"]
	if slug == "" {
		errorResponse(w, http.StatusBadRequest, "slug is required")
		return
	}

	detail, err := skills.GetClawHubSkill(slug)
	if err != nil {
		log.Printf("[api/skills] ClawHub detail error for %q: %v", slug, err)
		errorResponse(w, http.StatusBadGateway, "ClawHub detail failed: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, detail)
}

// handleInstallSkill downloads and installs a skill from ClawHub, then
// hot-registers it into the running capabilities registry.
func (s *Server) handleInstallSkill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	slug := vars["slug"]
	if slug == "" {
		errorResponse(w, http.StatusBadRequest, "slug is required")
		return
	}

	skillsPath := s.config.GetSkillsPath()
	log.Printf("[api/skills] installing ClawHub skill %q into %s", slug, skillsPath)

	skillDir, err := skills.DownloadAndInstall(slug, skillsPath)
	if err != nil {
		log.Printf("[api/skills] install error for %q: %v", slug, err)
		errorResponse(w, http.StatusInternalServerError, "Install failed: "+err.Error())
		return
	}

	// Hot-load the skill into the running registry.
	if err := skills.LoadSingleSkill(s.capabilities, skillDir); err != nil {
		log.Printf("[api/skills] hot-load error for %q: %v", slug, err)
		// Installed on disk but not in registry — still report partial success.
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"status":  "installed",
			"slug":    slug,
			"dir":     skillDir,
			"warning": "installed on disk but could not register: " + err.Error(),
		})
		return
	}

	log.Printf("[api/skills] skill %q installed and registered", slug)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "installed",
		"slug":   slug,
		"dir":    skillDir,
	})
}

// handleUninstallSkill removes an installed skill from disk and unregisters it.
// The URL parameter is the skill directory name (slug), not the display name.
func (s *Server) handleUninstallSkill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dirName := vars["name"]
	if dirName == "" {
		errorResponse(w, http.StatusBadRequest, "skill name is required")
		return
	}

	skillsPath := s.config.GetSkillsPath()
	skillDir := filepath.Join(expandSkillsPath(skillsPath), dirName)

	// Read manifest to get the registered capability name before deleting.
	infos, err := skills.ListInstalledSkills(skillsPath)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to read skills: "+err.Error())
		return
	}

	var registeredName string
	for _, info := range infos {
		if info.Dir == dirName {
			registeredName = info.Name
			break
		}
	}

	// Remove from disk.
	if err := os.RemoveAll(skillDir); err != nil {
		log.Printf("[api/skills] remove error for %q: %v", dirName, err)
		errorResponse(w, http.StatusInternalServerError, "Failed to remove skill directory: "+err.Error())
		return
	}

	// Unregister from the in-memory registry.
	if registeredName != "" {
		skills.UnloadExternalSkill(s.capabilities, registeredName)
	}

	log.Printf("[api/skills] skill %q uninstalled", dirName)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "uninstalled",
		"name":   dirName,
	})
}

// expandSkillsPath is a thin wrapper so this file doesn't import the unexported
// expandHome from skills package — we duplicate the simple logic here.
func expandSkillsPath(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return home + path[1:]
		}
	}
	return path
}
