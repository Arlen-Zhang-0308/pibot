package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ExternalSkillManifest describes an external skill loaded from ~/.pibot_skills
type ExternalSkillManifest struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Parameters  map[string]interface{} `yaml:"parameters"`
	Executable  string                 `yaml:"executable"`
}

// ExternalSkill wraps a script/executable as a Skill
type ExternalSkill struct {
	manifest   ExternalSkillManifest
	execPath   string
	skillDir   string
}

func (s *ExternalSkill) Name() string        { return s.manifest.Name }
func (s *ExternalSkill) Description() string { return s.manifest.Description }
func (s *ExternalSkill) Parameters() map[string]interface{} {
	if s.manifest.Parameters != nil {
		return s.manifest.Parameters
	}
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// Execute runs the external skill executable, passing params as JSON on stdin.
// The executable should write its result to stdout.
func (s *ExternalSkill) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	log.Printf("[skills/external] running %q executable=%s params=%s", s.manifest.Name, s.execPath, params)
	start := time.Now()

	cmd := exec.CommandContext(ctx, s.execPath)
	cmd.Dir = s.skillDir
	cmd.Stdin = bytes.NewReader(params)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		errMsg := strings.TrimRight(stderr.String(), "\n")
		if errMsg == "" {
			errMsg = err.Error()
		}
		log.Printf("[skills/external] %q FAILED in %s\n  stderr: %s", s.manifest.Name, elapsed, errMsg)
		return "", fmt.Errorf("skill %q failed: %s", s.manifest.Name, errMsg)
	}

	result := strings.TrimRight(stdout.String(), "\n")
	if result == "" {
		result = "(skill completed with no output)"
	}
	log.Printf("[skills/external] %q completed in %s\n  stdout: %s", s.manifest.Name, elapsed, result)
	return result, nil
}

// LoadExternalSkills scans the given directory for skill subdirectories and
// registers any valid skills it finds into the registry.
// Each skill subdirectory must contain a skill.yaml manifest file.
func LoadExternalSkills(registry *Registry, skillsDir string) error {
	expanded := expandHome(skillsDir)

	info, err := os.Stat(expanded)
	if os.IsNotExist(err) {
		// Directory doesn't exist yet — nothing to load
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot access skills directory %q: %w", expanded, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("skills path %q is not a directory", expanded)
	}

	entries, err := os.ReadDir(expanded)
	if err != nil {
		return fmt.Errorf("cannot read skills directory %q: %w", expanded, err)
	}

	loaded := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(expanded, entry.Name())
		skill, err := loadExternalSkill(skillDir)
		if err != nil {
			log.Printf("Skipping external skill %q: %v", entry.Name(), err)
			continue
		}
		registry.Register(skill)
		log.Printf("Loaded external skill %q from %s", skill.Name(), skillDir)
		loaded++
	}

	if loaded > 0 {
		log.Printf("Loaded %d external skill(s) from %s", loaded, expanded)
	}
	return nil
}

// loadExternalSkill reads a skill directory and returns an ExternalSkill.
func loadExternalSkill(skillDir string) (*ExternalSkill, error) {
	manifestPath := filepath.Join(skillDir, "skill.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("missing skill.yaml: %w", err)
	}

	var manifest ExternalSkillManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid skill.yaml: %w", err)
	}

	if manifest.Name == "" {
		return nil, fmt.Errorf("skill.yaml must specify a name")
	}
	if manifest.Description == "" {
		return nil, fmt.Errorf("skill.yaml must specify a description")
	}

	execPath, err := resolveExecutable(skillDir, manifest.Executable)
	if err != nil {
		return nil, err
	}

	return &ExternalSkill{
		manifest: manifest,
		execPath: execPath,
		skillDir: skillDir,
	}, nil
}

// resolveExecutable finds the executable for a skill.
// If manifest.Executable is set it is used directly; otherwise the first
// executable file in the skill directory is used.
func resolveExecutable(skillDir, hint string) (string, error) {
	if hint != "" {
		p := filepath.Join(skillDir, hint)
		if !filepath.IsAbs(hint) {
			p = filepath.Join(skillDir, hint)
		} else {
			p = hint
		}
		if err := checkExecutable(p); err != nil {
			return "", fmt.Errorf("executable %q: %w", hint, err)
		}
		return p, nil
	}

	// Auto-detect: find the first executable file in the directory
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return "", fmt.Errorf("cannot read skill directory: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "skill.yaml" {
			continue
		}
		p := filepath.Join(skillDir, e.Name())
		if checkExecutable(p) == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no executable file found in skill directory")
}

func checkExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("file is not executable")
	}
	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}
