// Package agents describes supported coding-agent backends.
package agents

// Agent describes a coding-agent CLI and its supported invocation flags.
type Agent struct {
	Key              string   `json:"key"`
	Label            string   `json:"label"`
	Binary           string   `json:"binary"`
	HomeEnv          string   `json:"home_env"`
	HomeDir          string   `json:"home_dir"`
	InstallScript    string   `json:"install_script"`
	DocsURL          string   `json:"docs_url"`
	SkillTrigger     string   `json:"skill_trigger"`
	SystemPromptFlag string   `json:"system_prompt_flag"`
	PromptFlag       string   `json:"prompt_flag"`
	SelfUpdateArgs   []string `json:"self_update_args"`
	DSPFlags         []string `json:"dsp_flags"`
	AutoFlags        []string `json:"auto_flags"`
}

const Default = "claude"

var registry = []Agent{
	{Key: "claude", Label: "Claude Code", Binary: "claude", HomeEnv: "CLAUDE_CONFIG_DIR", HomeDir: ".claude", InstallScript: "install_claude.sh", DocsURL: "https://claude.ai/download", SkillTrigger: "/", SystemPromptFlag: "--append-system-prompt", PromptFlag: "", SelfUpdateArgs: []string{"update"}, DSPFlags: []string{"--dangerously-skip-permissions"}, AutoFlags: []string{"--permission-mode", "auto"}},
	{Key: "codex", Label: "OpenAI Codex", Binary: "codex", HomeEnv: "CODEX_HOME", HomeDir: ".codex", InstallScript: "install_codex.sh", DocsURL: "https://developers.openai.com/codex", SkillTrigger: "$", SystemPromptFlag: "", PromptFlag: "", SelfUpdateArgs: []string{"update"}, DSPFlags: []string{"--dangerously-bypass-approvals-and-sandbox"}, AutoFlags: []string{"--sandbox", "workspace-write", "--ask-for-approval", "never"}},
	{Key: "grok", Label: "Grok Build", Binary: "grok", HomeEnv: "GROK_HOME", HomeDir: ".grok", InstallScript: "install_grok.sh", DocsURL: "https://docs.x.ai/build/overview", SkillTrigger: "/", SystemPromptFlag: "", PromptFlag: "", SelfUpdateArgs: []string{"update"}, DSPFlags: []string{"--always-approve"}, AutoFlags: []string{"--permission-mode", "auto"}},
	{Key: "opencode", Label: "OpenCode", Binary: "opencode", HomeEnv: "OPENCODE_CONFIG_DIR", HomeDir: ".config/opencode", InstallScript: "install_opencode.sh", DocsURL: "https://opencode.ai/docs/", SkillTrigger: "Use the skill ", SystemPromptFlag: "", PromptFlag: "--prompt", SelfUpdateArgs: []string{"upgrade"}, DSPFlags: []string{"--auto"}, AutoFlags: []string{"--auto"}},
	{Key: "kilo", Label: "Kilo Code", Binary: "kilo", HomeEnv: "", HomeDir: ".kilo", InstallScript: "install_kilo.sh", DocsURL: "https://kilo.ai/docs/code-with-ai/platforms/cli", SkillTrigger: "Use the skill ", SystemPromptFlag: "", PromptFlag: "--prompt", SelfUpdateArgs: []string{"upgrade"}, DSPFlags: []string{"--auto"}, AutoFlags: []string{"--auto"}},
}
