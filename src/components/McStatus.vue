<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
const address = ref('')

const router = useRouter()
function useRouterCustom() {
    router.push({ name: 'McServerStatus', params: { address: address.value } })
}
</script>

<template>
    <div class="mcstatus">
        <h1>MC Status</h1>
    </div>
    <br />
    <form @submit.prevent="useRouterCustom()">
        <input type="text" v-model="address" placeholder="Server Address" />
        <button type="submit">Check</button>
    </form>
</template>
