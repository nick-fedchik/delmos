<!-- Ліва навігаційна панель оболонки (CORE_SHELL.md §3).
     Групи: система і контекст проєкту. Максимум два рівні вкладення,
     видимий активний маршрут, підказки для значків у згорнутому стані.
     Єдиний орган керування — шеврон унизу, видимий в обох станах. -->
<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'

import PIcon from '@/components/PIcon.vue'
import type { IconName } from '@/components/PIcon.vue'
import { useProjectContext } from '@/composables/useProjectContext'

defineProps<{ expanded: boolean; overlay: boolean }>()
const emit = defineEmits<{ (event: 'toggle'): void; (event: 'navigate'): void }>()

const { projectId, project } = useProjectContext()

interface NavItem {
  name: string
  label: string
  icon: IconName
}

const systemItems: NavItem[] = [
  { name: 'projects', label: 'Усі проєкти', icon: 'project' },
]

const projectItems: NavItem[] = [
  { name: 'project', label: 'Огляд проєкту', icon: 'overview' },
  { name: 'project-plan', label: 'План PLAN-001', icon: 'planning' },
  { name: 'project-stakeholders', label: 'Стейкхолдери (RACI)', icon: 'users' },
  { name: 'project-risks', label: 'Реєстр ризиків', icon: 'warning' },
]

const contextLabel = computed(() => {
  if (!projectId.value) return ''
  return project.value ? `${project.value.code} · ${project.value.name}` : 'Проєкт'
})
</script>

<template>
  <aside
    class="app-sidebar"
    :class="{ 'app-sidebar--collapsed': !expanded, 'app-sidebar--overlay': overlay }"
    aria-label="Основна навігація"
  >
    <nav class="sidebar-scroll">
      <div class="sidebar-section">
        <p v-if="expanded" class="sidebar-section-title">Система</p>
        <RouterLink
          v-for="item in systemItems"
          :key="item.name"
          :to="{ name: item.name }"
          class="sidebar-nav-item"
          active-class="active"
          :title="expanded ? undefined : item.label"
          @click="emit('navigate')"
        >
          <PIcon :name="item.icon" :size="16" />
          <span v-if="expanded" class="sidebar-label">{{ item.label }}</span>
        </RouterLink>
      </div>

      <div v-if="projectId" class="sidebar-section">
        <p v-if="expanded" class="sidebar-section-title" :title="contextLabel">
          {{ contextLabel }}
        </p>
        <RouterLink
          v-for="item in projectItems"
          :key="item.name"
          :to="{ name: item.name, params: { projectId } }"
          class="sidebar-nav-item"
          active-class="active"
          :title="expanded ? undefined : item.label"
          @click="emit('navigate')"
        >
          <PIcon :name="item.icon" :size="16" />
          <span v-if="expanded" class="sidebar-label">{{ item.label }}</span>
        </RouterLink>
      </div>
    </nav>

    <div class="panel-footer">
      <button
        type="button"
        class="panel-toggle-btn"
        :title="expanded ? 'Згорнути навігацію ([)' : 'Розгорнути навігацію ([)'"
        :aria-label="expanded ? 'Згорнути навігацію' : 'Розгорнути навігацію'"
        :aria-expanded="expanded"
        @click="emit('toggle')"
      >
        <PIcon :name="expanded ? 'chevron-double-lg-left' : 'chevron-double-lg-right'" :size="14" />
        <span v-if="expanded">Згорнути</span>
      </button>
    </div>
  </aside>
</template>
