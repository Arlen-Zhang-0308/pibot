package skills

import (
	"github.com/pibot/pibot/internal/capabilities"
)

// Skill defines the interface for an external script-based skill.
// Skills are loaded from manifest files (skill.yaml / skill.md) in the skills
// directory and execute external scripts or executables. The Agent reads the
// skill's description and documentation to reason about how to use them.
//
// Skill is a type alias for capabilities.Capability — both share the same
// method set (Name, Description, Parameters, Execute).
type Skill = capabilities.Capability
