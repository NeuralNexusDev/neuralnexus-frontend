import './assets/index.css'

import { ViteSSG } from 'vite-ssg'
import * as cookies from 'vue-cookies'

import App from './App.vue'
import { routes } from './router'

export const createApp: any = ViteSSG(App, { routes }, (ctx) => {
    ctx.app.use(cookies)
})
