<script setup lang="ts">
import { ref } from 'vue'

const beeName = ref('')
const suggestedName = ref('')

// GET https://api.neuralnexus.dev/api/v1/bee-name-generator/name
function getBeeName(): string {
    fetch(`https://api.neuralnexus.dev/api/v1/bee-name-generator/name`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' }
    })
        .then((response) => response.json())
        .then((data) => {
            beeName.value = data.name
        })
    return beeName.value
}

// TODO: Add the rest of the methods when auth is stored as a cookie
// Make it so the other routes are hidden behind the permissions

// POST https://api.neuralnexus.dev/api/v1/bee-name-generator/suggestion
function suggestBeeName(name: string): void {
    fetch(`https://api.neuralnexus.dev/api/v1/bee-name-generator/suggestion`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name })
    })
        .then((response) => response.json())
        .then((data) => {
            console.log(data)
        })
}
</script>

<template>
    <div class="beenamegenerator">
        <h1>BeeNameGenerator</h1>
    </div>
    <br />
    <div>
        <h2>Bee Name: {{ beeName }}</h2>
        <button @click="getBeeName()">Generate</button>
    </div>
    <br />
    <input type="text" v-model="suggestedName" />
    <button @click="suggestBeeName(suggestedName)">Suggest</button>
</template>
