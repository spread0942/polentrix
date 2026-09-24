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

function apiBase(): string {
  const base = import.meta.env.VITE_API_BASE_URL
  return base === undefined || base === null ? '' : base.replace(/\/$/, '')
}

export async function streamChat(
  messages: ChatMessage[],
  onChunk: (chunk: StreamChunk) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${apiBase()}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages }),
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

  if (!res.body) {
    throw new Error('No response body')
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
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
      try {
        const parsed = JSON.parse(data) as StreamChunk
        onChunk(parsed)
        if (parsed.error) throw new Error(parsed.error)
      } catch (e) {
        if (e instanceof SyntaxError) continue
        throw e
      }
    }
  }
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
