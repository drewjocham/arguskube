package terminal

import (
	"os/exec"
	"strings"
)

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
