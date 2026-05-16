package opencode

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type SessionStatus string

const (
	StatusIdle      SessionStatus = "idle"
	StatusThinking  SessionStatus = "thinking"
	StatusExecuting SessionStatus = "executing"
	StatusDone      SessionStatus = "done"
	StatusError     SessionStatus = "error"
)

type Session struct {
	ID       string
	Status   SessionStatus
	Model    string
	Messages []Message
	Workdir  string
}

type ModelProvider string

const (
	ProviderOpenAI    ModelProvider = "openai"
	ProviderAnthropic ModelProvider = "anthropic"
	ProviderDeepSeek  ModelProvider = "deepseek"
	ProviderOllama    ModelProvider = "ollama"
	ProviderGeneric   ModelProvider = "generic"
)

type ModelConfig struct {
	Provider ModelProvider
	Model    string
	APIKey   string
	BaseURL  string
}

type ToolResult struct {
	Success bool
	Output  string
	Error   string
}
