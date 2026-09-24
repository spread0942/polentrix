export type Role = 'user' | 'assistant' | 'system'

export interface Message {
  id: string
  role: Role
  content: string
  createdAt?: number
  position?: number
}

export interface Chat {
  id: string
  title: string
  messages: Message[]
  systemPrompt: string
  temperature: number | null
  topP: number | null
  numPredict: number | null
  model: string
  contextSummary: string
  summarizedUntil: number
  createdAt: number
  updatedAt: number
}

const ACTIVE_KEY = 'polentrix.activeChatId'
const MODEL_KEY = 'polentrix.selectedModel'
const LEGACY_STORAGE_KEY = 'polentrix.chats'

export function loadSelectedModel(): string | null {
  return localStorage.getItem(MODEL_KEY)
}

export function saveSelectedModel(model: string | null): void {
  if (model) localStorage.setItem(MODEL_KEY, model)
  else localStorage.removeItem(MODEL_KEY)
}

export function uid(): string {
  return crypto.randomUUID()
}

export function loadActiveId(): string | null {
  return localStorage.getItem(ACTIVE_KEY)
}

export function saveActiveId(id: string | null): void {
  if (id) localStorage.setItem(ACTIVE_KEY, id)
  else localStorage.removeItem(ACTIVE_KEY)
}

/** One-time migration helper for chats previously stored in localStorage. */
export function loadLegacyChats(): Chat[] {
  try {
    const raw = localStorage.getItem(LEGACY_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Array<{
      id: string
      title: string
      messages: Message[]
      updatedAt: number
      systemPrompt?: string
      temperature?: number | null
      topP?: number | null
      numPredict?: number | null
      model?: string
      createdAt?: number
    }>
    if (!Array.isArray(parsed)) return []
    return parsed.map((c) => ({
      id: c.id,
      title: c.title || 'New chat',
      messages: c.messages || [],
      systemPrompt: c.systemPrompt ?? '',
      temperature: c.temperature ?? null,
      topP: c.topP ?? null,
      numPredict: c.numPredict ?? null,
      model: c.model ?? '',
      contextSummary: '',
      summarizedUntil: 0,
      createdAt: c.createdAt ?? c.updatedAt ?? Date.now(),
      updatedAt: c.updatedAt ?? Date.now(),
    }))
  } catch {
    return []
  }
}

export function clearLegacyChats(): void {
  localStorage.removeItem(LEGACY_STORAGE_KEY)
}

export function titleFromMessage(content: string): string {
  const trimmed = content.trim().replace(/\s+/g, ' ')
  if (!trimmed) return 'New chat'
  return trimmed.length > 42 ? trimmed.slice(0, 42) + '…' : trimmed
}

export const DEFAULT_TEMPERATURE = 0.7
