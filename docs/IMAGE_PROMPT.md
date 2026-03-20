# Story Engine — Image Prompt Feature (v1)

**Goal:** Add a synchronous `POST /v1/image-prompt` endpoint that takes a game state ID and returns a concise, AI-ready image generation prompt describing the most recent narrative moment.

---

## Table of Contents
1. [Overview](#overview)
2. [API Design](#api-design)
3. [Internal Flow](#internal-flow)
4. [Prompt Construction](#prompt-construction)
5. [New Files](#new-files)
6. [Modified Files](#modified-files)
7. [Implementation Checklist](#implementation-checklist)
8. [Future Work](#future-work)

---

## Overview

The image-prompt feature lets a client request an AI-generated image prompt based on the current story moment. It is **synchronous** (not queued) and **read-only** — it does not modify any game state.

Typical usage: after each chat turn, the client calls this endpoint to obtain a prompt it can forward to an image generation service (e.g. Stable Diffusion, Midjourney, DALL-E) to produce an illustration of the scene.

---

## API Design

### Endpoint
```
POST /v1/image-prompt
```

### Request Body
```json
{
  "gamestate_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `gamestate_id` | UUID string | Yes | ID of an existing game state session |

### Success Response — `200 OK`
```json
{
  "prompt": "A moonlit Victorian graveyard at midnight, fog curling around ancient headstones. A determined young woman in a long dark coat stands at the cemetery gates, lantern in hand, facing the shadowed path ahead. Gothic atmosphere, oil-painting style."
}
```

| Field | Type | Description |
|-------|------|-------------|
| `prompt` | string | Ready-to-use image generation prompt |

### Error Responses

| Status | Condition |
|--------|-----------|
| `400 Bad Request` | Missing or malformed `gamestate_id`, or no chat history available |
| `404 Not Found` | No game state found for the provided ID |
| `405 Method Not Allowed` | Non-POST request |
| `500 Internal Server Error` | LLM or storage failure |

---

## Internal Flow

```
Client
  │
  └─ POST /v1/image-prompt { gamestate_id }
       │
       ▼
  ImagePromptHandler.ServeHTTP
       │
       ├─ Validate request (method, body, gamestate_id)
       │
       ├─ storage.LoadGameState(gamestate_id)        ← Redis
       │
       ├─ storage.GetScenario(gs.Scenario)           ← Filesystem
       │
       ├─ prompts.BuildImagePromptMessages(gs, scenario)
       │       │
       │       ├─ System: image prompt generation instructions
       │       └─ User: last exchange + PC description + location description
       │
       ├─ llmService.Chat(ctx, messages)             ← Narrative LLM (non-streaming)
       │
       └─ Return { "prompt": "<llm response>" }
```

The handler depends on both the **storage layer** (to load game state and scenario) and the **LLM service** (to generate the prompt). It reuses the existing narrative LLM — no separate model is required.

---

## Prompt Construction

A new function `BuildImagePromptMessages` in `pkg/prompts` assembles the messages sent to the LLM.

### System Message
Instructs the LLM to act as an image-prompt specialist rather than a narrator:

> You are an expert at writing prompts for AI image generation tools (Stable Diffusion, Midjourney, DALL-E, etc.).
> Based on the game context provided, write a single, concise image generation prompt (80–120 words) that captures the most recent moment of the story.
> Include: setting atmosphere, character appearance, action or mood, and visual style.
> Output ONLY the image prompt. No commentary, no explanation, no quotation marks.

### User Message
Provides scoped context assembled from the game state:

```
SETTING: <current location name>
<current location description>

PLAYER CHARACTER: <PC name>
<PC description>

LAST EXCHANGE:
User: <last user message from chat history>
Narrator: <last assistant message from chat history>

Generate an image generation prompt for this moment.
```

If no chat history exists, the endpoint returns `400 Bad Request` — there is no narrative moment to illustrate yet.

---

## New Files

| File | Purpose |
|------|---------|
| `pkg/prompts/image_prompt.go` | `BuildImagePromptMessages(gs, scenario)` function |
| `internal/handlers/image_prompt.go` | HTTP handler for `POST /v1/image-prompt` |

---

## Modified Files

| File | Change |
|------|--------|
| `cmd/api/main.go` | Register `ImagePromptHandler` on `/v1/image-prompt` |

---

## Implementation Checklist

- [x] `pkg/prompts/image_prompt.go` — `BuildImagePromptMessages`
- [x] `internal/handlers/image_prompt.go` — `ImagePromptHandler`
- [x] `cmd/api/main.go` — route registration

---

## Future Work

- **Console support:** Add an `image-prompt` command to the interactive console.
- **Streaming:** Consider streaming the prompt token-by-token if latency is a concern.
- **Caching:** Cache the last generated prompt on the game state to avoid redundant LLM calls within the same turn.
- **Style injection:** Allow the request to include an optional `style` hint (e.g. `"oil painting"`, `"anime"`) that is appended to the generated prompt.
- **Narrator aesthetic:** Incorporate the narrator's stylistic identity into the visual style guidance (e.g. a Poe narrator → dark, gothic; a comedic narrator → bright, cartoonish).
- **OpenAPI spec:** Document the endpoint in `docs/openapi.yaml`.
