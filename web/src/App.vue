<script setup lang="ts">
import { RouterLink, RouterView, useRouter } from 'vue-router'

import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()

async function handleLogout(): Promise<void> {
  await session.logout()
  await router.push({ name: 'login' })
}
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <RouterLink :to="{ name: 'projects' }">DELMOS</RouterLink>
      <div v-if="session.isAuthenticated">
        <span class="muted">{{ session.user?.display_name || session.user?.login }}</span>
        <button class="btn-secondary" style="margin-left: 0.75rem" @click="handleLogout">Вийти</button>
      </div>
    </header>
    <main class="app-main">
      <RouterView />
    </main>
  </div>
</template>
