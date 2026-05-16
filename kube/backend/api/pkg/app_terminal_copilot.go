package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/argues/argus/internal/ai"
)

// ExplainTerminalOutput sends terminal output to the AI for analysis.
func (a *App) ExplainTerminalOutput(output string, domain string) (string, error) {
	if a.agent == nil || !a.agent.HasClient() {
		return "", fmt.Errorf("AI agent not configured — set DEEPSEEK_API_KEY")
	}
	if output == "" {
		return "", fmt.Errorf("no terminal output to analyze")
	}

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`Analyze this terminal output and explain:
1. What happened
2. Is this an error, warning, or expected?
3. If an error, what's the likely fix?
4. Next command to run?

Domain: %s

Output:
%s`, domainPrompt(domain), output)

	client := a.agent.Client()
	if client == nil {
		return "", fmt.Errorf("AI agent not configured")
	}

	resp, err := client.Chat(ctx, []ai.Message{
		{Role: "system", Content: explainSystemPrompt(domain)},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return "", fmt.Errorf("explain failed: %w", err)
	}

	return resp, nil
}

// GenerateCommand translates natural language to a CLI command.
func (a *App) GenerateCommand(userPrompt string, domain string) (string, error) {
	if a.agent == nil || !a.agent.HasClient() {
		return "", fmt.Errorf("AI agent not configured — set DEEPSEEK_API_KEY")
	}
	if userPrompt == "" {
		return "", fmt.Errorf("empty prompt")
	}

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	client := a.agent.Client()
	if client == nil {
		return "", fmt.Errorf("AI agent not configured")
	}

	resp, err := client.Chat(ctx, []ai.Message{
		{Role: "system", Content: commandSystemPrompt(domain)},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		return "", fmt.Errorf("command generation failed: %w", err)
	}

	return resp, nil
}

func explainSystemPrompt(domain string) string {
	switch domain {
	case "k8s":
		return `You are an expert Kubernetes SRE. Explain errors, suggest root causes, and provide kubectl commands. Be concise. Format commands in code blocks.`
	case "kafka":
		return `You are an expert Kafka operator. Explain errors, suggest fixes, and provide kcat/kafka-console commands. Be concise. Format commands in code blocks.`
	case "cloud":
		return `You are an expert Cloud SRE (GCP/AWS/Azure). Explain errors, suggest fixes, and provide gcloud/aws/az commands. Be concise. Format commands in code blocks.`
	default:
		return `You are an expert SRE. Explain errors, suggest root causes, and provide ready-to-paste commands. Be concise. Format commands in code blocks.`
	}
}

func commandSystemPrompt(domain string) string {
	switch domain {
	case "k8s":
		return `You translate natural language into kubectl commands. Output ONLY the command — no explanation, no markdown, no backticks. Examples: "list pods in prod" → kubectl get pods -n production`
	case "kafka":
		return `You translate natural language into Kafka CLI commands. Output ONLY the command. Examples: "list topics" → kafka-topics --bootstrap-server localhost:9092 --list`
	case "cloud":
		return `You translate natural language into cloud CLI commands. Output ONLY the command. Examples: "list instances" → gcloud compute instances list`
	default:
		return `You translate natural language into shell commands. Output ONLY the command — no explanation.`
	}
}

func domainPrompt(domain string) string {
	switch domain {
	case "k8s":
		return "Kubernetes (kubectl, helm, k9s)"
	case "kafka":
		return "Kafka (kcat, kafka-console-*)"
	case "cloud":
		return "Cloud CLI (gcloud, aws, az)"
	default:
		return "General shell"
	}
}
