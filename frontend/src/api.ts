export interface ChatMessage {
  role: string
  content: string
}

export interface StreamChunk {
  content?: string
  done?: boolean
  role?: string
  error?: string
}

export interface CatalogModel {
  name: string
  downloaded: boolean
  size?: number
  modified_at?: string
}

export interface ModelsResponse {
  default: string
  models: CatalogModel[]
}

export interface PullProgress {
  status?: string
  digest?: string
  total?: number
  completed?: number
  error?: string
}

export interface ChatOptions {
  temperature?: number
  top_p?: number
  top_k?: number
  num_predict?: number
}

export interface PromptPreset {
  id: string
  name: string
  prompt: string
}

export interface ApiConversation {
  id: string
  title: string
  system_prompt: string
  temperature: number | null
  top_p: number | null
  num_predict: number | null
  model: string
  context_summary?: string
  summarized_until?: number
  created_at: number
  updated_at: number
  messages?: ApiMessage[]
}

export interface ApiMessage {
  id: string
  conversation_id?: string
  role: string
  content: string
  created_at: number
  position: number
}

function apiBase(): string {
  const base = import.meta.env.VITE_API_BASE_URL
  return base === undefined || base === null ? '' : base.replace(/\/$/, '')
}

async function readJSON<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let detail = res.statusText
    try {
      const err = (await res.json()) as { error?: string }
      if (err.error) detail = err.error
    } catch {
      /* ignore */
    }
    throw new Error(detail || `Request failed (${res.status})`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

async function readSSE(
  res: Response,
  onChunk: (data: string) => void,
  signal?: AbortSignal,
): Promise<void> {
  if (!res.body) {
    throw new Error('No response body')
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    if (signal?.aborted) {
      await reader.cancel()
      throw new DOMException('Aborted', 'AbortError')
    }
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    const parts = buffer.split('\n')
    buffer = parts.pop() ?? ''

    for (const line of parts) {
      const trimmed = line.trim()
      if (!trimmed.startsWith('data:')) continue
      const data = trimmed.slice(5).trim()
      if (data === '[DONE]') return
      onChunk(data)
    }
  }
}

export async function streamChat(
  messages: ChatMessage[],
  onChunk: (chunk: StreamChunk) => void,
  signal?: AbortSignal,
  model?: string,
  options?: ChatOptions,
  conversationId?: string,
): Promise<void> {
  const body: {
    messages: ChatMessage[]
    model?: string
    options?: ChatOptions
    conversation_id?: string
  } = { messages }
  if (model) body.model = model
  if (options) body.options = options
  if (conversationId) body.conversation_id = conversationId

  const res = await fetch(`${apiBase()}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  })

  if (!res.ok) {
    let detail = res.statusText
    try {
      const err = (await res.json()) as { error?: string }
      if (err.error) detail = err.error
    } catch {
      /* ignore */
    }
    throw new Error(detail || `Request failed (${res.status})`)
  }

  await readSSE(
    res,
    (data) => {
      try {
        const parsed = JSON.parse(data) as StreamChunk
        onChunk(parsed)
        if (parsed.error) throw new Error(parsed.error)
      } catch (e) {
        if (e instanceof SyntaxError) return
        throw e
      }
    },
    signal,
  )
}

export async function fetchHealth(): Promise<{
  status: string
  model: string
  default_system_prompt?: string
  default_temperature?: number
} | null> {
  try {
    const res = await fetch(`${apiBase()}/health`)
    if (!res.ok) return null
    return (await res.json()) as {
      status: string
      model: string
      default_system_prompt?: string
      default_temperature?: number
    }
  } catch {
    return null
  }
}

export async function fetchModels(): Promise<ModelsResponse | null> {
  try {
    const res = await fetch(`${apiBase()}/api/models`)
    if (!res.ok) return null
    return (await res.json()) as ModelsResponse
  } catch {
    return null
  }
}

export async function fetchPromptPresets(): Promise<PromptPreset[]> {
  try {
    const res = await fetch(`${apiBase()}/api/prompt-presets`)
    if (!res.ok) return []
    const data = (await res.json()) as { presets: PromptPreset[] }
    return data.presets ?? []
  } catch {
    return []
  }
}

export async function pullModel(
  name: string,
  onProgress: (progress: PullProgress) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${apiBase()}/api/models/pull`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
    signal,
  })

  if (!res.ok) {
    let detail = res.statusText
    try {
      const err = (await res.json()) as { error?: string }
      if (err.error) detail = err.error
    } catch {
      /* ignore */
    }
    throw new Error(detail || `Pull failed (${res.status})`)
  }

  await readSSE(
    res,
    (data) => {
      try {
        const parsed = JSON.parse(data) as PullProgress
        onProgress(parsed)
        if (parsed.error) throw new Error(parsed.error)
      } catch (e) {
        if (e instanceof SyntaxError) return
        throw e
      }
    },
    signal,
  )
}

