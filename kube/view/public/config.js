// Runtime configuration for Argus frontend.
//
// Docker / SaaS deploys overwrite this file at build time to point
// the frontend at the right API origin (typically a relative URL).
// For local Wails dev this file is INTENTIONALLY a no-op — we leave
// `window.__argus_API_BASE__` unset so the bridge falls through to its
// safe default of `http://127.0.0.1:8080`.
//
// History: this used to set the variable to `http://localhost:8080`.
// On macOS that name resolves to ::1 (IPv6) before 127.0.0.1, and the
// backend binds IPv4 only. The fetch failed with a generic "Load
// failed" before falling back, and `make no-auth-run` looked broken
// for everyone running on macOS. Leave the variable unset; ship-time
// overrides still work because they replace the whole file.
