package agent

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/skills"
)

func buildOpenAIInput(request ProviderRequest) []any {
	input := make([]any, 0, 3+len(request.ConversationContext)+len(request.History))
	input = append(input, openAIDeveloperMessage(buildProviderDeveloperPrompt(request)...))
	if contextMessage := buildProviderUserContextMessage(request); contextMessage != "" {
		input = append(input, openAIMessage("user", contextMessage))
	}
	hasConversationInput := false
	if strings.TrimSpace(request.ContextSummary) != "" {
		input = append(input, openAIMessage("user", "Conversation summary:\n"+strings.TrimSpace(request.ContextSummary)))
		hasConversationInput = true
	}
	for _, message := range request.ConversationContext {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		input = append(input, openAIMessage(message.Role, message.Content))
		hasConversationInput = true
	}
	if !hasConversationInput {
		input = append(input, openAIMessage("user", strings.TrimSpace(request.Prompt)))
	}
	for _, item := range request.History {
		switch item.Type {
		case "tool_call":
			input = append(input, openAIFunctionCallInput{
				Type:      "function_call",
				CallID:    item.ToolCall.CallID,
				Name:      item.ToolCall.Name,
				Arguments: item.ToolCall.Arguments,
			})
		case "tool_result":
			input = append(input, openAIFunctionCallOutput{
				Type:   "function_call_output",
				CallID: item.ToolResult.CallID,
				Output: item.ToolResult.Output,
			})
		case "text":
			if strings.TrimSpace(item.Text) != "" {
				input = append(input, openAIInputMessage{
					Type: "message",
					Role: "assistant",
					Content: []openAIInputContent{
						{Type: "output_text", Text: item.Text},
					},
				})
			}
		}
	}
	return input
}

func openAIMessage(role, text string) openAIInputMessage {
	if role != "assistant" {
		role = "user"
	}
	contentType := "input_text"
	if role == "assistant" {
		contentType = "output_text"
	}
	return openAIInputMessage{
		Type: "message",
		Role: role,
		Content: []openAIInputContent{
			{Type: contentType, Text: strings.TrimSpace(text)},
		},
	}
}

func openAIDeveloperMessage(parts ...string) openAIInputMessage {
	content := make([]openAIInputContent, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		content = append(content, openAIInputContent{Type: "input_text", Text: strings.TrimSpace(part)})
	}
	return openAIInputMessage{
		Type:    "message",
		Role:    "developer",
		Content: content,
	}
}

func buildSummaryInput(request SummaryRequest) []any {
	input := make([]any, 0, 2+len(request.Messages))
	input = append(input, openAIDeveloperMessage(summaryDeveloperPrompt()...))
	if strings.TrimSpace(request.ExistingSummary) != "" {
		input = append(input, openAIMessage("user", "Existing conversation summary:\n"+strings.TrimSpace(request.ExistingSummary)))
	}
	for _, message := range request.Messages {
		input = append(input, openAIMessage(message.Role, message.Content))
	}
	return input
}

func buildProviderPrompt(request ProviderRequest) string {
	return strings.Join(buildProviderDeveloperPrompt(request), "\n\n") + "\n\nCurrent user prompt:\n" + strings.TrimSpace(request.Prompt)
}

func skillPromptPath(skill skills.Skill) string {
	key := strings.TrimSpace(skill.Key)
	if key == "" {
		return ""
	}
	switch skill.Source {
	case "patchpilot":
		return "~/.patchpilot/skills/" + key + "/SKILL.md"
	case "agents":
		return "~/.agents/skills/" + key + "/SKILL.md"
	default:
		return ""
	}
}

func buildProviderUserContextMessage(request ProviderRequest) string {
	blocks := []string{}
	var builder strings.Builder
	blocks = append(blocks, xmlBlockPreserve(openAIXMLTagEnvironmentContext, buildEnvironmentContext(request)))

	if len(request.RepoInstructions) > 0 {
		builder.WriteString("Effective repo instructions, in precedence order:\n")
		for _, source := range request.RepoInstructions {
			if strings.TrimSpace(source.Content) == "" {
				continue
			}
			builder.WriteString("\nSource: ")
			builder.WriteString(source.Path)
			builder.WriteString("\n")
			builder.WriteString(source.Content)
			builder.WriteString("\n")
		}
	}
	if text := strings.TrimSpace(builder.String()); text != "" {
		blocks = append(blocks, xmlBlock(openAIXMLTagRepoInstructions, text))
	}

	builder.Reset()
	if len(request.ContextWarnings) > 0 {
		builder.WriteString("Context warnings:\n")
		for _, warning := range request.ContextWarnings {
			if warning.Path != "" {
				builder.WriteString("- ")
				builder.WriteString(warning.Path)
				builder.WriteString(": ")
			} else {
				builder.WriteString("- ")
			}
			builder.WriteString(warning.Message)
			builder.WriteString("\n")
		}
	}
	if text := strings.TrimSpace(builder.String()); text != "" {
		blocks = append(blocks, xmlBlock(openAIXMLTagContextWarnings, text))
	}
	return strings.Join(blocks, "\n\n")
}

