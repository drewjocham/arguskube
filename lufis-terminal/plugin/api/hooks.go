package api

type Plugin interface {
	Name() string
	Version() string
	Init(host HostAPI) error
	Shutdown() error
}

type HostAPI interface {
	Logger() Logger
	RegisterCommand(name string, fn func(args []string) error)
	RegisterSidebarPanel(name string, panel Panel)
	RegisterHook(hook HookID, fn func(args interface{}) error)
}

type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

type HookID string

const (
	HookCommandExecuted HookID = "command_executed"
	HookKeyEvent        HookID = "key_event"
	HookRender          HookID = "render"
	HookSessionEvent    HookID = "session_event"
	HookPaneCreated     HookID = "pane_created"
	HookPaneClosed      HookID = "pane_closed"
)

type Panel struct {
	Title    string
	Content  func() string
	Priority int
}

type Command struct {
	Name        string
	Description string
	Execute     func(args []string) error
}

type Completion struct {
	Text        string
	DisplayText string
	Detail      string
}

type CompletionProvider func(cmd string, cursor int) []Completion
