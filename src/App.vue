<script setup lang="ts"></script>

<template>
    <div class="p-8 flex items-center gap-4 mb-8 text-lg text-red-600">
        <router-link to="/">NeuralNexus</router-link>
        <router-link to="/about">About</router-link>
        <router-link to="/contact">Contact</router-link>
        <router-link to="/projects">Projects</router-link>
        <router-link
            class="ml-auto py-2 px-4 rounded-lg bg-red-600 text-black font-bold hover:bg-black hover:text-red-600"
            to="/login"
            >Login</router-link
        >
    </div>

    <div id="app" class="app">
        <router-view></router-view>
    </div>
</template>
