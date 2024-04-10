import './assets/index.css'

import { ViteSSG } from 'vite-ssg'
import * as cookies from 'vue-cookies'

import App from './App.vue'
import { routes } from './router'

export const createApp: any = ViteSSG(App, { routes }, (ctx) => {
    ctx.app.use(cookies)

    // Create afterEach for router
    ctx.router.afterEach(() => {
        const title = <string>ctx.router.currentRoute.value.meta.title
        if (title) {
            document.title = title
        } else {
            document.title = 'NeuralNexus'
        }
    })
})
