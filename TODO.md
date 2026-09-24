# TODO — Local AI Chat Pipeline

## Architecture target

```text
                         ┌── LLM
                         │
User → Chat → Pipeline ──┼── Conversation history
                         ├── Long-term memory
                         ├── RAG / Documents
                         ├── Web search
                         ├── Tools
                         └── System prompt
```

The implementation should be incremental.
Each phase should leave the application fully functional before moving to the next one.

---

# Phase 1 — Core Chat

Goal: build a reliable local chatbot before adding advanced features.

### LLM

* [ ] Model selection in the UI
* [ ] Support multiple Ollama models/tags
* [ ] Streaming responses through the existing SSE path
* [ ] Configurable temperature
* [ ] Configurable generation parameters
* [ ] Handle model loading/errors gracefully
* [ ] Display the currently active model

### Conversation

* [ ] Persist conversations server-side
* [ ] SQLite as the initial database
* [ ] Conversation CRUD
* [ ] Message CRUD
* [ ] Correctly send conversation history to the LLM
* [ ] Create a new conversation
* [ ] Rename/delete conversations
* [ ] Per-conversation isolation

### System Prompt

* [ ] Default system prompt
* [ ] Per-conversation system prompt
* [ ] Editable system prompt in the UI
* [ ] Prompt presets
* [ ] Display the active system prompt

---

# Phase 2 — Memory

Goal: allow the assistant to retain useful information beyond a single conversation.

### Short-term memory

* [ ] Manage conversation context window
* [ ] Detect when conversation becomes too large
* [ ] Summarize older messages
* [ ] Keep recent messages verbatim
* [ ] Inject the summary into the context

### Long-term memory

* [ ] Define a memory data model
* [ ] Store user-specific memories
* [ ] Store conversation-specific memories
* [ ] Retrieve relevant memories before generating a response
* [ ] Inject memories into the LLM context
* [ ] Allow users to inspect stored memories
* [ ] Allow users to edit/delete memories
* [ ] Ensure strict user/session isolation

Example:

```text
User
 ↓
New message
 ↓
Retrieve relevant memories
 ↓
Conversation history
 ↓
System prompt
 ↓
LLM
 ↓
Response
```

---

# Phase 3 — RAG / Documents

Goal: allow the assistant to answer questions using user-provided documents.

### Document ingestion

* [ ] Document upload
* [ ] Support PDF
* [ ] Support Markdown
* [ ] Support TXT
* [ ] Extract document text
* [ ] Store document metadata
* [ ] Chunk documents
* [ ] Generate embeddings
* [ ] Process documents asynchronously

### Vector store

* [ ] Choose vector database/storage
* [ ] Store document embeddings
* [ ] Store chunk metadata
* [ ] Implement similarity search
* [ ] Filter results by user/document

### Retrieval

* [ ] Retrieve relevant chunks for a message
* [ ] Configure retrieval parameters
* [ ] Inject retrieved context into the LLM prompt
* [ ] Prevent irrelevant documents from contaminating the context

### Citations

* [ ] Track the source document for every retrieved chunk
* [ ] Return source metadata with the response
* [ ] Display citations in the UI
* [ ] Allow the user to open/view the referenced document

Example:

```text
User question
      ↓
Embedding
      ↓
Vector search
      ↓
Relevant chunks
      ↓
System + memory + conversation + RAG context
      ↓
LLM
      ↓
Answer + citations
```

---

# Phase 4 — Web Search

Goal: allow the assistant to retrieve up-to-date information when necessary.

### Search

* [ ] Integrate a search provider
* [ ] Create a search abstraction/interface
* [ ] Search query generation
* [ ] Retrieve search results
* [ ] Fetch relevant pages
* [ ] Extract useful page content
* [ ] Handle timeouts and failed pages

### LLM integration

* [ ] Add web-search context to the prompt
* [ ] Allow web search to be enabled/disabled
* [ ] Allow per-message web search
* [ ] Track which sources were used
* [ ] Display web citations

Example:

```text
User
 ↓
LLM
 ↓
Does this require external information?
 ↓
Search tool
 ↓
Web
 ↓
Relevant sources
 ↓
LLM
 ↓
Answer + sources
```

---

# Phase 5 — Tools

Goal: allow the LLM to interact with controlled application functionality.

### Tool framework

* [ ] Define a generic tool interface
* [ ] Register tools
* [ ] Expose tool schemas to the LLM
* [ ] Validate tool arguments
* [ ] Execute tools
* [ ] Return tool results to the LLM
* [ ] Support multiple tool calls
* [ ] Handle tool failures

### Security

* [ ] Tool allowlist
* [ ] Validate all arguments
* [ ] Restrict filesystem access
* [ ] Restrict network access
* [ ] Add execution timeouts
* [ ] Add configurable permissions
* [ ] Log tool executions

Example tools:

```text
list_files
read_file
search_code
calculate
get_current_time
```

---

# Phase 6 — Agent Loop

Goal: allow the LLM to autonomously combine tools and context.

```text
User
 ↓
LLM
 ↓
Tool call?
 ├── No → Response
 │
 └── Yes
      ↓
   Execute tool
      ↓
   Tool result
      ↓
     LLM
      ↓
   Tool call?
      ↓
     ...
      ↓
   Final response
```

### Agent

* [ ] Implement tool-call loop
* [ ] Maximum iteration limit
* [ ] Token/context budget
* [ ] Detect repeated tool calls
* [ ] Handle failed tools
* [ ] Allow cancellation
* [ ] Stream intermediate status to the UI
* [ ] Log the complete execution trace

---

# Phase 7 — Evaluation & Observability

Goal: measure whether models and pipeline changes actually improve the assistant.

### Evaluation

* [ ] Create a test dataset
* [ ] Create tests for Italian
* [ ] Create tests for coding
* [ ] Create tests for reasoning
* [ ] Create tests for RAG
* [ ] Create tests for tool calling
* [ ] Run the same tests against different models
* [ ] Store evaluation results

### Metrics

* [ ] Response latency
* [ ] Tokens generated
* [ ] Tokens/second
* [ ] Context size
* [ ] Tool-call success rate
* [ ] RAG retrieval quality
* [ ] Model comparison

Example:

```text
                    Qwen3 8B    Qwen3 14B
──────────────────────────────────────────
Italian             ✓           ✓
Coding              ✓           ✓
Reasoning           ?           ?
RAG                 ?           ?
Tool calling        ?           ?
Tokens/sec          ✓           ?
RAM usage            ✓           ?
```

---

# Recommended implementation order

```text
1. Core Chat
       ↓
2. Memory
       ↓
3. RAG
       ↓
4. Web Search
       ↓
5. Tools
       ↓
6. Agent
       ↓
7. Evaluation
```

Do not implement all phases at once.

After completing each phase:

1. Verify the current functionality.
2. Add tests for the new functionality.
3. Keep the application usable.
4. Only then move to the next phase.

## Initial target

The first usable version should support:

```text
┌──────────────────────────────────────┐
│              Local Chat              │
├──────────────────────────────────────┤
│ Model: Qwen3 8B                      │
│                                      │
│ Conversation history                 │
│                                      │
│ User message                         │
│ Assistant response (streaming)       │
│                                      │
├──────────────────────────────────────┤
│ System prompt                        │
│ Temperature                          │
│ Model selection                      │
└──────────────────────────────────────┘
```

The architecture should remain modular so that Memory, RAG, Web Search and Tools can be added later without rewriting the core chat pipeline.
