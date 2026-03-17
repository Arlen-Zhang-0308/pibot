package prompts

import "embed"

//go:embed files
var promptFiles embed.FS

// readPromptFile returns the raw bytes of a file inside the embedded files/ directory.
func readPromptFile(name string) ([]byte, error) {
	return promptFiles.ReadFile("files/" + name)
}
