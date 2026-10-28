# CSM AI Integration Service

Go HTTP server (`net/http`, Go 1.26+) that holds **every call the CSM platform makes to Claude**. Today
that is dashboard generation for the CSM portal; other AI features join under `/ai/`. It is the only
component with a Claude client, the Anthropic SDK and the model credentials: the CSM portal backend
(`apps/csm-portal/backend`) must hold none of them.

## What lives here and what does not

| Here | In the CSM portal backend |
|---|---|
| The Claude client (gateway base URL, API key, model, timeout, retries) | Authentication and permissions of the end user (`PermWrite` for generation) |
| The catalogues the model reads (built-in copies; a Choreo file mount overrides each) | The routes the webapp calls (`/dashboards/generate`, `/dashboards/generate-code`) |
| Building the prompt (catalogue + the team list the backend sends + prompt + current draft) | Validating what the model wrote (`dashboard.ParseGenerated`, the per-type rules) |
| The call, its timeout, and reading the answer (the JSON object or source, a decline, a refusal, a truncation) | The team registry and the team key -> id / name swap; saving and sharing |
| Mapping those outcomes to a stable error vocabulary | The sample generators used when this service is not configured |

The backend sends each request's **team list** (from its `CSM_TEAM_REGISTRY`); this service never reads
the registry itself, so it is configured in one place.

## Routes

| Route | Status |
|---|---|
| `GET /health` | liveness, always 200 |
| `POST /ai/dashboards/generate` | the JSON dashboard generator (`internal/dashboards`, `internal/handler`) |
| `POST /ai/dashboards/generate-code` | the code approach, phase 2 (`internal/dashboards/code.go`) |

### `POST /ai/dashboards/generate`

Body `{prompt, draft?, teams}`: the prompt (trimmed, 1-2000 characters), the dashboard being edited in the
definition file format (the backend's `draftToDefinition`; absent or null on a first message) and the teams
a dashboard can filter by (`{key, name, family, field}`, only teams with a group). Unknown fields are a 400.

The service makes one call and returns **every JSON object in the reply, in order** (`candidates`), or the
model's sentence when its last object is the catalogue's refusal shape `{"error": "..."}` (`declined`), plus
`usage`. It does **not** judge the candidates: the backend runs `dashboard.ParseGenerated` from the last one
back and keeps the first that passes, so the per-type rules live in one place. Errors are `{code, message}`;
the backend maps the code to its own status and user-facing text:

| Status | `code` | When |
|---|---|---|
| 400 | `invalid_request` | the body, prompt, draft or teams |
| 503 | `not_configured` | no `ANTHROPIC_API_KEY` |
| 503 | `catalogue_unavailable` | a mounted catalogue that has gone or become empty |
| 422 | `refused` | the model refused (stop reason `refusal`) |
| 502 | `truncated` / `unusable` | the reply was cut off / holds nothing usable (no JSON object; for code, no usable code block) |
| 504 | `timeout` | the model call ran out of time (`CLAUDE_TIMEOUT`) |
| 502 | `model_call_failed` | any other call failure; a gateway 401/403 is this service's key problem, never passed on |

### `POST /ai/dashboards/generate-code`

Body `{prompt, current?}`: the prompt (as above) and the source being edited (at most 60,000 characters;
absent or "" on a first message). No team list: the code catalogue has no Teams section. The reply checks
are all here, because they only read the code catalogue's output contract (one fenced block, the last one
wins, it must define `function render(`, at most 60,000 characters; a reply with no block that starts with
`CANNOT:` is the model declining). 200 is `{source, reply, usage}` or `{declined, usage}`; errors and codes
are the table above.

## Model settings

`ANTHROPIC_API_KEY` (a secret) and `CLAUDE_BASE_URL` go together: the key is the CSM AI gateway's, so
**`CLAUDE_BASE_URL` is required whenever the key is set**, and a key without it fails the start
(`loadConfig`). There is deliberately no fallback to the Anthropic API's own address: with the gateway's
key it would start looking healthy and fail every call. Without a key the service still starts with model
calls off (the AI routes answer 503 `not_configured`), so a local stack needs no secrets. In Choreo both
are set per environment in Configs & Secrets (the key as a secret).

## Auth model

- **Inbound: no authentication in the application.** The Choreo endpoint is **Organization**-visible
  (`.choreo/component.yaml`), and the only caller is the CSM portal backend, which uses the platform's
  shared OAuth2 client-credentials app (the same pattern as `csm-notification-service`). The backend has
  already authenticated and authorised the end user. Do not make the endpoint Public, and do not add
  end-user token validation here without confirming that assumption has changed.
- **No end-user identity is forwarded or needed.** The model call is the same for every user.

## Catalogues

The model is given a catalogue: what a dashboard definition may contain, written for the model. There are
two, in `internal/catalogue`: `catalogue.md` (a dashboard definition) and `code_catalogue.md` (a dashboard
written as JavaScript). Each is **built into the binary** as the default and can be **overridden by a file**
that Choreo mounts (Configs & Secrets -> file mount), named by `CATALOGUE_FILE` / `CODE_CATALOGUE_FILE`.
A file is checked at startup (missing or empty fails the start, so a mistyped path shows there) and
**re-read on every request**, so an edited mount applies without a restart; a file that goes missing
later is an error, never a silent fall back to the built-in copy. The startup log names each one's origin
(`built-in` or the path). Keep the built-in copies current: they are what local runs and tests use, and
what a deployment without the mount gets.

**Editing `catalogue.md` can break a backend test, by design.** The backend's
`TestParseGenerated_TheCatalogueExampleIsValid` (`apps/csm-portal/backend/internal/dashboard`) reads this
file and runs its section 10 example through the backend's own check, so the catalogue and the rules
that judge the model's output cannot drift apart. After editing it, run the backend's
`go test ./internal/dashboard`. In the local stack, an edit to a built-in catalogue needs this service's
image rebuilt (`docker compose up -d --build csm-ai-integration-service`), not the backend's.

The Teams section is not in either file: the backend sends the team list with each request and
`internal/teams` appends it to the catalogue (the same text the backend wrote before the move).

## Conventions

- Middleware chain: `SecurityHeaders -> CorrelationID -> Logger -> Mux`, as in `csm-integration-service`.
  Correlation IDs carry the `cai-` prefix.
- **No prompts, drafts or model answers in logs.** They are users' text; log lengths, token counts,
  outcome and duration only.
- The server's `WriteTimeout` follows the model's: `CLAUDE_TIMEOUT` plus a minute, never under two minutes
  (`writeTimeout` in `cmd/server/main.go`), because a generate call waits for the model. The backend's own
  client timeout (`CSM_AI_INTEGRATION_SERVICE_TIMEOUT`, 55 s) must stay above `CLAUDE_TIMEOUT` (45 s).
- Update `openapi.yaml` with every route change.

## Commands

```bash
make test    # vet + race-detector tests
make build   # tests, then compiles ./cmd/server
```

Run gosec and govulncheck before a PR, as for the other Go services.
