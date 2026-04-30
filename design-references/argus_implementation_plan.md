# Argus Hybrid SaaS Implementation Plan

This document outlines the sequential execution plan to implement the three major architectural pillars for the KubeWatcher/Argus platform. As a single autonomous agent, I will execute these phases systematically.

## Phase 1: The Argus Agent Tunnel (Hub and Spoke)
**Goal:** Enable the in-cluster agent to stream data outwards to the GCP SaaS backend, bypassing NAT/Firewalls.

1. **SaaS WebSocket Hub (`backend/api/pkg/hub.go`)**
   - Implement a concurrent WebSocket server in the Go backend.
   - Handle incoming connections from agents, authenticate them via a registration token, and map them to specific user organizations/namespaces.
2. **Agent Outbound Client (`backend/agent/internal/tunnel/client.go`)**
   - Update the in-cluster agent to initiate a persistent `gorilla/websocket` connection to `wss://api.kubewatcher.io/tunnel`.
   - Implement automatic reconnection and exponential backoff.
3. **Data Streaming**
   - Refactor the agent's informers to push anomaly payloads and metric summaries over the tunnel rather than waiting for the Desktop app to pull them.

## Phase 2: Standalone Terminal App (Warp-style)
**Goal:** Create a lightweight, highly-responsive desktop application dedicated solely to the AI-enhanced terminal experience.

1. **Terminal Entrypoint (`backend/cmd/terminal/main.go`)**
   - Create a separate Wails build target that strictly loads a `/terminal-only` Vue route.
   - Configure the native macOS/Windows window to be frameless, transparent, and floating (to mimic the Warp/Raycast experience).
2. **PTY Integration Enhancement (`backend/mcp/internal/terminal`)**
   - Ensure the Go `creack/pty` implementation supports rich resizing, scrollback buffers, and robust interrupt signaling (Ctrl+C).
3. **Frontend Polish (`view/src/components/desktop/ProDesktopApp.vue`)**
   - Adapt the terminal view to act as the root component, hiding the sidebar and navigation.

## Phase 3: ArgusCD (Continuous Deployment)
**Goal:** Allow the SaaS platform to autonomously or manually push Kubernetes manifests down to the cluster via the Agent.

1. **Command Router**
   - Enhance the Phase 1 WebSocket tunnel to handle **bi-directional** communication. The SaaS backend needs to send RPC commands (like `ApplyManifest` or `RollbackDeployment`) down to the Agent.
2. **Agent Applier (`backend/agent/internal/cd/applier.go`)**
   - Equip the agent with Server-Side Apply (SSA) capabilities using `k8s.io/client-go` dynamic client.
   - The agent receives the YAML manifest, applies it locally using its ServiceAccount permissions, and streams the rollout status back up the tunnel.
3. **SaaS UI Integration**
   - Add "Deploy" and "Sync" actions to the Wails/Vue frontend that trigger the backend to broadcast the deployment command to the active cluster tunnel.

---
### Execution Strategy
I will act as your dedicated engineering partner. While I cannot spawn "parallel sub-agents", I will work through these phases sequentially at high speed. 

**Shall we begin immediately with Phase 1: Building the WebSocket Hub and Agent Tunnel?**
