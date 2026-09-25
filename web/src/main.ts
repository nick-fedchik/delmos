import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import './style.css'
import { setApiFailureReporter } from '@/api/client'
import { useDiagnosticsStore } from '@/stores/diagnostics'

const app = createApp(App)
app.use(createPinia())
app.use(router)

// Збої API потрапляють до нижнього статус-бара, щоб користувач бачив стан
// сторінки, а не лише зникле повідомлення у формі (CORE_SHELL.md §6).
const diagnostics = useDiagnosticsStore()
setApiFailureReporter((failure) => {
  const level = failure.status >= 500 || failure.status === 0 ? 'error' : 'warning'
  diagnostics.report(
    level,
    `${failure.method} ${failure.path} — ${failure.status || 'немає відповіді'}`,
    failure.message,
    failure.code,
  )
})

app.mount('#app')
