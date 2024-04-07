<script setup lang="ts"></script>

<template>
    <div class="header">
        <router-link to="/">NeuralNexus</router-link>
        <router-link to="/about">About</router-link>
        <router-link to="/contact">Contact</router-link>
        <router-link to="/projects">Projects</router-link>
        <router-link class="login-button" to="/login">Login</router-link>
    </div>

    <div id="app" class="app">
        <br />

        <router-link :to="{ name: 'McStatus' }">MC Status</router-link>
        <router-view></router-view>
    </div>
</template>
