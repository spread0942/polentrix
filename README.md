# Polentrix

Local ChatGPT-style chatbot powered by [Ollama](https://ollama.com/), with a Vue UI and a Go streaming API.

## Stack

| Piece | Role |
|-------|------|
| **web** (Vue 3 + Vite) | Chat UI — chat list, streaming, model/prompt settings |
| **api** (Go) | SSE proxy to Ollama + Postgres conversation store |
| **db** (Postgres) | Conversations and messages |
| **ollama** | Local LLM runtime |
| **ollama-init** | Pulls the default model on first start |

Conversations and messages are stored in Postgres (`DATABASE_URL`). The UI migrates any older `localStorage` chats once on first load.

## Memory (Phase 2)

- **Short-term:** when a conversation exceeds the context budget (`CONTEXT_MAX_TOKENS`), older turns are summarized server-side; recent messages stay verbatim and the summary is injected into the system prompt.
- **Long-term:** durable facts are stored in Postgres as user-scoped or conversation-scoped memories (`MEMORY_USER_ID`). Relevant memories are retrieved (keyword overlap) and injected before generation. Inspect/edit/delete them from **Memories** in the UI. Optional auto-extract after each turn (`AUTO_EXTRACT_MEMORIES`).

## Requirements

- Docker + Docker Compose
- ~1GB+ free RAM for `qwen2.5:0.5b` (more for larger tags)

## Quick start

```bash
cp .env.example .env   # optional
docker compose up --build
```

On first boot, `ollama-init` pulls `qwen2.5:0.5b` (can take a few minutes).

Then open (via Traefik on port 80):

- UI: http://localhost or http://web.localhost
- API health: http://api.localhost/health
- Ollama: http://ollama.localhost
- Traefik dashboard: http://traefik.localhost

## Models

Default: `qwen2.5:0.5b` (lightest smoke test).

| Goal | Set in `.env` |
|------|----------------|
| Fastest test | `OLLAMA_MODEL=qwen2.5:0.5b` |
| Better quality, still light | `OLLAMA_MODEL=qwen2.5:1.5b` |
| Comfortable next step | `OLLAMA_MODEL=qwen2.5:3b` |

After changing the model:

```bash
docker compose up -d
# or pull manually:
docker compose exec ollama ollama pull qwen2.5:1.5b
```

Then restart the API so it uses the new default:

```bash
docker compose up -d api
```

## Local development (without rebuilding web)

1. Start Traefik + Ollama + API:

```bash
docker compose up traefik ollama ollama-init api
```

2. In another terminal:

```bash
cd frontend
npm install
npm run dev
```

Vite serves http://localhost:5173 and proxies `/api` and `/health` to `http://api.localhost` (Traefik).

Run the API on the host instead:

```bash
cd backend
OLLAMA_BASE_URL=http://ollama.localhost OLLAMA_MODEL=qwen2.5:0.5b go run ./cmd/api
```

## API

- `GET /health` — status + configured model
- `GET /api/models` — models known to Ollama + default
- `POST /api/chat` — body `{ "conversation_id"?, "messages"?, "model"?, "options"? }`, response is SSE. When `conversation_id` is set, the server assembles context (summary + memories + recent messages).
- `GET/POST /api/memories` — list/create long-term memories (`?conversation_id=&scope=user|conversation|all`)
- `GET/PATCH/DELETE /api/memories/{id}` — inspect/edit/delete (scoped to `MEMORY_USER_ID`)

```
data: {"content":"Hello","done":false,"role":"assistant"}

data: [DONE]
```

## Project layout

```
polentrix/
  docker-compose.yml
  .env.example
  backend/     # Go API
  frontend/    # Vue SPA
```

## Later

- Auth / multi-user
- Worker for RAG or file processing
- Larger Qwen (or other) models as your hardware allows
