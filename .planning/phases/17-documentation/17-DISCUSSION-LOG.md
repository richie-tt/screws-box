# Phase 17: Documentation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-06
**Phase:** 17-documentation
**Areas discussed:** Structure & depth, Audience & tone, Env var documentation style, Deployment examples

---

## Structure & depth

| Option | Description | Selected |
|--------|-------------|----------|
| Single README.md | One file with clear sections | ✓ |
| README + separate docs/ | Short README with links to detailed docs/ folder | |
| You decide | Claude picks | |

**User's choice:** Single README.md
**Notes:** Keeps everything discoverable in one place.

| Option | Description | Selected |
|--------|-------------|----------|
| Concise | Key workflows only, 1-2 sentences per feature | |
| Thorough | Step-by-step walkthroughs with examples for each feature | ✓ |
| You decide | Claude picks appropriate depth | |

**User's choice:** Thorough

| Option | Description | Selected |
|--------|-------------|----------|
| ASCII diagrams only | Text-based grid representation | |
| Screenshots + ASCII | Actual screenshots plus ASCII for coordinates | ✓ |
| No visuals | Text descriptions only | |
| You decide | Claude picks | |

**User's choice:** Screenshots + ASCII

| Option | Description | Selected |
|--------|-------------|----------|
| Setup first | Overview -> Quick Start -> Config -> Usage -> Admin -> Dev | ✓ |
| Features first | Overview -> Features -> Quick Start -> Config -> Dev | |
| You decide | Claude picks | |

**User's choice:** Setup first

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, linked TOC | Markdown TOC with anchor links | ✓ |
| No TOC | Sections self-explanatory from scrolling | |
| You decide | Claude decides based on length | |

**User's choice:** Yes, linked TOC

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, brief FAQ | Common issues: port conflicts, SQLite, OIDC, Redis | ✓ |
| No FAQ | Setup and usage only | |
| You decide | Claude includes if pitfalls exist | |

**User's choice:** Yes, brief FAQ

---

## Audience & tone

| Option | Description | Selected |
|--------|-------------|----------|
| English only | Consistent with UI text convention | ✓ |
| Bilingual (EN + PL) | English primary with Polish notes | |
| Polish only | Match PROJECT.md language | |

**User's choice:** English only

| Option | Description | Selected |
|--------|-------------|----------|
| Self-hosting user | Focus on install, configure, use | ✓ |
| Developer first | Focus on architecture, dev setup, testing | |
| Both equally | Full coverage for both audiences | |

**User's choice:** Self-hosting user

| Option | Description | Selected |
|--------|-------------|----------|
| Friendly practical | Warm but concise, no jargon | ✓ |
| Minimal technical | Just the facts | |
| You decide | Claude picks | |

**User's choice:** Friendly practical

| Option | Description | Selected |
|--------|-------------|----------|
| Personal project framing | "Built for managing a hardware organizer at home" | |
| Neutral/professional | General-purpose tool presentation | ✓ |
| You decide | Claude picks | |

**User's choice:** Neutral/professional

---

## Env var documentation style

| Option | Description | Selected |
|--------|-------------|----------|
| Table + .env.example | Markdown table PLUS committed .env.example file | ✓ |
| Table only | Markdown table in README | |
| Prose with code blocks | Each var explained in paragraph | |
| You decide | Claude picks | |

**User's choice:** Table + .env.example

| Option | Description | Selected |
|--------|-------------|----------|
| By feature | Groups: Core, Sessions, OIDC | ✓ |
| Alphabetical | Flat list A-Z | |
| You decide | Claude picks | |

**User's choice:** By feature

---

## Deployment examples

| Option | Description | Selected |
|--------|-------------|----------|
| Binary + systemd | go build, systemd unit for auto-start | ✓ |
| Docker + compose | Dockerfile and docker-compose.yml | ✓ |
| Plain binary only | Just go build && ./screwsbox | |
| Reverse proxy examples | Nginx/Caddy config snippets | |

**User's choice:** Binary + systemd AND Docker + compose

| Option | Description | Selected |
|--------|-------------|----------|
| Include Dockerfile | Multi-stage Dockerfile in repo root | ✓ |
| Document only | Example in README but not committed | |
| No Docker | Skip Docker entirely | |

**User's choice:** Include Dockerfile

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, with profiles | Redis as optional Docker Compose profile | ✓ |
| Yes, always included | Redis always in compose | |
| No Redis in compose | Document Redis separately | |

**User's choice:** Yes, with profiles

| Option | Description | Selected |
|--------|-------------|----------|
| Authelia example | Step-by-step Authelia OIDC config | ✓ |
| Generic OIDC only | Document env vars and flow only | |
| Multiple providers | Authelia + Google examples | |

**User's choice:** Authelia example

---

## Claude's Discretion

- FAQ entry wording and selection
- Screenshot selection and placement
- Systemd unit file details
- Development section depth

## Deferred Ideas

None
