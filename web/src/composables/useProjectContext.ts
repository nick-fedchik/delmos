// Активний проєктний контекст оболонки: назва й код замість UUID у навігації
// та breadcrumb (CORE_SHELL.md §2). Результат кешується, щоб перехід між
// сторінками проєкту не створював повторних запитів.
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { api } from '@/api/client'
import type { ProjectDetail } from '@/api/types'

const cache = new Map<string, { code: string; name: string }>()
const current = ref<{ id: string; code: string; name: string } | null>(null)
let pending = ''

export function useProjectContext() {
  const route = useRoute()

  const projectId = computed(() =>
    typeof route.params.projectId === 'string' ? route.params.projectId : null,
  )

  async function resolve(id: string): Promise<void> {
    const cached = cache.get(id)
    if (cached) {
      current.value = { id, ...cached }
      return
    }
    if (pending === id) {
      return
    }
    pending = id
    try {
      const detail = await api.get<ProjectDetail>(`/projects/${id}`)
      cache.set(id, { code: detail.code, name: detail.name })
      current.value = { id, code: detail.code, name: detail.name }
    } catch {
      // Назва недоступна (немає прав або зв'язку) — навігація лишається робочою
      // без підпису контексту, замість показу UUID як «назви».
      current.value = null
    } finally {
      pending = ''
    }
  }

  watch(
    projectId,
    (id) => {
      if (!id) {
        current.value = null
        return
      }
      if (current.value?.id !== id) {
        current.value = cache.get(id) ? { id, ...cache.get(id)! } : null
      }
      void resolve(id)
    },
    { immediate: true },
  )

  return {
    projectId,
    project: computed(() => (current.value?.id === projectId.value ? current.value : null)),
  }
}
