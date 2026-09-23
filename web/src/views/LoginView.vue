<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/client'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()
const route = useRoute()

const login = ref('')
const password = ref('')
const error = ref('')
const submitting = ref(false)

async function handleSubmit(): Promise<void> {
  error.value = ''
  submitting.value = true
  try {
    await session.login(login.value, password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (err) {
    // SWR-44 §2: сервер навмисно не розрізняє невідомий логін і невірний пароль.
    error.value = err instanceof ApiError ? err.message : 'Не вдалося увійти'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="card" style="max-width: 360px; margin: 3rem auto">
    <h1>Вхід до DELMOS</h1>
    <p v-if="error" class="alert-error" role="alert">{{ error }}</p>
    <form @submit.prevent="handleSubmit">
      <div class="field">
        <label for="login">Логін</label>
        <input id="login" v-model="login" type="text" autocomplete="username" required autofocus />
      </div>
      <div class="field">
        <label for="password">Пароль</label>
        <input id="password" v-model="password" type="password" autocomplete="current-password" required />
      </div>
      <button class="btn-primary" type="submit" :disabled="submitting">Увійти</button>
    </form>
  </div>
</template>
