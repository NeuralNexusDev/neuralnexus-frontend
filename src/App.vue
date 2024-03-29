<script setup lang="ts"></script>

<template>
    <router-link to="/">Home</router-link>
    <router-link :to="{ name: 'McStatus' }">MC Status</router-link>
    <router-view></router-view>
</template>
