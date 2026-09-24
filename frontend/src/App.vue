<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import {
  createConversation,
  createMessage,
  deleteConversation,
  fetchHealth,
  getConversation,
  listConversations,
  patchConversation,
  patchMessage,
  streamChat,
  type ApiConversation,
  type ChatOptions,
} from './api'
import ChatSettings from './ChatSettings.vue'
import MemoryPanel from './MemoryPanel.vue'
import ModelsSettings from './ModelsSettings.vue'
import {
  type Chat,
  type Message,
  clearLegacyChats,
  DEFAULT_TEMPERATURE,
  loadActiveId,
  loadLegacyChats,
  loadSelectedModel,
  saveActiveId,
  saveSelectedModel,
  titleFromMessage,
  uid,
} from './types'

const chats = ref<Chat[]>([])
const activeId = ref<string | null>(null)
const draft = ref('')
const streaming = ref(false)
const loading = ref(true)
const error = ref<string | null>(null)
const modelLabel = ref('')
const view = ref<'chat' | 'models'>('chat')
const showSettings = ref(false)
const showMemories = ref(false)
const renamingId = ref<string | null>(null)
const renameDraft = ref('')
const defaultSystemPrompt = ref('You are Polentrix, a helpful local AI assistant.')
const defaultTemperature = ref(DEFAULT_TEMPERATURE)
const messagesEl = ref<HTMLElement | null>(null)
let abort: AbortController | null = null

const activeChat = computed(() => chats.value.find((c) => c.id === activeId.value) ?? null)

const sortedChats = computed(() =>
  [...chats.value].sort((a, b) => b.updatedAt - a.updatedAt),
)

function fromApi(c: ApiConversation): Chat {
  return {
    id: c.id,
    title: c.title,
    systemPrompt: c.system_prompt ?? '',
    temperature: c.temperature,
    topP: c.top_p,
    numPredict: c.num_predict,
    model: c.model ?? '',
    contextSummary: c.context_summary ?? '',
    summarizedUntil: c.summarized_until ?? 0,
    createdAt: c.created_at,
    updatedAt: c.updated_at,
    messages: (c.messages ?? []).map(
      (m): Message => ({
        id: m.id,
        role: m.role as Message['role'],
        content: m.content,
        createdAt: m.created_at,
        position: m.position,
      }),
    ),
  }
}

function persistActive() {
  saveActiveId(activeId.value)
}

