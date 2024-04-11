import './assets/index.css'
import App from './App.vue'

import { ViteSSG } from 'vite-ssg'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'

export const createApp: any = ViteSSG(
    App,
    {
        routes: [
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
    },
    async (ctx) => {
        const pinia = createPinia()
        pinia.use(piniaPluginPersistedstate)
        ctx.app.use(pinia)

        if (import.meta.env.SSR) ctx.initialState.pinia = pinia.state.value
        else pinia.state.value = ctx.initialState.pinia || {}

        // ctx.router.afterEach(() => {
        //     const title = <string>ctx.router.currentRoute.value.meta.title
        //     if (title) {
        //         document.title = title
        //     }
        // })
    }
)
