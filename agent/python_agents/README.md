# Argus Agents

Multi-agent AI system for Argus — a framework of specialised LLM-powered agents for Kubernetes operations.

## Installation

```bash
pip install -e .
```

## Usage

```bash
# Run as a module
python -m argus_agents diagnose "Pod CrashLoopBackOff in production"

# Or via installed CLI
argus-agent analyze "CPU usage at 95% across cluster"

# Interactive session
argus-agent interactive

# Start HTTP server
argus-agent serve --port 8080
```

## Agents

| Agent | Purpose |
|-------|---------|
| **Orchestrator** | Routes requests to specialist agents, manages sessions, proactive background loop |
| **Diagnosis** | Root cause analysis for alerts and incidents |
| **Remediation** | Actionable fix steps and kubectl commands |
| **Analysis** | Cluster health, performance metrics, SLO tracking |
| **Security** | CVE triage, RBAC audit, misconfiguration detection |
| **Cost** | FinOps analysis, right-sizing, spot instance recommendations |
| **Docs** | Runbook and postmortem generation |
| **Context** | Session/memory management, document indexing for RAG |
| **Proactive** | Behavior pattern detection, pre-fetching, background insights |
