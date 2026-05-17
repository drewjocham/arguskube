# Font Exploration Report — Argus Terminal

**Reviewer:** @explore
**Date:** 2026-05-16

---

## 🚩 Root Cause: "Wrong Font" Issue

### Primary Cause: TerminalView.vue fontFamily

**File:** `kube/view/src/components/terminal/TerminalView.vue`, line 64

```js
fontFamily: "'Cascadia Mono', 'Cascadia Code', 'SF Mono', Consolas, monospace",
```

**Cascadia Mono and Cascadia Code are Microsoft fonts — NOT pre-installed on macOS.**

| Font | On macOS? | Result |
|------|-----------|--------|
| `Cascadia Mono` | **No** (only with VS Code) | Skipped |
| `Cascadia Code` | **No** (only with VS Code) | Skipped |
| `SF Mono` | **Yes** — macOS terminal font (with Xcode CLI tools) | Used (3rd fallback) |
| `Consolas` | **No** (Windows font) | Skipped |
| `monospace` | Yes — generic system font | Used if all else fails |

If the user has VS Code (which bundles Cascadia Mono), the terminal renders with a chunkier, squared-off "Windows Terminal look" that looks wrong on macOS.

If VS Code is NOT installed, the cascade falls all the way to `monospace` — typically Courier or Courier New — which looks ugly in a terminal context.

### Secondary Cause: Inconsistent Font Stacks

**File:** `kube/view/src/components/desktop/ProDesktopApp.vue`, line 174

```js
fontFamily: "var(--mono), 'Cascadia Mono', 'SF Mono', Consolas, monospace",
```

This is a **different font stack** from TerminalView! It starts with `var(--mono)` (resolves to 6 fonts from `theme.css`), then appends 4 more. The effective stack is 9 fonts long. **The embedded and pop-out terminals render with different fonts.**

### Tertiary Cause: Central CSS Variable

**File:** `kube/view/src/assets/theme.css`, line 33

```css
--mono: 'Cascadia Mono', 'Cascadia Code', 'SF Mono', Consolas, 'Liberation Mono', monospace;
```

This variable is not used by TerminalView.vue at all — it hardcodes its own stack. Only ProDesktopApp uses `var(--mono)`.

### Quaternary Cause: Font Size Inconsistency

| Component | fontFamily | fontSize |
|-----------|-----------|----------|
| TerminalView.vue (line 64-65) | `Cascadia Mono`, `Cascadia Code`, `SF Mono`, `Consolas`, `monospace` | **12px** |
| ProDesktopApp.vue (line 174-176) | `var(--mono), Cascadia Mono, SF Mono, Consolas, monospace` | **13px** |

The two terminals also have different **background colors** (TerminalView: `#1a1c1e`, ProDesktopApp: `#0d0d0d`).

---

## Font Scanning Across the Codebase

| File | Line | Font Stack |
|------|------|------------|
| `theme.css` | 33 | `Cascadia Mono, Cascadia Code, SF Mono, Consolas, Liberation Mono, monospace` |
| `TerminalView.vue` | 64 | `Cascadia Mono, Cascadia Code, SF Mono, Consolas, monospace` |
| `ProDesktopApp.vue` | 174 | `var(--mono), Cascadia Mono, SF Mono, Consolas, monospace` |
| `SlackPanel.vue` | 309 | `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace` |
| `TasksPanel.vue` | 380 | `ui-monospace, monospace` |
| `SheetsPanel.vue` | 263 | `ui-monospace, monospace` |
| `DocsPanel.vue` | 240 | `ui-monospace, SFMono-Regular, Menlo, monospace` |
| `GChatPanel.vue` | 153 | `ui-monospace, SFMono-Regular, Menlo, monospace` |

Note: Workspace panels correctly use `ui-monospace` (CSS system keyword for OS-preferred monospace). The terminal components do NOT use `ui-monospace`.

---

## Native Go Terminal — Font Loading

**File:** `lufis-terminal/internal/render/font.go`

```go
func fontPaths() []string {
    paths := []string{
        "/System/Library/Fonts/SFNSMono.ttf",
        "/System/Library/Fonts/Menlo.ttc",
        "/System/Library/Fonts/Supplemental/Courier New.ttf",
        "/System/Library/Fonts/Courier.ttf",
    }
```

**Issues:**

1. **SFNSMono.ttf exists** on this system — native terminal should render correctly
2. `extractFirstFont()` only extracts the first font from Menlo.ttc — Bold/Italic/BoldItalic variants are ignored
3. **No `FontFamily` config option** — users cannot choose fonts without recompiling (`config.go:16-21` has no `FontFamily` field)
4. Cell width is calculated as `cellW = cellH * 6 / 10` — rough heuristic, no glyph measurement
5. User font directory scan (`os.ReadDir(fd)`) silently discards errors
6. No JetBrains Mono or Nerd Font support (required per TC-11)

---

## Recommended Fixes

### Fix 1: Standardize font stack across terminal components (P0)

```js
fontFamily: "ui-monospace, 'SF Mono', Monaco, 'Cascadia Mono', Consolas, monospace",
```

Use `ui-monospace` as the first preference — it always resolves to the OS-preferred monospace font.

### Fix 2: Make TerminalView and ProDesktopApp use identical fontFamily (P0)

Extract to a shared constant or use `var(--mono)` consistently.

### Fix 3: Standardize font sizes (P1)

Pick 12px or 13px and use consistently. Document the rationale.

### Fix 4: Add JetBrains Mono bundle or reference (P1)

Per spec TC-11. Consider bundling the font file or linking to a CDN.

### Fix 5: Add FontFamily to native Go terminal config (P1)

```go
type TerminalConfig struct {
    // ...
    FontFamily string `toml:"font_family"`
}
```
