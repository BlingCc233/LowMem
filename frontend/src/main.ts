import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import Login from './views/Login.vue'
import ChatLayout from './components/ChatLayout.vue'
import './style.css'

const routes = [
    { path: '/', component: Login },
    { path: '/chat', component: ChatLayout }
]

const router = createRouter({
    history: createWebHashHistory(),
    routes,
})

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
app.use(router)
app.mount('#app')
