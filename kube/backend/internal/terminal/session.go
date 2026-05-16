package terminal

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Domain string

const (
	DomainDefault Domain = "default"
	DomainK8s     Domain = "k8s"
	DomainKafka   Domain = "kafka"
	DomainCloud   Domain = "cloud"
)

type Session struct {
	ID       string
	Domain   Domain
	Label    string
	Terminal *Terminal
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	logger   *slog.Logger
}

func NewSessionManager(logger *slog.Logger) *SessionManager {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &SessionManager{
		sessions: make(map[string]*Session),
		logger:   logger,
	}
}

func (sm *SessionManager) NewSession(id string, domain Domain, label string, rows, cols uint16, extraEnv []string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[id]; exists {
		return nil, fmt.Errorf("session %q already exists", id)
	}

	term := New(sm.logger.With("session", id, "domain", string(domain)))

	env := []string{
		"ARGUS_SESSION_ID=" + id,
		"ARGUS_SESSION_DOMAIN=" + string(domain),
	}
	env = append(env, kwDomainEnv(domain)...)
	env = append(env, extraEnv...)

	if err := term.StartWithEnv("", rows, cols, env); err != nil {
		return nil, err
	}

	sess := &Session{
		ID:       id,
		Domain:   domain,
		Label:    label,
		Terminal: term,
	}
	sm.sessions[id] = sess

	sm.logger.Info("session started",
		slog.String("session_id", id),
		slog.String("domain", string(domain)),
		slog.String("label", label),
	)

	return sess, nil
}

func (sm *SessionManager) GetSession(id string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[id]
}

func (sm *SessionManager) CloseSession(id string) error {
	sm.mu.Lock()
	sess, ok := sm.sessions[id]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("session %q not found", id)
	}
	delete(sm.sessions, id)
	sm.mu.Unlock()

	err := sess.Terminal.Close()
	sm.logger.Info("session closed",
		slog.String("session_id", id),
		slog.String("domain", string(sess.Domain)),
	)
	return err
}

func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	ids := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		ids = append(ids, id)
	}
	sm.mu.Unlock()

	for _, id := range ids {
		_ = sm.CloseSession(id)
	}
}

func (sm *SessionManager) ListSessions() []SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	infos := make([]SessionInfo, 0, len(sm.sessions))
	for id, sess := range sm.sessions {
		infos = append(infos, SessionInfo{
			ID:     id,
			Domain: string(sess.Domain),
			Label:  sess.Label,
			Alive:  sess.Terminal.IsRunning(),
		})
	}
	return infos
}

type SessionInfo struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
	Label  string `json:"label"`
	Alive  bool   `json:"alive"`
}

// WriteWelcome writes a welcome banner with tool detection into the PTY.
func WriteWelcome(term *Terminal, domain Domain, env []string) {
	var tools []string
	for _, name := range domainTools(domain) {
		if _, err := exec.LookPath(name); err == nil {
			tools = append(tools, name)
		}
	}

	var banner strings.Builder
	banner.WriteString("\n")

	switch domain {
	case DomainK8s:
		banner.WriteString("\x1b[36m┌─ Argus K8s Session ──────────────────────────────┐\x1b[0m\n")
	case DomainKafka:
		banner.WriteString("\x1b[35m┌─ Argus Kafka Session ────────────────────────────┐\x1b[0m\n")
	case DomainCloud:
		banner.WriteString("\x1b[33m┌─ Argus Cloud Session ─────────────────────────────┐\x1b[0m\n")
	default:
		banner.WriteString("\x1b[32m┌─ Argus Terminal ──────────────────────────────────┐\x1b[0m\n")
	}

	if len(tools) > 0 {
		banner.WriteString("│ Tools: \x1b[32m" + strings.Join(tools, ", ") + "\x1b[0m\n")
	} else {
		banner.WriteString("│ \x1b[33mNo domain-specific tools detected\x1b[0m\n")
	}

	for _, line := range strings.Split(filterKwEnv(env), "\n") {
		if line != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && parts[1] != "" {
				banner.WriteString("│ " + parts[0] + "=" + parts[1] + "\n")
			}
		}
	}

	banner.WriteString("\x1b[90m└────────────────────────────────────────────────┘\x1b[0m\n")
	banner.WriteString("\x1b[90m  Type \x1b[32mkw help\x1b[90m for quick commands\x1b[0m\n\n")

	_ = term.Write(banner.String())

	if src := kwSourceCmd(); src != "" {
		_ = term.Write(src)
	}
}

