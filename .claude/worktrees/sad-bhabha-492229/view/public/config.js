// Runtime configuration for KubeWatcher frontend.
// In Docker/SaaS mode, this is overwritten at build time to use relative URLs.
// For local Wails dev, this file is a no-op (Wails bindings take priority).
window.__KUBEWATCHER_API_BASE__ = "http://localhost:8080";
