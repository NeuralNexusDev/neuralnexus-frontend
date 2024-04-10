import { createMemoryHistory, createRouter } from 'vue-router'

export const routes = [
    {
        path: '/',
        name: 'Home',
        component: () => import('@/components/HomePage.vue')
    },
    {
        path: '/login',
        name: 'Login',
        component: () => import('@/components/LoginPage.vue')
    },
    {
        path: '/register',
        name: 'Register',
        component: () => import('@/components/RegisterPage')
    },
    // {
    //     path: '/forgot-password',
    //     name: 'ForgotPassword',
    //     component: () => import('@/components/ForgotPasswordPage.vue')
    // },
    {
        path: '/projects',
        name: 'Projects',
        component: () => import('@/components/ProjectsPage.vue')
    },
    {
        path: '/projects/bee-name-generator',
        name: 'BeeNameGenerator',
        component: () => import('@/components/BeeNameGenerator.vue')
    },
    {
        path: '/projects/mcstatus',
        name: 'McStatus',
        component: () => import('@/components/McStatus.vue')
    },
    {
        path: '/projects/mcstatus/:address',
        name: 'McServerStatus',
        component: () => import('@/components/McServerStatus.vue'),
        props: true
    }
]

export const router = createRouter({
    history: createMemoryHistory(),
    routes: routes
})
