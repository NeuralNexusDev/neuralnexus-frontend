import { createMemoryHistory, createRouter } from 'vue-router'

import McStatus from '../components/McStatus.vue'
import HomePage from '../components/HomePage.vue'
import McServerStatus from '@/components/McServerStatus.vue'

export const routes = [
    { path: '/', name: 'Home', component: HomePage },
    { path: '/mcstatus', name: 'McStatus', component: McStatus },
    {
        path: '/mcstatus/:address',
        name: 'McServerStatus',
        component: McServerStatus,
        props: true
    }
]

export const router = createRouter({
    history: createMemoryHistory(),
    routes: routes
})
