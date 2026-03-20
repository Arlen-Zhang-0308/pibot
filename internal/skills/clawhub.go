package skills

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	clawHubAPIBase     = "https://clawhub.ai/api"
	clawHubDownloadURL = "https://wry-manatee-359.convex.site/api/v1/download"
	clawHubHTTPTimeout = 30 * time.Second
)

var clawHubClient = &http.Client{Timeout: clawHubHTTPTimeout}

// ClawHubSkillSummary is a search result item from the ClawHub search API.
type ClawHubSkillSummary struct {
	Score       float64 `json:"score"`
	Slug        string  `json:"slug"`
	DisplayName string  `json:"displayName"`
	Summary     string  `json:"summary"`
	Version     *string `json:"version"`
	UpdatedAt   int64   `json:"updatedAt"`
}

// ClawHubStats holds skill statistics.
type ClawHubStats struct {
	Comments       int `json:"comments"`
	Downloads      int `json:"downloads"`
	InstallsAllTime int `json:"installsAllTime"`
	InstallsCurrent int `json:"installsCurrent"`
	Stars          int `json:"stars"`
	Versions       int `json:"versions"`
}

// ClawHubVersion holds version-specific info.
type ClawHubVersion struct {
	Version   string `json:"version"`
	CreatedAt int64  `json:"createdAt"`
	Changelog string `json:"changelog"`
}

// ClawHubOwner holds the skill author info.
type ClawHubOwner struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Image       string `json:"image"`
}

// ClawHubSkillDetail is the full skill metadata from the ClawHub detail API.
type ClawHubSkillDetail struct {
	Skill struct {
		Slug        string            `json:"slug"`
		DisplayName string            `json:"displayName"`
		Summary     string            `json:"summary"`
		Stats       ClawHubStats      `json:"stats"`
		CreatedAt   int64             `json:"createdAt"`
		UpdatedAt   int64             `json:"updatedAt"`
	} `json:"skill"`
	LatestVersion ClawHubVersion `json:"latestVersion"`
	Owner         ClawHubOwner   `json:"owner"`
}

// SearchClawHub queries the ClawHub search API and returns matching skills.
func SearchClawHub(query string) ([]ClawHubSkillSummary, error) {
	endpoint := fmt.Sprintf("%s/search?q=%s", clawHubAPIBase, url.QueryEscape(query))
	resp, err := clawHubClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("clawhub search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clawhub search returned status %d", resp.StatusCode)
	}

	var result struct {
		Results []ClawHubSkillSummary `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("clawhub search decode failed: %w", err)
	}
	return result.Results, nil
}

// GetClawHubSkill fetches full metadata for a skill by slug.
func GetClawHubSkill(slug string) (*ClawHubSkillDetail, error) {
	endpoint := fmt.Sprintf("%s/skill?slug=%s", clawHubAPIBase, url.QueryEscape(slug))
	resp, err := clawHubClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("clawhub detail request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clawhub detail returned status %d", resp.StatusCode)
	}

	var detail ClawHubSkillDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("clawhub detail decode failed: %w", err)
	}
	return &detail, nil
}

// DownloadAndInstall downloads a ClawHub skill by slug, extracts it into
// skillsDir/<slug>/, and writes a pibot-compatible skill.yaml manifest.
// Returns the installed skill directory path.
func DownloadAndInstall(slug, skillsDir string) (string, error) {
	expanded := expandHome(skillsDir)
	if err := os.MkdirAll(expanded, 0755); err != nil {
		return "", fmt.Errorf("cannot create skills directory: %w", err)
	}

	// Fetch skill metadata first for display name and description.
	detail, err := GetClawHubSkill(slug)
	if err != nil {
		return "", fmt.Errorf("cannot fetch skill metadata: %w", err)
	}

	// Download the skill zip.
	downloadEndpoint := fmt.Sprintf("%s?slug=%s", clawHubDownloadURL, url.QueryEscape(slug))
	resp, err := clawHubClient.Get(downloadEndpoint)
	if err != nil {
		return "", fmt.Errorf("clawhub download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("clawhub download returned status %d", resp.StatusCode)
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading zip data failed: %w", err)
	}

	skillDir := filepath.Join(expanded, slug)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create skill directory: %w", err)
	}

	// Extract all files from the zip.
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", fmt.Errorf("parsing zip failed: %w", err)
	}

	var skillMDContent string
	for _, f := range zipReader.File {
		name := filepath.Base(f.Name)
		// Only extract recognised skill files; skip directories.
		if f.FileInfo().IsDir() {
			continue
		}

		destPath := filepath.Join(skillDir, name)
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening zip entry %q: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("reading zip entry %q: %w", f.Name, err)
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return "", fmt.Errorf("writing %q: %w", destPath, err)
		}

		// Capture SKILL.md / instructions.md content for the pibot skill.md.
		upperName := strings.ToUpper(name)
		if upperName == "SKILL.MD" || upperName == "INSTRUCTIONS.MD" {
			skillMDContent = string(data)
		}
	}

	// Write pibot-compatible skill.yaml manifest (instruction-only skill).
	description := detail.Skill.Summary
	if description == "" {
		description = fmt.Sprintf("ClawHub skill: %s", detail.Skill.DisplayName)
	}

	manifest := fmt.Sprintf("name: %q\ndescription: %q\ninstruction_only: true\nclawhub_slug: %q\nclawhub_version: %q\n",
		detail.Skill.DisplayName,
		description,
		slug,
		detail.LatestVersion.Version,
	)
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte(manifest), 0644); err != nil {
		return "", fmt.Errorf("writing skill.yaml: %w", err)
	}

	// Write skill.md with YAML frontmatter + ClawHub instructions if we got them.
	if skillMDContent != "" {
		pibotSkillMD := fmt.Sprintf("---\nname: %q\ndescription: %q\ninstruction_only: true\n---\n\n%s",
			detail.Skill.DisplayName,
			description,
			skillMDContent,
		)
		if err := os.WriteFile(filepath.Join(skillDir, "skill.md"), []byte(pibotSkillMD), 0644); err != nil {
			return "", fmt.Errorf("writing skill.md: %w", err)
		}
	}

	return skillDir, nil
}
