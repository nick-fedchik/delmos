<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, nextTick, ref } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: string
  rows?: number
}>(), { rows: 16 })

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const editorMode = ref<'write' | 'preview'>('write')
const editor = ref<HTMLTextAreaElement | null>(null)

const renderedMarkdown = computed(() => DOMPurify.sanitize(marked.parse(props.modelValue) as string))
const markdownPlugins = [
  { label: 'B', title: 'Жирний', before: '**', after: '**', placeholder: 'текст' },
  { label: 'I', title: 'Курсив', before: '*', after: '*', placeholder: 'текст' },
  { label: 'H2', title: 'Заголовок', before: '## ', after: '', placeholder: 'заголовок' },
  { label: '[]', title: 'Список завдань', before: '- [ ] ', after: '', placeholder: 'завдання' },
  { label: '↗', title: 'Посилання', before: '[', after: '](https://)', placeholder: 'назва' },
] as const

async function applyPlugin(plugin: (typeof markdownPlugins)[number]): Promise<void> {
  await nextTick()
  const target = editor.value
  if (!target) return
  const start = target.selectionStart
  const end = target.selectionEnd
  const selected = props.modelValue.slice(start, end) || plugin.placeholder
  const value = props.modelValue.slice(0, start) + plugin.before + selected + plugin.after + props.modelValue.slice(end)
  emit('update:modelValue', value)
  await nextTick()
  target.focus()
  const cursor = start + plugin.before.length + selected.length + plugin.after.length
  target.setSelectionRange(cursor, cursor)
}
</script>

<template>
  <div class="markdown-editor-shell">
    <div class="editor-toolbar" aria-label="Markdown plugins">
      <button
        v-for="plugin in markdownPlugins"
        :key="plugin.title"
        class="tool-button"
        type="button"
        :title="plugin.title"
        :aria-label="plugin.title"
        @click="applyPlugin(plugin)"
      >{{ plugin.label }}</button>
      <span class="toolbar-spacer"></span>
      <button class="tool-button" type="button" :class="{ active: editorMode === 'write' }" @click="editorMode = 'write'">Редактор</button>
      <button class="tool-button" type="button" :class="{ active: editorMode === 'preview' }" @click="editorMode = 'preview'">Preview</button>
    </div>
    <textarea
      v-if="editorMode === 'write'"
      ref="editor"
      :value="modelValue"
      class="markdown-editor"
      :rows="rows"
      spellcheck="true"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
    ></textarea>
    <article v-else class="markdown-preview" v-html="renderedMarkdown"></article>
  </div>
</template>
