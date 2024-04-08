import { createMemoryHistory, createRouter } from 'vue-router'

export const routes = [
    {
        path: '/',
        name: 'Home',
        component: () => import('@/components/HomePage.vue')
    },
    {
        path: '/bee-name-generator',
        name: 'BeeNameGenerator',
        component: () => import('@/components/BeeNameGenerator.vue')
    },
    {
        path: '/mcstatus',
        name: 'McStatus',
        component: () => import('@/components/McStatus.vue')
    },
    {
        path: '/mcstatus/:address',
        name: 'McServerStatus',
        component: () => import('@/components/McServerStatus.vue'),
        props: true
    },
    {
        path: '/projects',
        name: 'Projects',
        component: () => import('@/components/ProjectsPage.vue')
    }
]

export const router = createRouter({
    history: createMemoryHistory(),
    routes: routes
})
