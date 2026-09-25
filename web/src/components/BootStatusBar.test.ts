import { createApp, nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import BootStatusBar from '@/components/BootStatusBar.vue'

describe('BootStatusBar', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('renders components and updates with boot health from server', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          status: 'green',
          version: '1.0.6-dev',
          components: [
            { id: 'core', name: 'Ядро DELMOS', status: 'green', message: 'Активне' },
            { id: 'database', name: 'База даних (PostgreSQL)', status: 'green', message: 'Підключено' },
            { id: 'schema', name: 'Схема даних', status: 'green', message: 'Актуальна' },
            { id: 'git_storage', name: 'Сховище репозиторіїв', status: 'green', message: 'Plain Git готовий' },
          ],
        }),
      }),
    )

    const container = document.createElement('div')
    document.body.appendChild(container)
    const app = createApp(BootStatusBar)
    app.mount(container)

    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 20))

    expect(container.textContent).toContain('Boot Health')
    expect(container.textContent).toContain('Ядро DELMOS')
    expect(container.textContent).toContain('База даних (PostgreSQL)')
    expect(container.textContent).toContain('Схема даних')
    expect(container.textContent).toContain('Сховище репозиторіїв')
    expect(container.querySelectorAll('.status-green').length).toBeGreaterThanOrEqual(4)

    app.unmount()
  })

  it('marks components red when server is unreachable', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Network error')))

    const container = document.createElement('div')
    document.body.appendChild(container)
    const app = createApp(BootStatusBar)
    app.mount(container)

    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 20))

    expect(container.textContent).toContain('Немає з’єднання з сервером DELMOS')
    expect(container.querySelectorAll('.status-red').length).toBeGreaterThanOrEqual(4)

    app.unmount()
  })
})
