import { createMemoryHistory, createRouter } from 'vue-router'

import McStatus from '../components/McStatus.vue'
import HomePage from '../components/HomePage.vue'

const routes = [
    { path: '/', name: 'Home', component: HomePage },
    { path: '/mcstatus', name: 'McStatus', component: McStatus }
]

const router = createRouter({
    history: createMemoryHistory(),
    routes: routes
})

export default router
