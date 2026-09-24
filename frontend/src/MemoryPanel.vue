<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  createMemory,
  deleteMemory,
  listMemories,
  patchMemory,
  type ApiMemory,
} from './api'
import { uid } from './types'

const props = defineProps<{
  conversationId: string
  contextSummary?: string
}>()

const emit = defineEmits<{
  close: []
}>()

const memories = ref<ApiMemory[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const draft = ref('')
const scope = ref<'user' | 'conversation'>('conversation')
const editingId = ref<string | null>(null)
const editDraft = ref('')

const userMemories = computed(() => memories.value.filter((m) => !m.conversation_id))
const chatMemories = computed(() =>
  memories.value.filter((m) => m.conversation_id === props.conversationId),
)

async function refresh() {
  loading.value = true
  error.value = null
  try {
    memories.value = await listMemories({
      conversationId: props.conversationId,
      scope: 'all',
    })
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load memories'
  } finally {
    loading.value = false
  }
}

async function addMemory() {
  const content = draft.value.trim()
  if (!content) return
  error.value = null
  try {
    await createMemory({
      id: uid(),
      content,
      conversation_id: scope.value === 'conversation' ? props.conversationId : null,
    })
    draft.value = ''
    await refresh()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to add memory'
  }
}

function startEdit(m: ApiMemory) {
  editingId.value = m.id
  editDraft.value = m.content
}

function cancelEdit() {
  editingId.value = null
  editDraft.value = ''
}

async function saveEdit() {
  const id = editingId.value
  if (!id) return
  const content = editDraft.value.trim()
  if (!content) return
  error.value = null
  try {
    await patchMemory(id, content)
    cancelEdit()
    await refresh()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to update memory'
  }
}

async function remove(id: string) {
  error.value = null
  try {
    await deleteMemory(id)
    if (editingId.value === id) cancelEdit()
    await refresh()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to delete memory'
  }
}

watch(
  () => props.conversationId,
  () => {
    void refresh()
  },
)

onMounted(() => {
  void refresh()
})
</script>

<template>
  <aside class="panel" aria-label="Memories">
    <header class="panel-head">
      <h2>Memories</h2>
      <button type="button" class="icon-btn" title="Close" @click="emit('close')">×</button>
    </header>

    <div class="panel-body">
      <p class="intro">
        Facts kept beyond a single turn. User memories apply everywhere; chat memories stay with this conversation.
      </p>

      <div v-if="contextSummary" class="summary">
        <span class="label">Short-term summary</span>
        <pre>{{ contextSummary }}</pre>
      </div>

      <form class="add" @submit.prevent="addMemory">
        <textarea v-model="draft" rows="2" placeholder="Add a memory…" />
        <div class="add-row">
          <label class="scope">
            <input v-model="scope" type="radio" value="conversation" />
            This chat
          </label>
          <label class="scope">
            <input v-model="scope" type="radio" value="user" />
            User
          </label>
          <button type="submit" class="btn primary" :disabled="!draft.trim()">Add</button>
        </div>
      </form>

      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="loading" class="muted">Loading…</p>

      <section class="group">
        <h3>This chat</h3>
        <p v-if="!chatMemories.length" class="muted">No conversation memories yet.</p>
        <ul v-else class="list">
          <li v-for="m in chatMemories" :key="m.id">
            <template v-if="editingId === m.id">
              <textarea v-model="editDraft" rows="2" />
              <div class="row-actions">
                <button type="button" class="btn primary" @click="saveEdit">Save</button>
                <button type="button" class="btn" @click="cancelEdit">Cancel</button>
              </div>
            </template>
            <template v-else>
              <p class="content">{{ m.content }}</p>
              <div class="row-actions">
                <button type="button" class="link" @click="startEdit(m)">Edit</button>
                <button type="button" class="link danger" @click="remove(m.id)">Delete</button>
              </div>
            </template>
          </li>
        </ul>
      </section>

      <section class="group">
        <h3>User</h3>
        <p v-if="!userMemories.length" class="muted">No user memories yet.</p>
        <ul v-else class="list">
          <li v-for="m in userMemories" :key="m.id">
            <template v-if="editingId === m.id">
              <textarea v-model="editDraft" rows="2" />
              <div class="row-actions">
                <button type="button" class="btn primary" @click="saveEdit">Save</button>
                <button type="button" class="btn" @click="cancelEdit">Cancel</button>
              </div>
            </template>
            <template v-else>
              <p class="content">{{ m.content }}</p>
              <div class="row-actions">
                <button type="button" class="link" @click="startEdit(m)">Edit</button>
                <button type="button" class="link danger" @click="remove(m.id)">Delete</button>
              </div>
            </template>
          </li>
        </ul>
      </section>
    </div>
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

.intro {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.45;
}

.summary {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.summary .label {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.summary pre {
  margin: 0;
  padding: 0.55rem 0.65rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.75rem;
  font-family: var(--mono);
  max-height: 7rem;
  overflow-y: auto;
  color: var(--text);
}

.add {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.add textarea,
.list textarea {
  width: 100%;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.55rem 0.65rem;
  color: var(--text);
  outline: none;
  resize: vertical;
  font: inherit;
  line-height: 1.45;
  box-sizing: border-box;
}

.add-row {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  flex-wrap: wrap;
}

.scope {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.group h3 {
  margin: 0 0 0.5rem;
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}

.list li {
  padding: 0.55rem 0.65rem;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 8px;
}

.content {
  margin: 0 0 0.4rem;
  font-size: 0.85rem;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
}

.row-actions {
  display: flex;
  gap: 0.65rem;
}

.link {
  border: none;
  background: transparent;
  padding: 0;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--accent);
}

.link.danger {
  color: var(--danger);
}

.muted {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.error {
  margin: 0;
  color: var(--danger);
  font-size: 0.85rem;
}

.btn {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.35rem 0.75rem;
  font-weight: 600;
  font-size: 0.8rem;
  background: var(--bg-elevated);
  color: var(--text);
  margin-left: auto;
}

.btn.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
</style>
