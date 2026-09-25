<!-- Довідка зі скорочень клавіатури (CORE_SHELL.md §7).
     Доступна з нижнього статус-бара і з клавіатури; інтерфейс лишається
     повністю керованим без скорочень. -->
<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'

import { SHORTCUTS, useShortcutsModal } from '@/composables/useShortcutsModal'

const { shortcutsOpen, closeShortcuts } = useShortcutsModal()

const dialogRef = ref<HTMLElement | null>(null)

watch(shortcutsOpen, async (open) => {
  if (!open) return
  await nextTick()
  dialogRef.value?.focus()
})
</script>

<template>
  <div v-if="shortcutsOpen" class="modal-backdrop" @click.self="closeShortcuts">
    <div
      ref="dialogRef"
      class="modal-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="shortcuts-title"
      tabindex="-1"
    >
      <div class="modal-header">
        <h2 id="shortcuts-title" class="modal-title">Скорочення клавіатури</h2>
        <button type="button" class="btn-secondary btn-sm" @click="closeShortcuts">Закрити</button>
      </div>
      <table class="data-table">
        <thead>
          <tr>
            <th scope="col">Клавіша</th>
            <th scope="col">Дія</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in SHORTCUTS" :key="entry.keys">
            <td><kbd>{{ entry.keys }}</kbd></td>
            <td>{{ entry.description }}</td>
          </tr>
        </tbody>
      </table>
      <p class="muted modal-note">
        Скорочення не спрацьовують у полях вводу. Усі дії доступні також через елементи інтерфейсу.
      </p>
    </div>
  </div>
</template>
