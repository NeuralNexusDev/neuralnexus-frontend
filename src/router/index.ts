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
        component: () => import('@/components/LoginPage.vue'),
        meta: {
            title: 'Login - NeuralNexus'
        }
    },
    {
        path: '/register',
        name: 'Register',
        component: () => import('@/components/RegisterPage'),
        meta: {
            title: 'Sign Up - NeuralNexus'
        }
    },
    // {
    //     path: '/forgot-password',
    //     name: 'ForgotPassword',
    //     component: () => import('@/components/ForgotPasswordPage.vue')
    // },
    {
        path: '/projects',
        name: 'Projects',
        component: () => import('@/components/ProjectsPage.vue'),
        meta: {
            title: 'Projects - NeuralNexus'
        }
    },
    {
        path: '/projects/bee-name-generator',
        name: 'BeeNameGenerator',
        component: () => import('@/components/BeeNameGenerator.vue'),
        meta: {
            title: 'Bee Name Generator - NeuralNexus'
        }
    },
    {
        path: '/projects/mcstatus',
        name: 'McStatus',
        component: () => import('@/components/McStatus.vue'),
        meta: {
            title: 'Minecraft Server Status - NeuralNexus'
        }
    },
    {
        path: '/projects/mcstatus/:address',
        name: 'McServerStatus',
        component: () => import('@/components/McServerStatus.vue'),
        props: true
    }
]
