// Terminal feature manifest.
//
// The terminal is mounted as a global panel (a drawer at the bottom of
// the shell, toggled by the titlebar) rather than as a sidebar section.
// That maps to `panels`, not `section`. The shell mounts it via
// `<FeaturePanel id="terminal" :visible="terminalOpen" />` — no direct
// import of any terminal component.
//
// The `component` thunk is a dynamic import, so xterm and the terminal
// view code only load when the user first opens the panel.

/** @type {import('../registry').FeatureManifest} */
const manifest = {
  id: 'terminal',
  panels: [
    {
      id: 'terminal',
      component: () => import('./TerminalPanel.vue'),
    },
  ],
}

export default manifest
