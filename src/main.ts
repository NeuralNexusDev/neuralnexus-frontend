import './assets/main.css'

import { createApp } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import App from './App.vue'
import McStatus from './components/McStatus.vue'
import HomePage from './components/HomePage.vue'

const router = createRouter({
    history: createMemoryHistory(),
    routes: [
        { path: '/', component: HomePage },
        { path: '/mcstatus', component: McStatus }
    ]
})

createApp(App).use(router).mount('#app')
