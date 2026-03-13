---
name: PiBot Skills and Prompts
overview: Add a skills/tools system with tool call support and system/user prompt templates to give PiBot its identity as a Raspberry Pi AI assistant capable of executing commands and managing files.
todos:
  - id: skills-package
    content: Create internal/skills package with skill interface and registry
    status: completed
  - id: skill-implementations
    content: "Implement core skills: execute_command, read_file, write_file, list_directory, system_info"
    status: completed
    dependencies:
      - skills-package
  - id: prompts-package
    content: Create internal/prompts package with system prompt template and tool definitions
    status: in_progress
    dependencies:
      - skills-package
  - id: tool-call-parser
    content: Add tool call parsing and execution loop in ai/provider.go
    status: pending
    dependencies:
      - skill-implementations
      - prompts-package
  - id: websocket-integration
    content: Update websocket.go to inject system prompt and handle tool calls in streaming
    status: pending
    dependencies:
      - tool-call-parser
  - id: config-update
    content: Add prompts section to config.yaml with customizable system prompt
    status: pending
    dependencies:
      - prompts-package
---

# PiBot Skills and Prompt System

## Problem

The screenshot shows PiBot responding as a generic LLM without awareness of its identity or capabilities. When asked "What is your current directory?", it claims to have no filesystem access, despite PiBot having command execution and file operation capabilities.

## Solution Architecture

```mermaid
flowchart TB
    subgraph prompts [Prompt Layer]
        SystemPrompt[System Prompt Template]
        ToolDefs[Tool Definitions]
    end
    
    subgraph skills [Skills Package]
        SkillRegistry[Skill Registry]
        ExecSkill[execute_command]
        FileReadSkill[read_file]
        FileWriteSkill[write_file]
        FileListSkill[list_directory]
        SystemInfoSkill[system_info]
    end
    
    subgraph ai [AI Layer]
        ProviderInterface[Provider Interface]
        ToolCallParser[Tool Call Parser]
    end
    
    SystemPrompt --> ProviderInterface
    ToolDefs --> ProviderInterface
    ProviderInterface --> ToolCallParser
    ToolCallParser --> SkillRegistry
    SkillRegistry --> ExecSkill
    SkillRegistry --> FileReadSkill
    SkillRegistry --> FileWriteSkill
    SkillRegistry --> FileListSkill
    SkillRegistry --> SystemInfoSkill