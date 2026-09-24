<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { fetchHealth, streamChat } from './api'
import {
  type Chat,
  createChat,
  loadActiveId,
  loadChats,
  saveActiveId,
  saveChats,
  titleFromMessage,
  uid,
} from './types'

const chats = ref<Chat[]>([])
const activeId = ref<string | null>(null)
const draft = ref('')
const streaming = ref(false)
const error = ref<string | null>(null)
const modelLabel = ref<string>('')
const messagesEl = ref<HTMLElement | null>(null)
let abort: AbortController | null = null

const activeChat = computed(() => chats.value.find((c) => c.id === activeId.value) ?? null)

const sortedChats = computed(() =>
  [...chats.value].sort((a, b) => b.updatedAt - a.updatedAt),
)

function persist() {
  saveChats(chats.value)
  saveActiveId(activeId.value)
}

function scrollToBottom() {
  nextTick(() => {
    const el = messagesEl.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function newChat() {
  const chat = createChat()
  chats.value = [chat, ...chats.value]
  activeId.value = chat.id
  draft.value = ''
  error.value = null
  persist()
}

function selectChat(id: string) {
  if (streaming.value) return
  activeId.value = id
  error.value = null
  persist()
}

function deleteChat(id: string) {
  if (streaming.value) return
  chats.value = chats.value.filter((c) => c.id !== id)
  if (activeId.value === id) {
    activeId.value = chats.value[0]?.id ?? null
    if (!activeId.value) {
      newChat()
      return
    }
  }
  persist()
}

async function send() {
  const text = draft.value.trim()
  if (!text || streaming.value) return

  let chat = activeChat.value
  if (!chat) {
    newChat()
    chat = activeChat.value
  }
  if (!chat) return

  error.value = null
  draft.value = ''

  const userMsg = { id: uid(), role: 'user' as const, content: text }
  chat.messages.push(userMsg)
  if (chat.messages.filter((m) => m.role === 'user').length === 1) {
    chat.title = titleFromMessage(text)
  }
  chat.updatedAt = Date.now()

  const assistantId = uid()
  chat.messages.push({ id: assistantId, role: 'assistant', content: '' })
  persist()
  scrollToBottom()

  const assistant = () => chat!.messages.find((m) => m.id === assistantId)

  streaming.value = true
  abort = new AbortController()

  try {
    await streamChat(
      chat.messages
        .filter((m) => m.id !== assistantId)
        .map((m) => ({ role: m.role, content: m.content })),
      (chunk) => {
        if (chunk.content) {
          const msg = assistant()
          if (msg) msg.content += chunk.content
          scrollToBottom()
        }
      },
      abort.signal,
    )
    const msg = assistant()
    if (msg && !msg.content) {
      msg.content = '(empty response)'
    }
  } catch (e) {
    const msg = assistant()
    if ((e as Error).name === 'AbortError') {
      if (msg && !msg.content) msg.content = '(stopped)'
    } else {
      error.value = e instanceof Error ? e.message : 'Request failed'
      if (msg && !msg.content) {
        chat.messages = chat.messages.filter((m) => m.id !== assistantId)
      }
    }
  } finally {
    streaming.value = false
    abort = null
    chat.updatedAt = Date.now()
    persist()
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

watch(chats, persist, { deep: true })

onMounted(async () => {
  chats.value = loadChats()
  activeId.value = loadActiveId()
  if (!chats.value.length) {
    newChat()
  } else if (!activeId.value || !chats.value.some((c) => c.id === activeId.value)) {
    activeId.value = chats.value[0].id
    persist()
  }
  const health = await fetchHealth()
  if (health?.model) modelLabel.value = health.model
})
</script>

<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="sidebar-top">
        <div class="brand">Polentrix</div>
        <button type="button" class="btn primary" :disabled="streaming" @click="newChat">
          New chat
        </button>
      </div>
      <nav class="chat-list" aria-label="Chats">
        <button
          v-for="chat in sortedChats"
          :key="chat.id"
          type="button"
          class="chat-item"
          :class="{ active: chat.id === activeId }"
          :disabled="streaming"
          @click="selectChat(chat.id)"
        >
          <span class="chat-title">{{ chat.title }}</span>
          <span
            class="chat-delete"
            title="Delete"
            @click.stop="deleteChat(chat.id)"
          >×</span>
        </button>
      </nav>
      <div v-if="modelLabel" class="sidebar-foot">{{ modelLabel }}</div>
    </aside>

    <main class="main">
      <header class="main-header">
        <h1>{{ activeChat?.title ?? 'Chat' }}</h1>
      </header>

      <div ref="messagesEl" class="messages">
        <div v-if="!activeChat?.messages.length" class="empty">
          <p>Ask anything. Replies stream from your local Ollama model.</p>
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
            :disabled="streaming"
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
            :disabled="!draft.trim()"
            @click="send"
          >
            Send
          </button>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: 260px 1fr;
  height: 100%;
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

.sidebar-foot {
  padding: 0.75rem 1rem;
  font-size: 0.75rem;
  color: var(--text-muted);
  border-top: 1px solid var(--border);
  font-family: var(--mono);
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
}

.main-header h1 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-muted);
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

@media (max-width: 720px) {
  .layout {
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
