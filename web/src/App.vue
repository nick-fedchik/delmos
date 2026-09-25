<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'

import AppLeftSidebar from '@/components/AppLeftSidebar.vue'
import AppRightSidebar from '@/components/AppRightSidebar.vue'
import AppStatusBar from '@/components/AppStatusBar.vue'
import KeyboardShortcutsModal from '@/components/KeyboardShortcutsModal.vue'
import { useLayout } from '@/composables/useLayout'
import { useShortcutsModal } from '@/composables/useShortcutsModal'
import { useSystemHealth } from '@/composables/useSystemHealth'
import { useSessionStore } from '@/stores/session'

const NARROW_BREAKPOINT_PX = 1024

const appVersion = import.meta.env.VITE_DELMOS_VERSION

const session = useSessionStore()
const router = useRouter()
const route = useRoute()

const { leftExpanded, rightExpanded, toggleLeft, toggleRight, openRight, closeRight } = useLayout()
const { shortcutsOpen, toggleShortcuts, closeShortcuts } = useShortcutsModal()
const { status, summary } = useSystemHealth()

const isLoginPage = computed(() => route.name === 'login')
const showShell = computed(() => session.isAuthenticated && !isLoginPage.value)

// Вузький viewport: панелі стають накладними, щоб вміст сторінки не звужувався
// під ними (CORE_SHELL.md §3, §5).
const narrow = ref(false)
const leftOverlayOpen = ref(false)

function syncViewport(): void {
  narrow.value = window.innerWidth < NARROW_BREAKPOINT_PX
  if (!narrow.value) {
    leftOverlayOpen.value = false
  }
}

const leftIsOpen = computed(() => (narrow.value ? leftOverlayOpen.value : leftExpanded.value))
const rightOverlayVisible = computed(() => narrow.value && rightExpanded.value)
const overlayActive = computed(() => leftOverlayOpen.value || rightOverlayVisible.value)

const userInitials = computed(() => {
  const name = session.user?.display_name || session.user?.login || 'U'
  return name.slice(0, 2).toUpperCase()
})

const roleLabel = computed(() => {
  const admin = session.hasPermission('access.grant')
  const manager = session.hasPermission('scopes.manage')
  if (admin && manager) return 'Адміністратор · Керівник проєкту'
  if (admin) return 'Адміністратор системи'
  if (manager) return 'Керівник проєкту'
  return 'Користувач'
})

function toggleLeftPanel(): void {
  if (narrow.value) {
    leftOverlayOpen.value = !leftOverlayOpen.value
    return
  }
  toggleLeft()
}

function closeOverlays(): void {
  leftOverlayOpen.value = false
  if (rightOverlayVisible.value) {
    closeRight()
  }
}

// Скорочення реєструються централізовано і не працюють у полях вводу та в
// contenteditable (CORE_SHELL.md §7).
function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    if (shortcutsOpen.value) {
      closeShortcuts()
      return
    }
    closeOverlays()
    return
  }

  if (event.ctrlKey || event.metaKey || event.altKey || event.isComposing) return

  const target = event.target as HTMLElement | null
  const tag = target?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target?.isContentEditable) return
  if (!showShell.value) return

  if (event.key === '[') {
    event.preventDefault()
    toggleLeftPanel()
  } else if (event.key === ']') {
    event.preventDefault()
    toggleRight()
  } else if (event.key === '?') {
    event.preventDefault()
    toggleShortcuts()
  }
}

// Перехід маршруту переміщує фокус до заголовка основного вмісту (CORE_SHELL.md §4).
watch(
  () => route.fullPath,
  () => {
    leftOverlayOpen.value = false
    requestAnimationFrame(() => {
      document.getElementById('main-content')?.focus()
    })
  },
)

async function handleLogout(): Promise<void> {
  await session.logout()
  await router.push({ name: 'login' })
}

onMounted(() => {
  syncViewport()
  window.addEventListener('resize', syncViewport)
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  window.removeEventListener('resize', syncViewport)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="app-shell" :class="{ 'app-shell--plain': !showShell }">
    <a class="skip-link" href="#main-content">Перейти до вмісту</a>

    <header class="app-header">
      <div class="app-header-left">
        <div class="app-brand">
          <RouterLink :to="{ name: 'projects' }" class="brand-link">DELMOS</RouterLink>
          <span class="app-version">{{ appVersion }}</span>
        </div>
      </div>

      <!-- Верхній бар показує лише агрегований збій чи деградацію, а не
           постійний індикатор справності (CORE_SHELL.md §2). -->
      <div class="app-header-center">
        <button
          v-if="showShell && status !== 'green'"
          type="button"
          class="header-alert"
          :class="'header-alert--' + status"
          title="Відкрити діагностику компонентів системи"
          @click="openRight('diagnostics')"
        >
          <span class="status-circle" :class="'status-' + status"></span>
          <span>{{ summary }}</span>
        </button>
      </div>

      <div v-if="session.isAuthenticated" class="app-header-right">
        <div class="user-profile">
          <span class="user-avatar" :title="session.user?.display_name || session.user?.login">
            {{ userInitials }}
          </span>
          <div class="user-meta">
            <span class="user-name">{{ session.user?.display_name || session.user?.login }}</span>
            <span class="user-role">{{ roleLabel }}</span>
          </div>
        </div>
        <button class="btn-secondary btn-sm" type="button" @click="handleLogout">Вийти</button>
      </div>
    </header>

    <div class="app-body">
      <div v-if="showShell" class="panel-slot" :class="{ 'panel-slot--narrow': narrow }">
        <AppLeftSidebar
          :expanded="leftIsOpen"
          :overlay="narrow && leftOverlayOpen"
          @toggle="toggleLeftPanel"
          @navigate="leftOverlayOpen = false"
        />
      </div>

      <main id="main-content" class="app-main" tabindex="-1">
        <RouterView />
      </main>

      <div
        v-if="showShell"
        class="panel-slot"
        :class="{ 'panel-slot--narrow-right': rightOverlayVisible }"
      >
        <AppRightSidebar :overlay="narrow" />
      </div>

      <div v-if="overlayActive" class="shell-scrim" @click="closeOverlays"></div>
    </div>

    <AppStatusBar v-if="showShell" />
    <KeyboardShortcutsModal v-if="showShell" />
  </div>
</template>
