import { createRouter, createWebHistory } from 'vue-router'
import HomePage from './views/HomePage.vue'
import LoginView from './views/LoginView.vue'
import RegisterView from './views/RegisterView.vue'
import PricingView from './views/PricingView.vue'

const routes = [
  { path: '/', name: 'home', component: HomePage },
  { path: '/login', name: 'login', component: LoginView },
  { path: '/register', name: 'register', component: RegisterView },
  { path: '/pricing', name: 'pricing', component: PricingView },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
