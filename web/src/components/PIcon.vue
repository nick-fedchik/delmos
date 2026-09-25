<script setup lang="ts">
// Значок із спрайта GitLab Pajamas (@gitlab/svgs). Спрайт постачається разом із
// бінарником DELMOS (embedded SPA), тому значки доступні офлайн без CDN.
import { computed } from 'vue'

import spriteUrl from '@gitlab/svgs/dist/icons.svg?url'

export type IconName =
  | 'book'
  | 'check-circle'
  | 'chevron-double-lg-left'
  | 'chevron-double-lg-right'
  | 'clock'
  | 'collapse-left'
  | 'collapse-right'
  | 'copy-to-clipboard'
  | 'dashboard'
  | 'doc-text'
  | 'error'
  | 'group'
  | 'hamburger'
  | 'information-o'
  | 'issue-type-objective'
  | 'list-bulleted'
  | 'milestone'
  | 'overview'
  | 'planning'
  | 'project'
  | 'question-o'
  | 'search'
  | 'status-health'
  | 'users'
  | 'warning'

const props = withDefaults(
  defineProps<{
    name: IconName
    size?: number
    title?: string
  }>(),
  {
    size: 16,
    title: undefined,
  },
)

const href = computed(() => `${spriteUrl}#${props.name}`)
const pixels = computed(() => `${props.size}px`)
</script>

<template>
  <svg
    class="p-icon"
    :width="pixels"
    :height="pixels"
    :aria-hidden="title ? 'false' : 'true'"
    :role="title ? 'img' : undefined"
    fill="currentColor"
    focusable="false"
  >
    <title v-if="title">{{ title }}</title>
    <use :href="href" />
  </svg>
</template>

<style scoped>
.p-icon {
  display: inline-block;
  flex-shrink: 0;
  vertical-align: middle;
}
</style>
