// Shared xterm.js theme used by all terminal instances
export interface TerminalTheme {
  background: string
  foreground: string
  cursor: string
  cursorAccent: string
  selectionBackground: string
  black: string
  red: string
  green: string
  yellow: string
  blue: string
  magenta: string
  cyan: string
  white: string
  brightBlack: string
  brightRed: string
  brightGreen: string
  brightYellow: string
  brightBlue: string
  brightMagenta: string
  brightCyan: string
  brightWhite: string
}

export const TERMINAL_THEME: TerminalTheme = {
  background: '#1a1c1e',
  foreground: '#e8eaec',
  cursor: '#4f8ef7',
  cursorAccent: '#1a1c1e',
  selectionBackground: 'rgba(79,142,247,0.25)',
  black: '#1a1c1e',
  red: '#f05454',
  green: '#3ecf8e',
  yellow: '#f5a623',
  blue: '#4f8ef7',
  magenta: '#a78bfa',
  cyan: '#2dd4bf',
  white: '#e8eaec',
  brightBlack: '#5c6168',
  brightRed: '#ff7575',
  brightGreen: '#5edba6',
  brightYellow: '#ffc04d',
  brightBlue: '#6ba3f9',
  brightMagenta: '#c4b3fd',
  brightCyan: '#5ee8d4',
  brightWhite: '#ffffff',
}

// macOS-first monospace font stack.
// ui-monospace is the CSS system keyword for the OS-preferred monospace font.
// SF Mono ships with macOS (Xcode CLI Tools).
// Monaco is the classic macOS monospace fallback.
// Cascadia Mono/Code are included for users who have them from VS Code.
// Consolas is the Windows fallback.
// monospace is the universal last resort.
export const TERMINAL_FONT_FAMILY =
  "ui-monospace, 'SF Mono', Monaco, 'Cascadia Mono', 'Cascadia Code', Consolas, monospace"

export const TERMINAL_DOMAIN_ICONS: Record<string, string> = {
  default: '>',
  k8s: '\u2388',
  kafka: 'K',
  cloud: '\u2601',
}

export const TERMINAL_DOMAIN_LABELS: Record<string, string> = {
  default: 'Shell',
  k8s: 'K8s',
  kafka: 'Kafka',
  cloud: 'Cloud',
}
