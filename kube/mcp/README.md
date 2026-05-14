# kube/mcp — Orphaned

Status: **orphaned / non-building** as of 2026-05.

This directory contains an early prototype of an MCP (Model Context Protocol)
server for exposing Argus tooling. It currently:

- Has **no `go.mod`** of its own and is not wired into any parent module.
- Is **not imported** by anything else in the repo (`kube/backend`, agents, or
  build scripts).
- Will **not compile** standalone.

It is kept in-tree pending a decision on whether to revive it as a proper
sub-module or remove it. The only external reference is a mention in the
top-level `.context.md` describing intent, not a build dependency.

Do not assume any code here is current or correct. Before extending it, either
restore the module wiring or delete the directory.