function scrollToBottom() {
  nextTick(() => {
    const el = messagesEl.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function chatOptions(chat: Chat): ChatOptions | undefined {
  const opts: ChatOptions = {}
  const temp = chat.temperature ?? defaultTemperature.value
  if (temp != null) opts.temperature = temp
  if (chat.topP != null) opts.top_p = chat.topP
  if (chat.numPredict != null) opts.num_predict = chat.numPredict
  return Object.keys(opts).length ? opts : undefined
}

function llmMessages(chat: Chat, excludeAssistantId?: string) {
  const out: { role: string; content: string }[] = []
  const system = (chat.systemPrompt || defaultSystemPrompt.value).trim()
  if (system) {
    out.push({ role: 'system', content: system })
  }
  for (const m of chat.messages) {
    if (excludeAssistantId && m.id === excludeAssistantId) continue
    if (m.role === 'system') continue
    out.push({ role: m.role, content: m.content })
  }
  return out
}

async function refreshList() {
  const list = await listConversations()
  const detailed: Chat[] = []
  for (const summary of list) {
    const existing = chats.value.find((c) => c.id === summary.id)
    if (existing && existing.id === activeId.value && existing.messages.length) {
      detailed.push({
        ...fromApi(summary),
        messages: existing.messages,
      })
    } else if (summary.id === activeId.value) {
      const full = await getConversation(summary.id)
      detailed.push(fromApi(full))
    } else {
      detailed.push(fromApi(summary))
    }
  }
  chats.value = detailed
}

async function ensureActiveLoaded() {
  if (!activeId.value) return
  const full = await getConversation(activeId.value)
  const chat = fromApi(full)
  const idx = chats.value.findIndex((c) => c.id === chat.id)
  if (idx >= 0) chats.value[idx] = chat
  else chats.value = [chat, ...chats.value]
}

async function newChat() {
  const id = uid()
  const created = await createConversation({
    id,
    title: 'New chat',
    system_prompt: defaultSystemPrompt.value,
    temperature: defaultTemperature.value,
    model: modelLabel.value || undefined,
  })
  const chat = fromApi(created)
  chats.value = [chat, ...chats.value.filter((c) => c.id !== chat.id)]
  activeId.value = chat.id
  draft.value = ''
  error.value = null
  showSettings.value = false
  showMemories.value = false
  persistActive()
}

async function selectChat(id: string) {
  if (streaming.value) return
  activeId.value = id
  error.value = null
  showSettings.value = false
  showMemories.value = false
  persistActive()
  try {
    await ensureActiveLoaded()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load chat'
  }
}

async function deleteChat(id: string) {
  if (streaming.value) return
  try {
    await deleteConversation(id)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Delete failed'
    return
  }
  chats.value = chats.value.filter((c) => c.id !== id)
  if (activeId.value === id) {
    activeId.value = chats.value[0]?.id ?? null
    if (!activeId.value) {
      await newChat()
      return
    }
    await ensureActiveLoaded()
  }
  persistActive()
}

function startRename(chat: Chat) {
  if (streaming.value) return
  renamingId.value = chat.id
  renameDraft.value = chat.title
}

async function commitRename() {
  const id = renamingId.value
  if (!id) return
  const title = renameDraft.value.trim() || 'New chat'
  renamingId.value = null
  try {
    const updated = await patchConversation(id, { title })
    const chat = chats.value.find((c) => c.id === id)
    if (chat) {
      chat.title = updated.title
      chat.updatedAt = updated.updated_at
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Rename failed'
  }
}

function cancelRename() {
  renamingId.value = null
}

function openModels() {
  if (streaming.value) return
  view.value = 'models'
  showSettings.value = false
  showMemories.value = false
}

function closeModels() {
  view.value = 'chat'
}

function onSelectModel(name: string) {
  modelLabel.value = name
  saveSelectedModel(name)
}

async function saveSettings(payload: {
  title: string
  systemPrompt: string
  temperature: number | null
  topP: number | null
  numPredict: number | null
}) {
  const chat = activeChat.value
  if (!chat) return
  try {
    const body: Record<string, unknown> = {
      title: payload.title,
      system_prompt: payload.systemPrompt,
      temperature: payload.temperature,
      top_p: payload.topP,
      num_predict: payload.numPredict,
    }
    if (payload.topP == null) body.clear_top_p = true
    if (payload.numPredict == null) body.clear_num_predict = true
    const updated = await patchConversation(chat.id, body)
    chat.title = updated.title
    chat.systemPrompt = updated.system_prompt
    chat.temperature = updated.temperature
    chat.topP = updated.top_p
    chat.numPredict = updated.num_predict
    chat.updatedAt = updated.updated_at
    showSettings.value = false
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to save settings'
  }
}

function openMemories() {
  if (!activeChat.value || streaming.value) return
  showMemories.value = !showMemories.value
  if (showMemories.value) showSettings.value = false
}

function openPromptSettings() {
  if (!activeChat.value || streaming.value) return
  showSettings.value = !showSettings.value
  if (showSettings.value) showMemories.value = false
}

async function migrateLegacyIfNeeded() {
  const legacy = loadLegacyChats()
  if (!legacy.length) return
  for (const chat of legacy) {
    try {
      await createConversation({
        id: chat.id,
        title: chat.title,
        system_prompt: chat.systemPrompt || defaultSystemPrompt.value,
        temperature: chat.temperature ?? defaultTemperature.value,
        top_p: chat.topP,
        num_predict: chat.numPredict,
        model: chat.model || modelLabel.value || undefined,
      })
      for (const m of chat.messages) {
        await createMessage(chat.id, {
          id: m.id,
          role: m.role,
          content: m.content,
        })
      }
    } catch {
      /* conversation may already exist */
    }
  }
  clearLegacyChats()
}

async function send() {
  const text = draft.value.trim()
  if (!text || streaming.value) return

  let chat = activeChat.value
  if (!chat) {
    await newChat()
    chat = activeChat.value
  }
  if (!chat) return

  error.value = null
  draft.value = ''

  const userMsg: Message = { id: uid(), role: 'user', content: text }
  chat.messages.push(userMsg)
  const firstUser = chat.messages.filter((m) => m.role === 'user').length === 1
  if (firstUser) {
    chat.title = titleFromMessage(text)
  }
  chat.updatedAt = Date.now()

  try {
    await createMessage(chat.id, { id: userMsg.id, role: 'user', content: text })
    if (firstUser) {
      await patchConversation(chat.id, { title: chat.title })
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to save message'
    chat.messages = chat.messages.filter((m) => m.id !== userMsg.id)
    return
  }

  const assistantId = uid()
  chat.messages.push({ id: assistantId, role: 'assistant', content: '' })
  scrollToBottom()

  try {
    await createMessage(chat.id, { id: assistantId, role: 'assistant', content: '' })
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to create assistant message'
    chat.messages = chat.messages.filter((m) => m.id !== assistantId)
    return
  }

  const assistant = () => chat!.messages.find((m) => m.id === assistantId)

  streaming.value = true
  abort = new AbortController()

  try {
    await streamChat(
      llmMessages(chat, assistantId),
      (chunk) => {
        if (chunk.content) {
          const msg = assistant()
          if (msg) msg.content += chunk.content
          scrollToBottom()
        }
      },
      abort.signal,
      modelLabel.value || undefined,
      chatOptions(chat),
      chat.id,
    )
    const msg = assistant()
    if (msg && !msg.content) {
      msg.content = '(empty response)'
    }
    if (msg) {
      await patchMessage(chat.id, assistantId, msg.content)
    }
    try {
      const full = await getConversation(chat.id)
      chat.contextSummary = full.context_summary ?? ''
      chat.summarizedUntil = full.summarized_until ?? 0
    } catch {
      /* ignore summary refresh */
    }
  } catch (e) {
    const msg = assistant()
    if ((e as Error).name === 'AbortError') {
      if (msg && !msg.content) msg.content = '(stopped)'
      if (msg) {
        try {
          await patchMessage(chat.id, assistantId, msg.content)
        } catch {
          /* ignore */
        }
      }
    } else {
      error.value = e instanceof Error ? e.message : 'Request failed'
      if (msg && !msg.content) {
        chat.messages = chat.messages.filter((m) => m.id !== assistantId)
      } else if (msg) {
        try {
          await patchMessage(chat.id, assistantId, msg.content)
        } catch {
          /* ignore */
        }
      }
    }
  } finally {
    streaming.value = false
    abort = null
    chat.updatedAt = Date.now()
    scrollToBottom()
  }
}

function stop() {
  abort?.abort()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    void send()
  }
}

onMounted(async () => {
  loading.value = true
  error.value = null

  const saved = loadSelectedModel()
  if (saved) modelLabel.value = saved

  const health = await fetchHealth()
  if (health?.default_system_prompt) {
    defaultSystemPrompt.value = health.default_system_prompt
  }
  if (typeof health?.default_temperature === 'number') {
    defaultTemperature.value = health.default_temperature
  }
  if (!modelLabel.value && health?.model) {
    modelLabel.value = health.model
    saveSelectedModel(health.model)
  }

  try {
    await migrateLegacyIfNeeded()
    const list = await listConversations()
    chats.value = list.map(fromApi)
    activeId.value = loadActiveId()
    if (!chats.value.length) {
      await newChat()
    } else {
      if (!activeId.value || !chats.value.some((c) => c.id === activeId.value)) {
        activeId.value = chats.value[0].id
      }
      persistActive()
      await ensureActiveLoaded()
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load conversations'
    if (!chats.value.length) {
      /* keep empty; user can retry */
    }
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div
    class="layout"
    :class="{
      'with-settings': (showSettings || showMemories) && view === 'chat',
    }"
  >
    <aside class="sidebar">
      <div class="sidebar-top">
        <div class="brand">Polentrix</div>
        <button type="button" class="btn primary" :disabled="streaming || loading" @click="newChat">
          New chat
        </button>
      </div>
      <nav class="chat-list" aria-label="Chats">
        <div v-for="chat in sortedChats" :key="chat.id" class="chat-row">
          <button
            v-if="renamingId !== chat.id"
            type="button"
            class="chat-item"
            :class="{ active: chat.id === activeId && view === 'chat' }"
            :disabled="streaming"
            @click="selectChat(chat.id); closeModels()"
            @dblclick="startRename(chat)"
          >
            <span class="chat-title">{{ chat.title }}</span>
            <span
              class="chat-delete"
              title="Delete"
              @click.stop="deleteChat(chat.id)"
            >×</span>
          </button>
          <form
            v-else
            class="rename-form"
            @submit.prevent="commitRename"
          >
            <input
              v-model="renameDraft"
              type="text"
              maxlength="120"
              @keydown.escape="cancelRename"
            />
            <button type="submit" class="rename-ok" title="Save">✓</button>
          </form>
        </div>
      </nav>
      <div class="sidebar-foot">
        <div class="model-line">
          <span class="model-label" :title="modelLabel || 'No model'">{{ modelLabel || '—' }}</span>
          <button
            type="button"
            class="settings-btn"
            title="Model settings"
            aria-label="Model settings"
            :disabled="streaming"
            :class="{ active: view === 'models' }"
            @click="openModels"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z"
                stroke="currentColor"
                stroke-width="1.75"
              />
              <path
                d="M19.4 13a7.7 7.7 0 0 0 .06-2l2.03-1.58-2-3.46-2.4.96a7.6 7.6 0 0 0-1.73-1L15 3h-4l-.36 2.92a7.6 7.6 0 0 0-1.73 1l-2.4-.96-2 3.46L6.54 11a7.7 7.7 0 0 0 0 2l-2.03 1.58 2 3.46 2.4-.96a7.6 7.6 0 0 0 1.73 1L11 21h4l.36-2.92a7.6 7.6 0 0 0 1.73-1l2.4.96 2-3.46L19.4 13Z"
                stroke="currentColor"
                stroke-width="1.75"
                stroke-linejoin="round"
              />
            </svg>
          </button>
        </div>
      </div>
    </aside>

    <ModelsSettings
      v-if="view === 'models'"
      :selected-model="modelLabel"
      @back="closeModels"
      @select="onSelectModel"
    />

    <template v-else>
      <main class="main">
        <header class="main-header">
          <h1>{{ activeChat?.title ?? 'Chat' }}</h1>
          <div class="header-actions">
            <button
              type="button"
              class="header-btn"
              :disabled="!activeChat || streaming"
              :class="{ active: showMemories }"
              @click="openMemories"
            >
              Memories
            </button>
            <button
              type="button"
              class="header-btn"
              :disabled="!activeChat || streaming"
              :class="{ active: showSettings }"
              @click="openPromptSettings"
            >
              Prompt &amp; params
            </button>
          </div>
        </header>

        <div ref="messagesEl" class="messages">
          <div v-if="loading" class="empty">
            <p>Loading conversations…</p>
          </div>
          <div v-else-if="!activeChat?.messages.length" class="empty">
            <p>Ask anything. Replies stream from your local Ollama model.</p>
            <p v-if="activeChat?.systemPrompt" class="prompt-hint">
              System: {{ activeChat.systemPrompt }}
            </p>
          </div>
          <div
            v-for="msg in activeChat?.messages ?? []"
            :key="msg.id"
            class="message"
            :class="msg.role"
          >
            <div class="role">{{ msg.role === 'user' ? 'You' : 'Assistant' }}</div>
            <div class="content">{{ msg.content || (streaming ? '…' : '') }}</div>
          </div>
        </div>

        <div class="composer-wrap">
          <p v-if="error" class="error">{{ error }}</p>
          <div class="composer">
            <textarea
              v-model="draft"
              rows="1"
              placeholder="Message…"
              :disabled="streaming || loading"
              @keydown="onKeydown"
            />
            <button
              v-if="streaming"
              type="button"
              class="btn danger"
              @click="stop"
            >
              Stop
            </button>
            <button
              v-else
              type="button"
              class="btn primary"
              :disabled="!draft.trim() || loading"
              @click="send"
            >
              Send
            </button>
          </div>
        </div>
      </main>

      <ChatSettings
        v-if="showSettings && activeChat"
        :title="activeChat.title"
        :system-prompt="activeChat.systemPrompt"
        :temperature="activeChat.temperature"
        :top-p="activeChat.topP"
        :num-predict="activeChat.numPredict"
        :default-temperature="defaultTemperature"
        :default-system-prompt="defaultSystemPrompt"
        @close="showSettings = false"
        @save="saveSettings"
      />

      <MemoryPanel
        v-else-if="showMemories && activeChat"
        :conversation-id="activeChat.id"
        :context-summary="activeChat.contextSummary"
        @close="showMemories = false"
      />
    </template>
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: 260px 1fr;
  height: 100%;
}

.layout.with-settings {
  grid-template-columns: 260px 1fr min(360px, 100%);
}

.sidebar {
  display: flex;
  flex-direction: column;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border);
  min-height: 0;
}

.sidebar-top {
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.brand {
  font-weight: 700;
  letter-spacing: 0.02em;
  font-size: 1.1rem;
  color: #c8e6c9;
}

.chat-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 0.5rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.chat-row {
  min-width: 0;
}

.chat-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  text-align: left;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 8px;
  padding: 0.6rem 0.65rem;
  color: var(--text-muted);
}

.chat-item:hover:not(:disabled) {
  background: var(--bg-elevated);
  color: var(--text);
}

.chat-item.active {
  background: var(--bg-elevated);
  border-color: var(--border);
  color: var(--text);
}

.chat-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.9rem;
}

.chat-delete {
  opacity: 0;
  font-size: 1.1rem;
  line-height: 1;
  padding: 0 0.2rem;
  color: var(--text-muted);
}

.chat-item:hover .chat-delete {
  opacity: 1;
}

.chat-delete:hover {
  color: var(--danger);
}

.rename-form {
  display: flex;
  gap: 0.25rem;
  padding: 0.25rem;
}

.rename-form input {
  flex: 1;
  min-width: 0;
  background: var(--bg-elevated);
  border: 1px solid var(--accent);
  border-radius: 6px;
  padding: 0.4rem 0.5rem;
  color: var(--text);
  outline: none;
  font-size: 0.85rem;
}

.rename-ok {
  border: none;
  background: var(--accent);
  color: #fff;
  border-radius: 6px;
  padding: 0 0.55rem;
  font-weight: 700;
}

.sidebar-foot {
  padding: 0.65rem 0.75rem 0.75rem;
  border-top: 1px solid var(--border);
}

.model-line {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  min-width: 0;
}

.model-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: var(--text-muted);
  font-family: var(--mono);
  padding-left: 0.25rem;
}

.settings-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.85rem;
  height: 1.85rem;
  border-radius: 8px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-muted);
  padding: 0;
}

.settings-btn:hover:not(:disabled) {
  color: var(--text);
  background: var(--bg-elevated);
  border-color: var(--border);
}

.settings-btn.active {
  color: #c8e6c9;
  background: var(--bg-elevated);
  border-color: var(--border);
}

.settings-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  background:
    radial-gradient(ellipse 80% 50% at 50% -20%, rgba(61, 143, 90, 0.12), transparent),
    var(--bg);
}

