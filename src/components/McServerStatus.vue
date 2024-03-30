<script setup lang="ts">
import { autoToHTML } from '@sfirew/minecraft-motd-parser'

const props = defineProps<{
    address: string
}>()

const ONLINE_PLAYERS = 0
const MAX_PLAYERS = 0
const FAVICON = ''

fetch(`https://api.neuralnexus.dev/api/v1/mcstatus/${props.address}`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' }
})
    .then((response) => response.json())
    .then((data) => {
        console.log(data)
        const motd = document.getElementById('motd')
        if (motd) {
            motd.innerHTML = autoToHTML(data.name)
        }
        document.getElementById
    })
</script>

<template>
    <div class="mcserverstatus">
        <h1>McServerStatus</h1>
    </div>
    <br />
    <div>
        <h2>Server Address: {{ props.address }}</h2>
    </div>
    <div style="text-align: center" id="motd"></div>
    <br />
    <img class="center" src="" alt="Server Favicon" id="favicon" />
    <br />
    <p style="text-align: center">Players: {{ ONLINE_PLAYERS }}/{{ MAX_PLAYERS }}</p>
</template>
