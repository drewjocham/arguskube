import { createApp } from 'vue'
import { createPinia } from 'pinia'
import naive from 'naive-ui'
import App from './App.vue'
import router from './router'
import { useContentStore } from './stores/content'

const app = createApp(App)
app.use(createPinia())
app.use(naive)
app.use(router)

const content = useContentStore()
content.fetchAll()

app.mount('#app')