export async function listConversations(): Promise<ApiConversation[]> {
  const res = await fetch(`${apiBase()}/api/conversations`)
  const data = await readJSON<{ conversations: ApiConversation[] }>(res)
  return data.conversations ?? []
}

export async function getConversation(id: string): Promise<ApiConversation> {
  const res = await fetch(`${apiBase()}/api/conversations/${id}`)
  return readJSON<ApiConversation>(res)
}

export async function createConversation(body: {
  id: string
  title?: string
  system_prompt?: string
  temperature?: number | null
  top_p?: number | null
  num_predict?: number | null
  model?: string
}): Promise<ApiConversation> {
  const res = await fetch(`${apiBase()}/api/conversations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return readJSON<ApiConversation>(res)
}

export async function patchConversation(
  id: string,
  body: Record<string, unknown>,
): Promise<ApiConversation> {
  const res = await fetch(`${apiBase()}/api/conversations/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return readJSON<ApiConversation>(res)
}

export async function deleteConversation(id: string): Promise<void> {
  const res = await fetch(`${apiBase()}/api/conversations/${id}`, { method: 'DELETE' })
  await readJSON<void>(res)
}

export interface ApiMemory {
  id: string
  user_id: string
  conversation_id: string | null
  content: string
  created_at: number
  updated_at: number
}

export async function listMemories(opts?: {
  conversationId?: string
  scope?: 'user' | 'conversation' | 'all'
}): Promise<ApiMemory[]> {
  const params = new URLSearchParams()
  if (opts?.conversationId) params.set('conversation_id', opts.conversationId)
  if (opts?.scope) params.set('scope', opts.scope)
  const qs = params.toString()
  const res = await fetch(`${apiBase()}/api/memories${qs ? `?${qs}` : ''}`)
  const data = await readJSON<{ memories: ApiMemory[] }>(res)
  return data.memories ?? []
}

export async function createMemory(body: {
  id: string
  content: string
  conversation_id?: string | null
}): Promise<ApiMemory> {
  const res = await fetch(`${apiBase()}/api/memories`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return readJSON<ApiMemory>(res)
}

export async function patchMemory(id: string, content: string): Promise<ApiMemory> {
  const res = await fetch(`${apiBase()}/api/memories/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  })
  return readJSON<ApiMemory>(res)
}

export async function deleteMemory(id: string): Promise<void> {
  const res = await fetch(`${apiBase()}/api/memories/${id}`, { method: 'DELETE' })
  await readJSON<void>(res)
}

export async function createMessage(
  conversationId: string,
  body: { id: string; role: string; content: string },
): Promise<ApiMessage> {
  const res = await fetch(`${apiBase()}/api/conversations/${conversationId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return readJSON<ApiMessage>(res)
}

export async function patchMessage(
  conversationId: string,
  messageId: string,
  content: string,
): Promise<ApiMessage> {
  const res = await fetch(
    `${apiBase()}/api/conversations/${conversationId}/messages/${messageId}`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content }),
    },
  )
  return readJSON<ApiMessage>(res)
}
