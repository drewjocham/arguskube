import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './assets/theme.css'
// xterm ships its own stylesheet that hides the helper textarea xterm uses
// for keyboard input and styles the .xterm-screen / .xterm-viewport / cursor.
// Without this import the helper textarea renders as a visible white box and
// keystrokes never reach the PTY (the terminal looks "broken").
import 'xterm/css/xterm.css'

const app = createApp(App)
app.use(createPinia())
app.mount('#app')
