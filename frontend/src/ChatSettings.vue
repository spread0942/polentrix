<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { fetchPromptPresets, type PromptPreset } from './api'

const props = defineProps<{
  title: string
  systemPrompt: string
  temperature: number | null
  topP: number | null
  numPredict: number | null
  defaultTemperature: number
  defaultSystemPrompt: string
}>()

const emit = defineEmits<{
  close: []
  save: [
    payload: {
      title: string
      systemPrompt: string
      temperature: number | null
      topP: number | null
      numPredict: number | null
    },
  ]
}>()

const titleDraft = ref(props.title)
const promptDraft = ref(props.systemPrompt)
const temperatureDraft = ref(props.temperature ?? props.defaultTemperature)
const topPDraft = ref(props.topP)
const numPredictDraft = ref(props.numPredict)
const useTopP = ref(props.topP != null)
const useNumPredict = ref(props.numPredict != null)
const presets = ref<PromptPreset[]>([])
const activePreset = ref<string | null>(null)

const activePromptPreview = computed(() => promptDraft.value.trim() || props.defaultSystemPrompt)

watch(
  () => props.title,
  (v) => {
    titleDraft.value = v
  },
)

watch(
  () => [props.systemPrompt, props.temperature, props.topP, props.numPredict] as const,
  ([sp, t, tp, np]) => {
    promptDraft.value = sp
    temperatureDraft.value = t ?? props.defaultTemperature
    topPDraft.value = tp
    numPredictDraft.value = np
    useTopP.value = tp != null
    useNumPredict.value = np != null
  },
)

function applyPreset(p: PromptPreset) {
  promptDraft.value = p.prompt
  activePreset.value = p.id
}

function save() {
  emit('save', {
    title: titleDraft.value.trim() || 'New chat',
    systemPrompt: promptDraft.value,
    temperature: temperatureDraft.value,
    topP: useTopP.value ? topPDraft.value : null,
    numPredict: useNumPredict.value ? numPredictDraft.value : null,
  })
}

onMounted(async () => {
  presets.value = await fetchPromptPresets()
})
</script>

<template>
  <aside class="panel" aria-label="Conversation settings">
    <header class="panel-head">
      <h2>Chat settings</h2>
      <button type="button" class="icon-btn" title="Close" @click="emit('close')">×</button>
    </header>

    <div class="panel-body">
      <label class="field">
        <span>Title</span>
        <input v-model="titleDraft" type="text" maxlength="120" />
      </label>

      <div class="field">
        <span>System prompt</span>
        <div v-if="presets.length" class="presets">
          <button
            v-for="p in presets"
            :key="p.id"
            type="button"
            class="preset"
            :class="{ active: activePreset === p.id }"
            @click="applyPreset(p)"
          >
            {{ p.name }}
          </button>
        </div>
        <textarea v-model="promptDraft" rows="5" placeholder="System instructions…" />
        <p class="hint">Active prompt sent to the model:</p>
        <pre class="preview">{{ activePromptPreview }}</pre>
      </div>

      <label class="field">
        <span>Temperature <em>{{ temperatureDraft.toFixed(2) }}</em></span>
        <input
          v-model.number="temperatureDraft"
          type="range"
          min="0"
          max="2"
          step="0.05"
        />
      </label>

      <label class="field check">
        <input v-model="useTopP" type="checkbox" />
        <span>Top P</span>
      </label>
      <label v-if="useTopP" class="field">
        <span>Top P <em>{{ (topPDraft ?? 0.9).toFixed(2) }}</em></span>
        <input
          v-model.number="topPDraft"
          type="range"
          min="0"
          max="1"
          step="0.05"
        />
      </label>

      <label class="field check">
        <input v-model="useNumPredict" type="checkbox" />
        <span>Max tokens (num_predict)</span>
      </label>
      <label v-if="useNumPredict" class="field">
        <span>Max tokens</span>
        <input
          v-model.number="numPredictDraft"
          type="number"
          min="16"
          max="8192"
          step="16"
        />
      </label>
    </div>

    <footer class="panel-foot">
      <button type="button" class="btn" @click="emit('close')">Cancel</button>
      <button type="button" class="btn primary" @click="save">Save</button>
    </footer>
  </aside>
</template>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  width: min(360px, 100%);
  border-left: 1px solid var(--border);
  background: var(--bg-sidebar);
  min-height: 0;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1rem 0.75rem;
  border-bottom: 1px solid var(--border);
}

.panel-head h2 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
}

.icon-btn {
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 1.35rem;
  line-height: 1;
  padding: 0.2rem 0.4rem;
}

.icon-btn:hover {
  color: var(--text);
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.field.check {
  flex-direction: row;
  align-items: center;
  gap: 0.5rem;
}

.field em {
  font-style: normal;
  color: var(--text);
  font-family: var(--mono);
  margin-left: 0.35rem;
}

.field input[type='text'],
.field input[type='number'],
.field textarea {
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.55rem 0.65rem;
  color: var(--text);
  outline: none;
}

.field input[type='range'] {
  width: 100%;
}

.field textarea {
  resize: vertical;
  min-height: 6rem;
  line-height: 1.45;
}

.presets {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.preset {
  border: 1px solid var(--border);
  background: var(--bg-elevated);
  color: var(--text-muted);
  border-radius: 6px;
  padding: 0.25rem 0.55rem;
  font-size: 0.75rem;
}

.preset:hover,
.preset.active {
  color: var(--text);
  border-color: var(--accent);
}

.hint {
  margin: 0;
  font-size: 0.72rem;
}

.preview {
  margin: 0;
  padding: 0.55rem 0.65rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.75rem;
  color: var(--text);
  font-family: var(--mono);
  max-height: 8rem;
  overflow-y: auto;
}

.panel-foot {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  padding: 0.75rem 1rem 1rem;
  border-top: 1px solid var(--border);
}

.btn {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.45rem 0.9rem;
  font-weight: 600;
  font-size: 0.85rem;
  background: var(--bg-elevated);
  color: var(--text);
}

.btn.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.btn.primary:hover {
  background: var(--accent-hover);
}
</style>
