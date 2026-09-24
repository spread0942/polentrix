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

function apiBase(): string {
  const base = import.meta.env.VITE_API_BASE_URL
  return base === undefined || base === null ? '' : base.replace(/\/$/, '')
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
): Promise<void> {
  const body: { messages: ChatMessage[]; model?: string } = { messages }
  if (model) body.model = model

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

export async function fetchHealth(): Promise<{ status: string; model: string } | null> {
  try {
    const res = await fetch(`${apiBase()}/health`)
    if (!res.ok) return null
    return (await res.json()) as { status: string; model: string }
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
