<!-- Довідка з підключення Git-сховища у стилі порожнього репозиторію GitLab/GitHub:
     готові команди, які можна скопіювати, замість опису словами. -->
<script setup lang="ts">
import { computed, ref } from 'vue'

import PIcon from '@/components/PIcon.vue'

const props = defineProps<{
  remoteUrl: string | null
  defaultBranch: string
  projectCode: string
}>()

const copiedId = ref('')

const branch = computed(() => props.defaultBranch || 'main')
const slug = computed(() => props.projectCode.toLowerCase())
const localBarePath = computed(() => `/var/lib/delmos/repos/${slug.value}.git`)

// Тека, яку створює `git clone`, збігається з іменем сховища без суфікса `.git`.
const workingDir = computed(() => {
  if (!props.remoteUrl) return slug.value
  const tail = props.remoteUrl.split(/[/:]/).pop() ?? ''
  return tail.replace(/\.git$/, '') || slug.value
})

const initBare = computed(
  () =>
    `git init --bare ${localBarePath.value}\n` +
    `git -C ${localBarePath.value} symbolic-ref HEAD refs/heads/${branch.value}`,
)

const remoteExamples = computed(
  () =>
    `https://gitlab.example.com/group/${slug.value}.git\n` +
    `git@gitlab.example.com:group/${slug.value}.git`,
)

const cloneCommands = computed(
  () => `git clone --branch ${branch.value} ${props.remoteUrl}\ncd ${workingDir.value}`,
)

const pushCommands = computed(() =>
  [
    `cd ${workingDir.value}`,
    `git init -b ${branch.value}`,
    `git remote add origin ${props.remoteUrl}`,
    'git add .',
    'git commit -m "Initial commit"',
    `git push -u origin ${branch.value}`,
  ].join('\n'),
)

async function copy(id: string, text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
    copiedId.value = id
  } catch {
    copiedId.value = ''
    return
  }
  setTimeout(() => {
    if (copiedId.value === id) copiedId.value = ''
  }, 2000)
}
</script>

<template>
  <div class="git-help">
    <!-- ── Сховище ще не прив'язане ─────────────────────────────────────── -->
    <template v-if="!remoteUrl">
      <p class="git-help-note">
        Прив'язка не є умовою роботи з артефактами. Робочі продукти й ревізії зберігаються в базі
        даних DELMOS; Git отримує їхні копії лише тоді, коли ви явно запускаєте експорт.
      </p>

      <section class="git-help-step">
        <h3 class="git-help-title">Варіант A · Локальне сховище на сервері DELMOS</h3>
        <p class="git-help-text">
          Створювати нічого не потрібно: під час прив'язки DELMOS сам виконає
          <code>git init --bare</code> за вказаним шляхом, якщо сховища там ще немає. Вкажіть у формі
          нижче шлях у файловій системі сервера:
        </p>
        <div class="git-help-cmd">
          <pre><code>{{ localBarePath }}</code></pre>
          <button
            type="button"
            class="git-help-copy"
            :title="'Скопіювати шлях'"
            @click="copy('local-path', localBarePath)"
          >
            <PIcon name="copy-to-clipboard" :size="14" />
            <span>{{ copiedId === 'local-path' ? 'Скопійовано' : 'Копіювати' }}</span>
          </button>
        </div>
        <p class="git-help-text">
          Якщо сховище має підготувати системний адміністратор заздалегідь — на сервері DELMOS:
        </p>
        <div class="git-help-cmd">
          <pre><code>{{ initBare }}</code></pre>
          <button
            type="button"
            class="git-help-copy"
            title="Скопіювати команди"
            @click="copy('local-init', initBare)"
          >
            <PIcon name="copy-to-clipboard" :size="14" />
            <span>{{ copiedId === 'local-init' ? 'Скопійовано' : 'Копіювати' }}</span>
          </button>
        </div>
      </section>

      <section class="git-help-step">
        <h3 class="git-help-title">Варіант B · Зовнішній Git (GitLab, GitHub, Gitea)</h3>
        <ol class="git-help-list">
          <li>
            Створіть <strong>порожнє</strong> сховище на своєму хостингу — без README, .gitignore та
            ліцензії, щоб перший коміт зробив DELMOS.
          </li>
          <li>Скопіюйте його HTTPS- або SSH-адресу й вставте у форму нижче.</li>
        </ol>
        <div class="git-help-cmd">
          <pre><code>{{ remoteExamples }}</code></pre>
        </div>
        <p class="git-help-note">
          DELMOS не зберігає ваші паролі й токени. Доступ до зовнішнього сховища сервіс отримує через
          обліковий запис ОС, під яким він працює: <code>credential helper</code> для HTTPS або ключ у
          SSH-агенті для SSH. Це налаштовує системний адміністратор на сервері DELMOS.
        </p>
      </section>
    </template>

    <!-- ── Сховище прив'язане ───────────────────────────────────────────── -->
    <template v-else>
      <section class="git-help-step">
        <h3 class="git-help-title">Клонувати сховище</h3>
        <div class="git-help-cmd">
          <pre><code>{{ cloneCommands }}</code></pre>
          <button
            type="button"
            class="git-help-copy"
            title="Скопіювати команди"
            @click="copy('clone', cloneCommands)"
          >
            <PIcon name="copy-to-clipboard" :size="14" />
            <span>{{ copiedId === 'clone' ? 'Скопійовано' : 'Копіювати' }}</span>
          </button>
        </div>
      </section>

      <section class="git-help-step">
        <h3 class="git-help-title">Надіслати наявну теку</h3>
        <div class="git-help-cmd">
          <pre><code>{{ pushCommands }}</code></pre>
          <button
            type="button"
            class="git-help-copy"
            title="Скопіювати команди"
            @click="copy('push', pushCommands)"
          >
            <PIcon name="copy-to-clipboard" :size="14" />
            <span>{{ copiedId === 'push' ? 'Скопійовано' : 'Копіювати' }}</span>
          </button>
        </div>
      </section>

      <p class="git-help-note">
        Експорт артефакту з DELMOS створює у корені сховища файл <code>&lt;КОД&gt;.md</code> і окремий
        коміт із повідомленням <code>Export &lt;КОД&gt; from DELMOS</code>. Автором коміта вказано ваш
        логін у DELMOS; адреса <code>&lt;логін&gt;@delmos.local</code> є службовою й не призначена для
        листування.
      </p>
    </template>
  </div>
</template>
