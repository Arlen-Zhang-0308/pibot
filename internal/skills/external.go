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

	"github.com/pibot/pibot/internal/capabilities"
	"gopkg.in/yaml.v3"
)

// InstalledSkillInfo describes an installed external skill for API responses.
type InstalledSkillInfo struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Dir             string `json:"dir"`
	InstructionOnly bool   `json:"instruction_only"`
	ClawHubSlug     string `json:"clawhub_slug,omitempty"`
	ClawHubVersion  string `json:"clawhub_version,omitempty"`
}

// ExternalSkillManifest describes an external skill loaded from ~/.pibot_skills
type ExternalSkillManifest struct {
	Name            string                 `yaml:"name"`
	Description     string                 `yaml:"description"`
	Parameters      map[string]interface{} `yaml:"parameters"`
	Executable      string                 `yaml:"executable"`
	InstructionOnly bool                   `yaml:"instruction_only"`
	ClawHubSlug     string                 `yaml:"clawhub_slug"`
	ClawHubVersion  string                 `yaml:"clawhub_version"`
}

// ExternalSkill wraps a script/executable as a Skill
type ExternalSkill struct {
	manifest   ExternalSkillManifest
	execPath   string
	skillDir   string
}

// InstructionOnlySkill is an external skill backed by a markdown instructions
// file (no executable). The AI receives the instructions as tool output, which
// guides its behavior for that capability.
type InstructionOnlySkill struct {
	manifest     ExternalSkillManifest
	instructions string
	skillDir     string
}

func (s *InstructionOnlySkill) Name() string        { return s.manifest.Name }
func (s *InstructionOnlySkill) Description() string { return s.manifest.Description }
func (s *InstructionOnlySkill) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "What you want to do or know with this skill",
			},
		},
		"required": []string{"query"},
	}
}

// Execute returns the skill's instructions as context so the AI can apply them.
func (s *InstructionOnlySkill) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	if s.instructions != "" {
		return s.instructions, nil
	}
	return fmt.Sprintf("Skill %q is active. Description: %s", s.manifest.Name, s.manifest.Description), nil
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
	cmd.Env = os.Environ() // explicitly inherit the full server environment
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
// Each skill subdirectory must contain a skill.yaml or skill.md manifest file.
// skill.md uses YAML frontmatter (between --- delimiters) for name, description,
// parameters, and optional executable; the rest of the file is documentation.
func LoadExternalSkills(registry *capabilities.Registry, skillsDir string) error {
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
		registry.Register(skill, capabilities.KindSkill)
		log.Printf("Loaded external skill %q from %s", skill.Name(), skillDir)
		loaded++
	}

	if loaded > 0 {
		log.Printf("Loaded %d external skill(s) from %s", loaded, expanded)
	}
	return nil
}

// loadExternalSkill reads a skill directory and returns a Capability.
// For instruction-only skills (no executable required) it returns an
// InstructionOnlySkill; otherwise it returns an ExternalSkill.
// Manifest is read from skill.yaml, or from skill.md (YAML frontmatter).
func loadExternalSkill(skillDir string) (capabilities.Capability, error) {
	manifest, rawContent, err := readManifest(skillDir)
	if err != nil {
		return nil, err
	}

	if manifest.Name == "" {
		return nil, fmt.Errorf("manifest must specify a name")
	}
	if manifest.Description == "" {
		return nil, fmt.Errorf("manifest must specify a description")
	}

	// Instruction-only mode: no executable needed.
	if manifest.InstructionOnly {
		instructions := extractInstructions(skillDir, rawContent)
		return &InstructionOnlySkill{
			manifest:     manifest,
			instructions: instructions,
			skillDir:     skillDir,
		}, nil
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

// extractInstructions returns the body of a skill.md file (after frontmatter)
// or the content of any instructions.md/SKILL.md found in the directory.
func extractInstructions(skillDir string, rawContent []byte) string {
	// If rawContent came from skill.md, strip the frontmatter.
	if len(rawContent) > 0 {
		const delim = "---"
		lines := strings.Split(string(rawContent), "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[0]) == delim {
			for i := 1; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == delim {
					return strings.Join(lines[i+1:], "\n")
				}
			}
		}
	}

	// Fall back to SKILL.md or instructions.md in the directory.
	for _, candidate := range []string{"SKILL.md", "skill.md", "instructions.md"} {
		data, err := os.ReadFile(filepath.Join(skillDir, candidate))
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// UnloadExternalSkill removes the named skill from the registry.
func UnloadExternalSkill(registry *capabilities.Registry, name string) bool {
	return registry.Unregister(name)
}

// LoadSingleSkill loads and registers one skill from the given directory.
// If a skill with the same name is already registered it is replaced.
func LoadSingleSkill(registry *capabilities.Registry, skillDir string) error {
	skill, err := loadExternalSkill(skillDir)
	if err != nil {
		return err
	}
	// Remove any previous version first (idempotent install).
	registry.Unregister(skill.Name())
	registry.Register(skill, capabilities.KindSkill)
	log.Printf("Hot-loaded external skill %q from %s", skill.Name(), skillDir)
	return nil
}

// ListInstalledSkills returns metadata for every skill found in skillsDir.
func ListInstalledSkills(skillsDir string) ([]InstalledSkillInfo, error) {
	expanded := expandHome(skillsDir)
	entries, err := os.ReadDir(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return []InstalledSkillInfo{}, nil
		}
		return nil, fmt.Errorf("cannot read skills directory: %w", err)
	}

	var infos []InstalledSkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(expanded, entry.Name())
		manifest, _, err := readManifest(skillDir)
		if err != nil {
			// No manifest found — still surface the directory with fallback metadata.
			infos = append(infos, InstalledSkillInfo{
				Name:        entry.Name(),
				Description: "(no description — manifest file missing)",
				Dir:         entry.Name(),
			})
			continue
		}
		infos = append(infos, InstalledSkillInfo{
			Name:            manifest.Name,
			Description:     manifest.Description,
			Dir:             entry.Name(),
			InstructionOnly: manifest.InstructionOnly,
			ClawHubSlug:     manifest.ClawHubSlug,
			ClawHubVersion:  manifest.ClawHubVersion,
		})
	}
	return infos, nil
}

// clawMetaFile is the JSON metadata file included in ClawHub skill zips.
type clawMetaFile struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Summary     string `json:"summary"`
	Version     string `json:"version"`
	Slug        string `json:"slug"`
	Entry       string `json:"entry"`
}

