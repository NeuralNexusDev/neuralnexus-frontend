<script setup lang="ts">
import { sessionStore } from './composables/store'

const store = sessionStore()

function revokeSession() {
    fetch('https://api.neuralnexus.dev/api/v1/auth/logout', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${store.getSession().session_id}`
        }
    })
        .then((res) => {
            if (res.ok) {
                store.deleteSession()
                window.location.href = '/'
            }
        })
        .catch((error) => {
            console.error('Error:', error)
        })
}
</script>

<template>
    <div class="p-8 flex items-center gap-4 mb-8 text-lg text-red-600">
        <router-link to="/">NeuralNexus</router-link>
        <router-link to="/about">About</router-link>
        <router-link to="/contact">Contact</router-link>
        <router-link to="/projects">Projects</router-link>

        <router-link
            v-if="store.getSession().session_id === ''"
            class="ml-auto py-2 px-4 rounded-lg bg-red-600 text-black font-bold hover:bg-black hover:text-red-600"
            to="/login"
            >Login</router-link
        >
        <button
            v-else
            class="ml-auto py-2 px-4 rounded-lg bg-red-600 text-black font-bold hover:bg-black hover:text-red-600"
            @click="revokeSession()"
        >
            Logout
        </button>
    </div>

    <div id="app" class="app">
        <router-view></router-view>
    </div>
</template>
