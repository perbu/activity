package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/perbu/activity/internal/config"
)

// ShowPrompts displays the current prompts being used (custom or default)
func ShowPrompts(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("show-prompts", flag.ExitOnError)
	showDefaults := flags.Bool("defaults", false, "Show default prompts even if custom ones are configured")

	if err := flags.Parse(args); err != nil {
		return err
	}

	// Determine which prompts to show
	phase2Prompt := ctx.Config.GetPhase2Prompt()
	agentPrompt := ctx.Config.GetAgentSystemPrompt()

	isPhase2Custom := ctx.Config.LLM.Phase2Prompt != ""
	isAgentCustom := ctx.Config.LLM.AgentSystemPrompt != ""

	// Print header
	fmt.Println("Current Prompts Configuration")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Show Phase 2 Prompt
	fmt.Println("Phase 2 (Simple LLM) Prompt:")
	if isPhase2Custom && !*showDefaults {
		fmt.Println("  Source: Custom (from config)")
	} else if *showDefaults {
		fmt.Println("  Source: Default")
		phase2Prompt = config.DefaultPhase2Prompt
	} else {
		fmt.Println("  Source: Default (no custom prompt configured)")
	}
	fmt.Println()
	fmt.Println(indentText(phase2Prompt, "  "))
	fmt.Println()
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println()

	// Show Agent Prompt
	fmt.Println("Phase 3 (Agent) System Prompt:")
	if isAgentCustom && !*showDefaults {
		fmt.Println("  Source: Custom (from config)")
	} else if *showDefaults {
		fmt.Println("  Source: Default")
		agentPrompt = config.DefaultAgentSystemPrompt
	} else {
		fmt.Println("  Source: Default (no custom prompt configured)")
	}
	fmt.Println()
	// Note about the %d placeholder
	fmt.Println("  Note: The '%d' placeholder is replaced with max_diff_fetches at runtime")
	fmt.Println()
	fmt.Println(indentText(agentPrompt, "  "))
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Show config info
	fmt.Println("Configuration:")
	fmt.Printf("  Current mode: %s\n", getCurrentMode(ctx))
	fmt.Printf("  Max diff fetches: %d\n", ctx.Config.LLM.MaxDiffFetches)
	fmt.Println()

	// Show how to customize
	if !isPhase2Custom && !isAgentCustom {
		fmt.Println("To customize prompts, add to your config.yaml:")
		fmt.Println()
		fmt.Println("  llm:")
		fmt.Println("    phase2_prompt: |")
		fmt.Println("      Your custom Phase 2 prompt here...")
		fmt.Println("    agent_system_prompt: |")
		fmt.Println("      Your custom agent system prompt here...")
		fmt.Println()
	}

	return nil
}

// getCurrentMode returns a description of the current analysis mode
func getCurrentMode(ctx *Context) string {
	if ctx.Config.LLM.UseAgent {
		return "Phase 3 (Agent-based with tools)"
	}
	return "Phase 2 (Simple LLM)"
}

// indentText adds indentation to each line of text
func indentText(text, indent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
