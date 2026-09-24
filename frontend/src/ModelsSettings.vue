<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { fetchModels, pullModel, type CatalogModel, type PullProgress } from './api'

const props = defineProps<{
  selectedModel: string
}>()

const emit = defineEmits<{
  back: []
  select: [name: string]
}>()

const models = ref<CatalogModel[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const customName = ref('')
const pulling = ref<string | null>(null)
const pullStatus = ref('')
const pullPct = ref<number | null>(null)
let pullAbort: AbortController | null = null

const sortedModels = computed(() =>
  [...models.value].sort((a, b) => {
    if (a.downloaded !== b.downloaded) return a.downloaded ? -1 : 1
    return a.name.localeCompare(b.name)
  }),
)

function formatSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return ''
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) return `${gb.toFixed(1)} GB`
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(0)} MB`
}

function progressFrom(p: PullProgress): void {
  pullStatus.value = p.status ?? ''
  if (p.total && p.total > 0 && typeof p.completed === 'number') {
    pullPct.value = Math.min(100, Math.round((p.completed / p.total) * 100))
  }
}

async function load() {
  loading.value = true
  error.value = null
  const res = await fetchModels()
  loading.value = false
  if (!res) {
    error.value = 'Could not load models from Ollama'
    return
  }
  models.value = res.models
}

async function startPull(name: string) {
  const trimmed = name.trim()
  if (!trimmed || pulling.value) return

  error.value = null
  pulling.value = trimmed
  pullStatus.value = 'Starting…'
  pullPct.value = null
  pullAbort = new AbortController()

  try {
    await pullModel(trimmed, progressFrom, pullAbort.signal)
    await load()
    emit('select', trimmed)
  } catch (e) {
    if ((e as Error).name !== 'AbortError') {
      error.value = e instanceof Error ? e.message : 'Pull failed'
    }
  } finally {
    pulling.value = null
    pullStatus.value = ''
    pullPct.value = null
    pullAbort = null
  }
}

function cancelPull() {
  pullAbort?.abort()
}

function useModel(name: string) {
  if (pulling.value) return
  emit('select', name)
}

function pullCustom() {
  void startPull(customName.value)
}

onMounted(() => {
  void load()
})

onUnmounted(() => {
  pullAbort?.abort()
})
</script>

<template>
  <div class="models-view">
    <header class="models-header">
      <button type="button" class="btn ghost" @click="emit('back')">← Back</button>
      <div class="models-title">
        <h1>Models</h1>
        <p>Pull from Ollama and choose which model chats use.</p>
      </div>
    </header>

    <div class="models-body">
      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="loading" class="muted">Loading models…</p>

      <ul v-else class="model-list" aria-label="Available models">
        <li
          v-for="m in sortedModels"
          :key="m.name"
          class="model-row"
          :class="{
            downloaded: m.downloaded,
            remote: !m.downloaded,
            active: m.name === selectedModel,
            pulling: pulling === m.name,
          }"
        >
          <div class="model-info">
            <span class="model-name">{{ m.name }}</span>
            <span class="model-meta">
              <template v-if="m.downloaded">
                Installed{{ formatSize(m.size) ? ` · ${formatSize(m.size)}` : '' }}
              </template>
              <template v-else>Not downloaded</template>
            </span>
            <div v-if="pulling === m.name" class="pull-progress">
              <div class="pull-bar" :style="{ width: `${pullPct ?? 8}%` }" />
              <span class="pull-label">
                {{ pullStatus || 'Pulling…' }}
                <template v-if="pullPct != null"> · {{ pullPct }}%</template>
              </span>
            </div>
          </div>
          <div class="model-actions">
            <button
              v-if="pulling === m.name"
              type="button"
              class="btn danger"
              @click="cancelPull"
            >
              Cancel
            </button>
            <template v-else-if="m.downloaded">
              <button
                type="button"
                class="btn primary"
                :disabled="!!pulling || m.name === selectedModel"
                @click="useModel(m.name)"
              >
                {{ m.name === selectedModel ? 'In use' : 'Use' }}
              </button>
            </template>
            <button
              v-else
              type="button"
              class="btn"
              :disabled="!!pulling"
              @click="startPull(m.name)"
            >
              Pull
            </button>
          </div>
        </li>
      </ul>

      <form class="custom-pull" @submit.prevent="pullCustom">
        <label for="custom-model">Pull another tag</label>
        <div class="custom-row">
          <input
            id="custom-model"
            v-model="customName"
            type="text"
            placeholder="e.g. qwen2.5:3b"
            :disabled="!!pulling"
            autocomplete="off"
            spellcheck="false"
          />
          <button
            type="submit"
            class="btn primary"
            :disabled="!!pulling || !customName.trim()"
          >
            Pull
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.models-view {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  background:
    radial-gradient(ellipse 80% 50% at 50% -20%, rgba(61, 143, 90, 0.12), transparent),
    var(--bg);
}

.models-header {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--border);
}

.models-title h1 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
}

.models-title p {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.models-body {
  flex: 1;
  overflow-y: auto;
  padding: 1.25rem 1.5rem 2rem;
  max-width: 40rem;
  width: 100%;
  margin: 0 auto;
}

.muted {
  color: var(--text-muted);
}

.error {
  color: var(--danger);
  font-size: 0.85rem;
  margin: 0 0 1rem;
}

.model-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.model-row {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.85rem 1rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  transition: opacity 0.2s ease, filter 0.2s ease, border-color 0.2s ease;
}

.model-row.downloaded {
  opacity: 1;
  border-color: #3a5a42;
}

.model-row.downloaded .model-name {
  color: var(--text);
  font-weight: 600;
}

.model-row.remote {
  opacity: 0.42;
  filter: saturate(0.55) brightness(0.85);
}

.model-row.remote:hover {
  opacity: 0.7;
  filter: saturate(0.75) brightness(0.95);
}

.model-row.active {
  border-color: var(--accent);
  box-shadow: inset 0 0 0 1px rgba(61, 143, 90, 0.35);
}

.model-row.pulling {
  opacity: 1;
  filter: none;
}

.model-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.model-name {
  font-family: var(--mono);
  font-size: 0.9rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-meta {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.pull-progress {
  margin-top: 0.45rem;
  height: 1.35rem;
  position: relative;
  background: rgba(0, 0, 0, 0.25);
  border-radius: 4px;
  overflow: hidden;
}

.pull-bar {
  position: absolute;
  inset: 0 auto 0 0;
  background: var(--accent);
  opacity: 0.55;
  transition: width 0.2s ease;
  min-width: 4px;
}

.pull-label {
  position: relative;
  z-index: 1;
  display: block;
  padding: 0.15rem 0.45rem;
  font-size: 0.7rem;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.model-actions {
  flex-shrink: 0;
}

.custom-pull {
  margin-top: 1.75rem;
  padding-top: 1.25rem;
  border-top: 1px solid var(--border);
}

.custom-pull label {
  display: block;
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-bottom: 0.5rem;
}

.custom-row {
  display: flex;
  gap: 0.65rem;
}

.custom-row input {
  flex: 1;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.55rem 0.75rem;
  outline: none;
  font-family: var(--mono);
  font-size: 0.9rem;
}

.custom-row input:focus {
  border-color: var(--accent);
}

.btn {
  border: none;
  border-radius: 8px;
  padding: 0.5rem 0.9rem;
  font-weight: 600;
  font-size: 0.85rem;
  white-space: nowrap;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  color: var(--text);
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.btn.primary {
  background: var(--accent);
  border-color: transparent;
  color: #fff;
}

.btn.primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn.danger {
  background: var(--danger);
  border-color: transparent;
  color: #fff;
}

.btn.ghost {
  background: transparent;
  border-color: transparent;
  color: var(--text-muted);
  padding-left: 0.25rem;
}

.btn.ghost:hover {
  color: var(--text);
}
</style>