func buildProviderDeveloperPrompt(request ProviderRequest) []string {
	blocks := []string{
		xmlBlock(openAIXMLTagPatchPilotAgent, providerInstructions()),
	}
	if text := buildSelectedSkillsBlock(request.SelectedSkills); text != "" {
		blocks = append(blocks, text)
	}
	return blocks
}

func buildSelectedSkillsBlock(selectedSkills []skills.Skill) string {
	var builder strings.Builder
	if len(selectedSkills) > 0 {
		builder.WriteString("Selected local skills:\n")
		for _, skill := range selectedSkills {
			if strings.TrimSpace(skill.Name) == "" || strings.TrimSpace(skill.Description) == "" {
				continue
			}
			builder.WriteString("\nName: ")
			builder.WriteString(skill.Name)
			builder.WriteString("\nDescription: ")
			builder.WriteString(skill.Description)
			builder.WriteString("\n")
			if path := skillPromptPath(skill); path != "" {
				builder.WriteString("Path: ")
				builder.WriteString(path)
				builder.WriteString("\n")
			}
		}
	}
	if text := strings.TrimSpace(builder.String()); text != "" {
		return xmlBlock(openAIXMLTagSkillsInstructions, text)
	}
	return ""
}

func buildEnvironmentContext(request ProviderRequest) string {
	now := time.Now()
	cwd := strings.TrimSpace(request.WorkspaceRoot)
	shell := strings.TrimSpace(request.Shell)
	if shell == "" {
		shell = filepath.Base(os.Getenv("SHELL"))
	}
	currentDate := strings.TrimSpace(request.CurrentDate)
	if currentDate == "" {
		currentDate = now.Format("2006-01-02")
	}
	timezone := strings.TrimSpace(request.Timezone)
	if timezone == "" {
		timezone = os.Getenv("TZ")
	}
	if timezone == "" {
		timezone = now.Location().String()
	}
	return strings.Join([]string{
		"\t" + xmlInlineBlock(openAIXMLTagCWD, cwd),
		"\t" + xmlInlineBlock(openAIXMLTagShell, shell),
		"\t" + xmlInlineBlock(openAIXMLTagCurrentDate, currentDate),
		"\t" + xmlInlineBlock(openAIXMLTagTimezone, timezone),
	}, "\n")
}

type openAIXMLTag string

func xmlBlock(tag openAIXMLTag, content string) string {
	name := string(tag)
	return "<" + name + ">\n" + strings.TrimSpace(content) + "\n</" + name + ">"
}

func xmlBlockPreserve(tag openAIXMLTag, content string) string {
	name := string(tag)
	return "<" + name + ">\n" + content + "\n</" + name + ">"
}

func xmlInlineBlock(tag openAIXMLTag, content string) string {
	name := string(tag)
	return "<" + name + ">" + strings.TrimSpace(content) + "</" + name + ">"
}

func providerInstructions() string {
	return `You are PatchPilot's coding agent.

Rules:
- Inspect context only from the server-provided conversation context and the available workspace tools.
- Return assistant text when useful.
- Use tools for workspace discovery, file reads, commands, and patches.
- When a local skill description is relevant and you need its detailed instructions, read the listed skill Path through run_command. Prefer cat <skill-path>; use sed chunks only when needed.
- When calling tools, include concise output_text in the same response that tells the user what you are checking or changing so the user sees progress while tool calls are pending.
- Write assistant text, including output_text sent with tool calls, in the same language as the user's prompt unless the user explicitly asks for a different language.
- If the user prompt asks for a change or investigation, do not answer with readiness, greetings, or "what would you like me to do" questions.
- For change or investigation prompts, first call at least one workspace inspection tool such as search_files, list_files, or run_command unless the answer can be completed entirely from prior tool results.
- Read file contents through run_command. Prefer sed chunks such as sed -n '1,160p' path/to/file. Use cat path/to/file only when the full file is needed.
- Inspect Git only through run_command. Prefer exact commands git status, git diff, and git log; do not call dedicated Git status/diff tools.
- Ask a clarifying question only when the user prompt is genuinely missing the target or desired outcome.
- Do not claim files were modified unless the apply_patch tool result says the patch was applied.
- Do not include secrets.
- apply_patch input must include "summary" and a complete git-apply-compatible unified "diff".
- Every changed file in a patch diff must include diff --git, ---/+++, and @@ hunk headers with line numbers.
- Tool calls in one response may run as a batch. Approval-required tools are decided by the user before the batch runs.`
}

func summaryDeveloperPrompt() []string {
	return []string{
		xmlBlock(openAIXMLTagSummaryTask, "Summarize older PatchPilot conversation context for a coding agent."),
		xmlBlock(openAIXMLTagSummaryRules, summaryRules()),
	}
}

func summaryRules() string {
	return `Rules:
- Preserve user intent, decisions, constraints, referenced files, commands, applied or rejected changes, and open tasks.
- Keep the summary concise and factual.
- Do not invent details.
- Do not include secrets, raw environment values, or host paths outside the workspace.`
}
