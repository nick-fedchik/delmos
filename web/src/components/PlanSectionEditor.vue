<script setup lang="ts">
type PlanItem = Record<string, unknown>
type Option = { value: string; label: string }
type Field = { key: string; label: string; kind: string; required?: boolean; optionsKey?: string }

defineProps<{
  sectionKey: string
  title: string
  singular: string
  items: PlanItem[]
  fields: Field[]
  options: Record<string, Option[]>
  editable: boolean
}>()

const emit = defineEmits<{ (event: 'add'): void; (event: 'remove', index: number): void }>()

function setValue(item: PlanItem, key: string, event: Event): void {
  item[key] = (event.target as HTMLInputElement).value
}

function setLines(item: PlanItem, key: string, event: Event): void {
  item[key] = (event.target as HTMLTextAreaElement).value.split('\n')
}

function valuesFor(item: PlanItem, key: string): string[] {
  const value = item[key]
  return Array.isArray(value) ? value as string[] : []
}

function toggleReference(item: PlanItem, key: string, value: string, event: Event): void {
  const selected = [...valuesFor(item, key)]
  item[key] = (event.target as HTMLInputElement).checked
    ? [...selected, value]
    : selected.filter((entry) => entry !== value)
}
</script>

<template>
  <section class="section-box">
    <div class="section-box-header">
      <h2 class="section-box-title">{{ title }}</h2>
      <button v-if="editable" type="button" class="btn-secondary btn-sm" @click="emit('add')">Додати {{ singular }}</button>
    </div>
    <div class="section-box-body plan-section-list">
      <p v-if="!items.length" class="muted">Поки немає записів.</p>
      <div v-for="(item, index) in items" :key="String(item.key)" class="plan-entry">
        <div class="plan-entry-header">
          <strong>{{ singular }} {{ index + 1 }}</strong>
          <span class="muted">{{ item.key }}</span>
          <button v-if="editable" type="button" class="btn-secondary btn-sm" :aria-label="`Видалити ${singular} ${index + 1}`" @click="emit('remove', index)">Видалити</button>
        </div>
        <div class="plan-entry-fields">
          <fieldset v-for="field in fields" :key="field.key" class="field plan-fieldset">
            <legend v-if="field.kind === 'multi'">{{ field.label }}</legend>
            <label v-else :for="`${sectionKey}-${item.key}-${field.key}`">{{ field.label }}</label>
            <template v-if="field.kind === 'multi'">
              <div class="plan-reference-options">
                <label
                  v-for="option in (options[field.optionsKey ?? ''] ?? []).filter((entry) => field.optionsKey !== 'phases' || sectionKey !== 'phases' || entry.value !== item.key)"
                  :key="option.value"
                  class="plan-checkbox"
                >
                  <input
                    type="checkbox"
                    :checked="valuesFor(item, field.key).includes(option.value)"
                    :disabled="!editable"
                    @change="toggleReference(item, field.key, option.value, $event)"
                  />
                  {{ option.label }}
                </label>
                <span v-if="!(options[field.optionsKey ?? ''] ?? []).length" class="muted">Немає доступних записів</span>
              </div>
            </template>
            <select
              v-else-if="field.kind === 'select'"
              :id="`${sectionKey}-${item.key}-${field.key}`"
              :value="String(item[field.key] ?? '')"
              :disabled="!editable"
              :required="field.required"
              @change="setValue(item, field.key, $event)"
            >
              <option value="">Не вибрано</option>
              <option
                v-if="item[field.key] && !(options[field.optionsKey ?? ''] ?? []).some((entry) => entry.value === item[field.key])"
                :value="String(item[field.key])"
              >{{ item[field.key] }}</option>
              <option v-for="option in options[field.optionsKey ?? ''] ?? []" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
            <textarea
              v-else-if="field.kind === 'textarea' || field.kind === 'lines'"
              :id="`${sectionKey}-${item.key}-${field.key}`"
              :value="field.kind === 'lines' ? valuesFor(item, field.key).join('\n') : String(item[field.key] ?? '')"
              :readonly="!editable"
              :required="field.required"
              rows="2"
              @input="field.kind === 'lines' ? setLines(item, field.key, $event) : setValue(item, field.key, $event)"
            ></textarea>
            <input
              v-else
              :id="`${sectionKey}-${item.key}-${field.key}`"
              :type="field.kind === 'date' ? 'date' : 'text'"
              :value="String(item[field.key] ?? '')"
              :readonly="!editable"
              :required="field.required"
              @input="setValue(item, field.key, $event)"
            />
            <span v-if="field.kind === 'lines'" class="field-hint muted">Один пункт на рядок</span>
          </fieldset>
        </div>
      </div>
    </div>
  </section>
</template>