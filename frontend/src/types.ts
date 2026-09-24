export type Role = 'user' | 'assistant' | 'system'

export interface Message {
  id: string
  role: Role
  content: string
}

export interface Chat {
  id: string
  title: string
  messages: Message[]
  updatedAt: number
}

const STORAGE_KEY = 'polentrix.chats'
const ACTIVE_KEY = 'polentrix.activeChatId'

export function uid(): string {
  return crypto.randomUUID()
}

export function loadChats(): Chat[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Chat[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export function saveChats(chats: Chat[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(chats))
}

export function loadActiveId(): string | null {
  return localStorage.getItem(ACTIVE_KEY)
}

export function saveActiveId(id: string | null): void {
  if (id) localStorage.setItem(ACTIVE_KEY, id)
  else localStorage.removeItem(ACTIVE_KEY)
}

export function createChat(): Chat {
  return {
    id: uid(),
    title: 'New chat',
    messages: [],
    updatedAt: Date.now(),
  }
}

export function titleFromMessage(content: string): string {
  const trimmed = content.trim().replace(/\s+/g, ' ')
  if (!trimmed) return 'New chat'
  return trimmed.length > 42 ? trimmed.slice(0, 42) + '…' : trimmed
}
