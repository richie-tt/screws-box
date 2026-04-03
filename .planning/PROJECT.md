# Screws Box

## What This Is

Aplikacja webowa do zarządzania organizerem na drobne elementy złączne (śruby, podkładki, nakrętki itp.). Prezentuje półkę jako konfigurowalną siatkę pojemników (np. 5x10), umożliwia dodawanie elementów z tagami/kategoriami i wyszukiwanie po nazwie lub tagu z wizualnym podświetleniem pozycji na siatce. Dostępna w sieci domowej.

## Core Value

Szybkie znalezienie pozycji pojemnika (np. "3B") po wpisaniu nazwy lub tagu elementu.

## Requirements

### Validated

- [x] Dostęp sieciowy (nie tylko localhost) — Validated in Phase 1: Project Skeleton
- [x] Dane przechowywane w SQLite — Validated in Phase 2: Database Foundation
- [x] Element ma nazwę i wiele tagów/kategorii — Validated in Phase 7: Tag Autocomplete (autocomplete prevents fragmentation)

### Active

- [ ] Konfigurowalna siatka półki (np. 5x10 = 10 kolumn, 5 rzędów)
- [ ] Wizualna reprezentacja siatki jako szachownica z oznaczeniami (1A, 1B... 5J)
- [ ] Dodawanie elementów do pojemnika przez kliknięcie w siatkę
- [x] Element ma nazwę i wiele tagów/kategorii (np. "m6", "sprężynowa", "falista", "powiększona") — Validated in Phase 5-7
- [ ] Jeden pojemnik może zawierać wiele różnych elementów
- [x] Wyszukiwanie tekstowe po nazwie lub tagu — Validated in Phase 8: Search Backend (GET /api/search?q=... with name LIKE + exact tag match)
- [x] Wyniki wyszukiwania jako lista z pozycjami (np. "3B") — Validated in Phase 8: Search Backend (ItemResponse includes container_label)
- [ ] Wizualne podświetlenie pasujących pojemników na siatce podczas wyszukiwania
### Out of Scope

- Zarządzanie ilościami/inwentaryzacja — nie potrzebne w v1
- Wiele półek — jedna półka wystarczy na start
- Autentykacja/logowanie — aplikacja w sieci domowej, zaufane środowisko
- Aplikacja mobilna — web responsywny wystarczy

## Context

- Użytkownik ma fizyczny organizer z pojemnikami ułożonymi w siatkę
- Pojemniki zawierają różne drobne elementy złączne (śruby, podkładki, nakrętki różnych typów)
- Główny problem: "gdzie jest M6 sprężynowa?" — trzeba przeszukiwać pojemniki ręcznie
- Aplikacja rozwiązuje to — wpisujesz tag, dostajesz pozycję na siatce
- Adresowanie: kolumny = cyfry (1, 2, 3...), rzędy = litery (A, B, C...) → pozycja "3B" = kolumna 3, rząd B

## Constraints

- **Stack**: Go (Golang) backend, plain HTML frontend (plugin frontend-design do projektowania UI), SQLite baza danych
- **Frontend design**: Użyć pluginu `frontend-design` do zaprojektowania interfejsu
- **Deployment**: Dostępna w sieci domowej (nasłuch na 0.0.0.0, nie 127.0.0.1)
- **Simplicity**: Jeden binary Go + plik SQLite, minimalna konfiguracja

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go + SQLite + plain HTML | Prosty stack, jeden binary, zero zależności | — Pending |
| Jedna półka w v1 | Prostota, użytkownik potrzebuje jednej | — Pending |
| Tagi zamiast kategorii hierarchicznych | Elastyczność — element może mieć wiele tagów | — Pending |
| Adresowanie: cyfra+litera (3B) | Intuicyjne, jak szachownica / arkusz kalkulacyjny | — Pending |
| Plugin frontend-design dla UI | Lepszy design frontendu | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-03 after Phase 8 completion*
