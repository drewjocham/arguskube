import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './assets/theme.css'
// xterm ships its own stylesheet that hides the helper textarea xterm uses
// for keyboard input and styles the .xterm-screen / .xterm-viewport / cursor.
// Without this import the helper textarea renders as a visible white box and
// keystrokes never reach the PTY (the terminal looks "broken").
import 'xterm/css/xterm.css'
import { useAppearanceStore } from './stores/appearance'
// Side-effect import: auto-discovers src/features/*/manifest.js and
// registers each feature with the registry. The shell consumes the
// registry via <FeaturePanel> / listSections() — never via direct
// import of feature components.
import './features'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
// Eagerly construct the appearance store so its persisted theme +
// slider values land on <html> before the first paint. Without this,
// the user gets a one-frame flash of the default look on every reload.
useAppearanceStore(pinia)
app.mount('#app')
