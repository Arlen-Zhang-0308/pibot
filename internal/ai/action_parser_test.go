package ai

import (
	"testing"
)

func TestParseActions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		types    []string
	}{
		{
			name:     "no actions",
			input:    "Just a regular response without any actions.",
			expected: 0,
			types:    nil,
		},
		{
			name: "single execute_command",
			input: `Let me check that for you.

<action type="execute_command">
command: ls -la
</action>

I'll show you the results.`,
			expected: 1,
			types:    []string{"execute_command"},
		},
		{
			name: "system_info action",
			input: `<action type="system_info">
</action>`,
			expected: 1,
			types:    []string{"system_info"},
		},
		{
			name: "multiple actions",
			input: `First, let me get system info.

<action type="system_info">
</action>

Then I'll list the files.

<action type="execute_command">
command: ls -la /home
</action>`,
			expected: 2,
			types:    []string{"system_info", "execute_command"},
		},
		{
			name: "read_file action",
			input: `<action type="read_file">
path: /etc/hostname
</action>`,
			expected: 1,
			types:    []string{"read_file"},
		},
		{
			name: "write_file with multiline content",
			input: `<action type="write_file">
path: /tmp/test.txt
content: |
  Line 1
  Line 2
  Line 3
</action>`,
			expected: 1,
			types:    []string{"write_file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := ParseActions(tt.input)
			if len(actions) != tt.expected {
				t.Errorf("ParseActions() got %d actions, want %d", len(actions), tt.expected)
			}
			for i, action := range actions {
				if i < len(tt.types) && action.Type != tt.types[i] {
					t.Errorf("Action[%d] type = %s, want %s", i, action.Type, tt.types[i])
				}
			}
		})
	}
}

func TestParseActionParams(t *testing.T) {
	input := `Let me run that command.

<action type="execute_command">
command: uname -a
</action>`

	actions := ParseActions(input)
	if len(actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(actions))
	}

	action := actions[0]
	if action.Type != "execute_command" {
		t.Errorf("Expected type 'execute_command', got %s", action.Type)
	}

	if cmd, ok := action.Parameters["command"]; !ok || cmd != "uname -a" {
		t.Errorf("Expected command 'uname -a', got %v", action.Parameters["command"])
	}
}

func TestHasActions(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"No actions here", false},
		{`<action type="system_info"></action>`, true},
		{`Some text <action type="execute_command">
command: ls
</action> more text`, true},
	}

	for _, tt := range tests {
		result := HasActions(tt.input)
		if result != tt.expected {
			t.Errorf("HasActions(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestRemoveActions(t *testing.T) {
	input := `Before action

<action type="system_info">
</action>

After action`

	result := RemoveActions(input)
	if HasActions(result) {
		t.Errorf("RemoveActions() still contains actions: %s", result)
	}
}