func domainTools(d Domain) []string {
	switch d {
	case DomainK8s:
		return []string{"kubectl", "helm", "kubectx", "kubens", "k9s", "stern", "popeye", "trivy"}
	case DomainKafka:
		return []string{"kcat", "kafka-console-consumer", "kafka-console-producer", "kafka-topics", "kafka-consumer-groups", "kafka-avro-console-consumer"}
	case DomainCloud:
		return []string{"gcloud", "aws", "az", "terraform", "tofu", "pulumi"}
	default:
		return nil
	}
}

func kwDomainEnv(domain Domain) []string {
	switch domain {
	case DomainK8s:
		return []string{"KW_DOMAIN=k8s"}
	case DomainKafka:
		return []string{"KW_DOMAIN=kafka"}
	case DomainCloud:
		return []string{"KW_DOMAIN=cloud"}
	default:
		return []string{"KW_DOMAIN=default"}
	}
}

func filterKwEnv(env []string) string {
	var lines []string
	for _, e := range env {
		if strings.HasPrefix(e, "ARGUS_") || strings.HasPrefix(e, "KUBECONFIG") ||
			strings.HasPrefix(e, "CLOUDSDK_") || strings.HasPrefix(e, "AWS_") ||
			strings.HasPrefix(e, "KW_") {
			lines = append(lines, e)
		}
	}
	return strings.Join(lines, "\n")
}

// ── kw shell helper ─────────────────────────────────────────────

func initKwScript() string {
	dir := os.TempDir()
	path := dir + "/.argus_kw.sh"
	if _, err := os.Stat(path); err == nil {
		return path
	}
	script := `# Argus kw — quick domain operations
kw() {
  local cmd="$1"; shift
  case "$cmd" in
    pods|po) kubectl get pods "$@" ;;
    deploy|deployments) kubectl get deployments "$@" ;;
    svc|services) kubectl get services "$@" ;;
    events|ev) kubectl get events --sort-by='.lastTimestamp' "$@" ;;
    logs) kubectl logs "$@" ;;
    ctx)
      if [ $# -eq 0 ]; then kubectl config current-context 2>/dev/null || echo "no context"
      else kubectl config use-context "$1"; fi ;;
    ns|namespace)
      if [ $# -eq 0 ]; then kubectl config view --minify -o jsonpath='{..namespace}'
      else kubectl config set-context --current --namespace "$1"; fi ;;
    desc|describe) kubectl describe "$@" ;;
    top) kubectl top "$@" ;;
    exec)
      if [ $# -lt 1 ]; then echo "Usage: kw exec <pod> [command]"; return 1; fi
      kubectl exec -it "$@" ;;
    kafka|kfk)
      local kcmd="$1"; shift
      case "$kcmd" in
        topics) kafka-topics --bootstrap-server "${KAFKA_BROKERS:-localhost:9092}" --list "$@" ;;
        consume) kcat -b "${KAFKA_BROKERS:-localhost:9092}" -t "$1" -o -10 -e ;;
        produce) kcat -b "${KAFKA_BROKERS:-localhost:9092}" -t "$1" -P ;;
        groups) kafka-consumer-groups --bootstrap-server "${KAFKA_BROKERS:-localhost:9092}" --list "$@" ;;
        *) kcat "$@" ;;
      esac ;;
    gcloud|gcp)
      if [ $# -eq 0 ]; then gcloud config list project 2>/dev/null || echo "gcloud not configured"
      else gcloud "$@"; fi ;;
    aws)
      if [ $# -eq 0 ]; then aws sts get-caller-identity 2>/dev/null || echo "AWS not configured"
      else aws "$@"; fi ;;
    help|--help|-h)
      echo "Argus kw — quick domain operations"
      echo ""
      echo "  Kubernetes:"
      echo "    kw pods, kw deploy, kw svc, kw events, kw logs <pod>"
      echo "    kw ctx [name], kw ns [name], kw exec <pod> [cmd]"
      echo "    kw top, kw desc <resource>"
      echo ""
      echo "  Kafka:"
      echo "    kw kafka topics, kw kafka consume <topic>"
      echo "    kw kafka produce <topic>, kw kafka groups"
      echo ""
      echo "  Cloud:"
      echo "    kw gcloud [args], kw aws [args]"
      ;;
    *) echo "kw: unknown command '$cmd'. Try: kw help" ;;
  esac
}
`
	_ = os.WriteFile(path, []byte(script), 0644)
	return path
}

func kwSourceCmd() string {
	path := initKwScript()
	if path == "" {
		return ""
	}
	return "source " + path + " 2>/dev/null\n"
}
