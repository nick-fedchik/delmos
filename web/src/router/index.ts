import { createRouter, createWebHistory } from 'vue-router'

import { useSessionStore } from '@/stores/session'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    { path: '/', name: 'projects', component: () => import('@/views/ProjectListView.vue') },
    {
      path: '/projects/:projectId',
      name: 'project',
      component: () => import('@/views/ProjectDetailView.vue'),
      props: true,
    },
    {
      path: '/projects/:projectId/plan',
      name: 'project-plan',
      component: () => import('@/views/ProjectPlanView.vue'),
      props: true,
    },
    {
      path: '/projects/:projectId/stakeholders',
      name: 'project-stakeholders',
      component: () => import('@/views/ProjectStakeholdersView.vue'),
      props: true,
    },
    {
      path: '/projects/:projectId/risks',
      name: 'project-risks',
      component: () => import('@/views/ProjectRisksView.vue'),
      props: true,
    },
    {
      path: '/projects/:projectId/work-products/:workProductId',
      name: 'work-product',
      component: () => import('@/views/WorkProductDetailView.vue'),
      props: true,
    },
  ],
})

// Прямий URL на захищений маршрут без чинної сесії веде на /login (SWR-42 §1),
// а не показує порожню сторінку з непояснювальною помилкою 401.
router.beforeEach(async (to) => {
  const session = useSessionStore()
  if (!session.checked) {
    await session.restore()
  }
  if (!to.meta.public && !session.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && session.isAuthenticated) {
    return { name: 'projects' }
  }
  return true
})

export default router