// readManifest loads a skill manifest from the skill directory.
// It tries the following sources in order:
//  1. skill.yaml  — pibot YAML manifest
//  2. skill.md    — markdown with YAML frontmatter
//  3. meta        — ClawHub JSON metadata file (as shipped in the zip)
//
// Returns manifest, raw file content (populated for .md; nil otherwise), and error.
func readManifest(skillDir string) (ExternalSkillManifest, []byte, error) {
	var manifest ExternalSkillManifest

	// 1. skill.yaml
	yamlPath := filepath.Join(skillDir, "skill.yaml")
	data, err := os.ReadFile(yamlPath)
	if err == nil {
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return manifest, nil, fmt.Errorf("invalid skill.yaml: %w", err)
		}
		return manifest, data, nil
	}
	if !os.IsNotExist(err) {
		return manifest, nil, fmt.Errorf("reading skill.yaml: %w", err)
	}

	// 2. skill.md (YAML frontmatter)
	mdPath := filepath.Join(skillDir, "skill.md")
	data, err = os.ReadFile(mdPath)
	if err == nil {
		frontmatter, err := parseYAMLFrontmatter(data)
		if err != nil {
			return manifest, nil, fmt.Errorf("invalid skill.md frontmatter: %w", err)
		}
		if err := yaml.Unmarshal(frontmatter, &manifest); err != nil {
			return manifest, nil, fmt.Errorf("invalid skill.md frontmatter: %w", err)
		}
		return manifest, data, nil
	}
	if !os.IsNotExist(err) {
		return manifest, nil, fmt.Errorf("reading skill.md: %w", err)
	}

	// 3. meta (ClawHub JSON metadata shipped in the zip)
	metaPath := filepath.Join(skillDir, "meta")
	data, err = os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return manifest, nil, fmt.Errorf("missing skill.yaml, skill.md, or meta file")
		}
		return manifest, nil, fmt.Errorf("reading meta: %w", err)
	}
	var claw clawMetaFile
	if err := json.Unmarshal(data, &claw); err != nil {
		return manifest, nil, fmt.Errorf("invalid meta JSON: %w", err)
	}
	manifest.Name = claw.DisplayName
	if manifest.Name == "" {
		manifest.Name = claw.Name
	}
	manifest.Description = claw.Description
	if manifest.Description == "" {
		manifest.Description = claw.Summary
	}
	manifest.InstructionOnly = true
	manifest.ClawHubSlug = claw.Slug
	manifest.ClawHubVersion = claw.Version
	return manifest, nil, nil
}

// parseYAMLFrontmatter extracts the first --- ... --- block from md content.
func parseYAMLFrontmatter(md []byte) ([]byte, error) {
	const delim = "---"
	lines := strings.Split(string(md), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != delim {
		return nil, fmt.Errorf("markdown must start with %q", delim)
	}
	var block []string
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == delim {
			return []byte(strings.Join(block, "\n")), nil
		}
		block = append(block, lines[i])
	}
	return nil, fmt.Errorf("no closing %q found", delim)
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
	skipFiles := map[string]bool{
		"skill.yaml": true,
		"skill.md":   true,
		"SKILL.md":   true,
		"meta":       true,
		"README.md":  true,
		"readme.md":  true,
	}
	for _, e := range entries {
		if e.IsDir() || skipFiles[e.Name()] {
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