.main-header {
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.main-header h1 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.header-btn {
  flex-shrink: 0;
  border: 1px solid var(--border);
  background: var(--bg-elevated);
  color: var(--text-muted);
  border-radius: 8px;
  padding: 0.4rem 0.75rem;
  font-size: 0.8rem;
  font-weight: 600;
}

.header-btn:hover:not(:disabled),
.header-btn.active {
  color: var(--text);
  border-color: var(--accent);
}

.header-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.messages {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.empty {
  margin: auto;
  text-align: center;
  color: var(--text-muted);
  max-width: 28rem;
}

.prompt-hint {
  margin-top: 0.75rem;
  font-size: 0.8rem;
  opacity: 0.85;
  font-family: var(--mono);
}

.message {
  max-width: 48rem;
  width: 100%;
  margin: 0 auto;
}

.message.user .content {
  background: var(--user-bubble);
  border-radius: var(--radius);
  padding: 0.85rem 1rem;
  white-space: pre-wrap;
  word-break: break-word;
}

.message.assistant .content {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.55;
}

.role {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  margin-bottom: 0.35rem;
}

.composer-wrap {
  padding: 0 1.5rem 1.5rem;
  max-width: 48rem;
  width: 100%;
  margin: 0 auto;
}

.error {
  color: var(--danger);
  font-size: 0.85rem;
  margin: 0 0 0.5rem;
}

.composer {
  display: flex;
  gap: 0.75rem;
  align-items: flex-end;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 0.65rem 0.75rem;
}

.composer textarea {
  flex: 1;
  resize: none;
  border: none;
  background: transparent;
  outline: none;
  min-height: 1.5rem;
  max-height: 10rem;
  line-height: 1.45;
}

.btn {
  border: none;
  border-radius: 8px;
  padding: 0.55rem 1rem;
  font-weight: 600;
  font-size: 0.9rem;
  white-space: nowrap;
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.btn.primary {
  background: var(--accent);
  color: #fff;
}

.btn.primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn.danger {
  background: var(--danger);
  color: #fff;
}

@media (max-width: 900px) {
  .layout.with-settings {
    grid-template-columns: 260px 1fr;
    grid-template-rows: auto 1fr auto;
  }
}

@media (max-width: 720px) {
  .layout,
  .layout.with-settings {
    grid-template-columns: 1fr;
    grid-template-rows: auto 1fr;
  }

  .sidebar {
    max-height: 40vh;
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
}
</style>
