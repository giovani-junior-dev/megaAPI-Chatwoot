# Goal Plan — redesign-admin-shell-phase1 v2

## 0. Lições da versão anterior

- v1 (`2026-05-27-redesign-admin-shell-phase1.md`) foi commitado mas **não executado** — usuário não colou `/goal` na sessão anterior.
- Condição idêntica é re-disparada nesta sessão. Conteúdo, restrições, prova e bound permanecem válidos.
- Ajuste único v2: reforçar lembrete operacional de **branch feature/admin-shell-phase1 antes do paste** + auto mode ligado.
- Sem mudanças de escopo, comandos ou restrições.

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.25, html/template, htmx 1.9, Alpine.js 3.14, Tailwind CDN
- Estado atual `layout.html` (28 linhas): top nav `max-w-6xl`, paleta `slate-*`, sem Inter, sem CSS vars, sem sidebar, sem `HideShell`/`ActivePath`
- Templates: `layout.html`, `index.html`, `wizard.html`, `messages.html`, `dlq.html`, `settings.html`, `login.html`, `pair.html`
- Comando teste: `go test ./...`
- Comando lint: `go vet ./...` + `golangci-lint run`
- Comando build: `go build -o bridge.exe ./cmd/bridge`
- Tokens canônicos: `DESIGN.md` raiz. Estratégia Restrained, hue 264, accent 250
- Princípios: `PRODUCT.md` raiz. Register `product`, continuidade com Chatwoot

## 2. Estado final mensurável

Fase 1 = shell + tokens + Inter + sidebar funcional. Critérios:

1. `internal/bridge/web/templates/partials/sidebar.html` existe — marca (logo+nome), nav vertical (Painel, Tenants, Mensagens, DLQ, Configurações), user card bottom (email + logout)
2. `layout.html` reescrito:
   - `<html lang="pt-BR">`
   - `<link>` Google Fonts Inter `display=swap`
   - `<style>` inline declarando CSS vars de DESIGN.md (color, typography, spacing, radius, motion)
   - Tailwind CDN mantido
   - Body grid `240px 1fr` em ≥1024px
   - `{{template "sidebar" .}}` antes do `<main>`
   - `<main>` sem `max-w-6xl`, padding `var(--space-8) var(--space-8)`
   - `@media (prefers-reduced-motion)` zerando durações
3. `ActivePath` em `web.PageData` + sidebar marca item ativo com `aria-current="page"`
4. Drawer mobile `<768px` via Alpine `x-data="{open:false}"` + hamburger
5. User card bottom mostra email admin logado + botão logout funcional
6. `HideShell bool` em `PageData`; layout omite sidebar+grid quando true (`login.html`, `pair.html`)
7. Páginas internas NÃO modificadas além de remover `max-w-6xl` herdado se quebrar
8. Tests novos:
   - Snapshot golden de `layout.html` renderizado
   - `TestSidebarActiveItem` — `aria-current` correto
   - `TestLayoutHidesShellOnLogin` — sem `<aside` quando `HideShell`
   - `TestLayoutIncludesInterFont` — output contém `fonts.googleapis.com`
   - `TestLayoutDeclaresDesignTokens` — output contém `--bg-app` e `--accent`
9. Tests existentes verdes
10. `go test ./...` exits 0
11. `go vet ./... && golangci-lint run` exits 0
12. `go build` produz `bridge.exe`

## 3. Prova surfaceável

Comandos cada turno:
- `go test ./... 2>&1 | tail -20`
- `go vet ./... 2>&1 | tail -5`

Turno final:
- `golangci-lint run 2>&1 | tail -5`
- `go build -o bridge.exe ./cmd/bridge 2>&1 | tail -3`
- `ls internal/bridge/web/templates/partials/sidebar.html`
- `grep -c "fonts.googleapis.com" internal/bridge/web/templates/layout.html`
- `grep -c "\-\-bg-app\|\-\-accent" internal/bridge/web/templates/layout.html`
- `grep -c "aria-current" internal/bridge/web/templates/partials/sidebar.html`
- `grep -c "HideShell" internal/bridge/web/templates/layout.html`

Output literal esperado:
- `ok` em todos pacotes; `--- FAIL` ausente
- Cada `grep -c` ≥ 1
- Build sem stderr
- `ls` retorna path

Echo obrigatório: sim.

## 4. Restrições

### Projeto
- NÃO criar pacote Go novo
- NÃO modificar handlers além de `ActivePath string` e `HideShell bool` em `PageData`
- NÃO tocar em rotas
- NÃO remover htmx/Alpine
- NÃO Tailwind colors hardcoded (`bg-slate-*`, `text-emerald-*`) em templates novos/alterados — usar CSS vars
- NÃO `#000`/`#fff` puros
- NÃO em dash em copy UI
- NÃO shadow em sidebar/cards
- NÃO refatorar páginas internas (Fase 2+)
- NÃO build step novo (Tailwind CDN fica)
- NÃO adicionar deps Go (`go.mod` intocado)
- Tokens DESIGN.md = source of truth, sempre `var(--*)`

### TDD code-craftsman
- RED primeiro, commit
- GREEN mínimo, commit
- REFACTOR, commit
- Snapshot inline ≤50 linhas (golden externo permitido se exceder)

### Padrão
- NÃO `--no-verify`
- NÃO `//nolint`
- NÃO modificar `go.mod`/`go.sum`
- NÃO commitar segredos
- NÃO force-push
- Mensagens descritivas: `feat(ui)`, `test(ui)`, `refactor(ui)`, `style(ui)`

## 5. Bound

- **25 turns** — shell + tokens + sidebar partial + drawer + user card + 5 tests + golden. Páginas internas fora limitam blast radius.

## 6. Modo

- Auto mode: ligar antes
- Headless: opcional
- Branch: `feature/admin-shell-phase1`

## 7. Condição final (cole no /goal)

```
Refactor admin shell Fase 1 com TDD. Estado final: (1) partials/sidebar.html novo contendo marca, nav vertical (Painel, Tenants, Mensagens, DLQ, Configurações), user card bottom com email+logout; (2) layout.html reescrito com lang=pt-BR, link Google Fonts Inter display=swap, bloco style inline declarando todas CSS vars de DESIGN.md (--bg-app, --bg-sidebar, --fg-strong, --fg-default, --accent, --accent-hover, --border-subtle, --radius-md, --space-*, --dur-*, --ease-out), Tailwind CDN mantido, grid 240px 1fr em ≥1024px, main sem max-w-6xl, reduce-motion media query zerando durações; (3) campo ActivePath na PageData, sidebar marca item ativo com aria-current=page; (4) drawer mobile <768px via Alpine x-data, hamburger button; (5) user card mostra email admin logado e botão logout funcional; (6) variável HideShell na PageData, layout omite sidebar+grid quando true (usado em login.html e pair.html); (7) páginas internas index/wizard/messages/dlq/settings NÃO modificadas além de remover max-w-6xl herdado se quebrar; (8) tests novos: snapshot golden de layout.html renderizado, TestSidebarActiveItem (aria-current correto), TestLayoutHidesShellOnLogin (sem <aside quando HideShell), TestLayoutIncludesInterFont (output contém fonts.googleapis.com), TestLayoutDeclaresDesignTokens (output contém --bg-app e --accent); todos tests existentes continuam verdes. Provar com: `go test ./... 2>&1 | tail -20` mostrando `ok` em todos pacotes zero `--- FAIL`; `go vet ./... 2>&1 | tail -5` sem violação; turno final `golangci-lint run 2>&1 | tail -5` exits 0; `go build -o bridge.exe ./cmd/bridge 2>&1 | tail -3` sem stderr; `ls internal/bridge/web/templates/partials/sidebar.html` retorna o path; `grep -c "fonts.googleapis.com" internal/bridge/web/templates/layout.html` ≥1; `grep -c "\-\-bg-app\|\-\-accent" internal/bridge/web/templates/layout.html` ≥1; `grep -c "aria-current" internal/bridge/web/templates/partials/sidebar.html` ≥1; `grep -c "HideShell" internal/bridge/web/templates/layout.html` ≥1. TDD obrigatório: para cada test novo escrever RED primeiro (commitar), implementar mínimo GREEN (commitar), refatorar (commitar). Sem criar pacote Go novo, sem modificar lógica de handlers além de campos ActivePath e HideShell em PageData, sem tocar em rotas, sem remover htmx/Alpine, sem Tailwind colors hardcoded (bg-slate-*, text-emerald-*) em templates novos/alterados (usar CSS vars), sem #000 ou #fff puros, sem em dash em copy UI, sem shadow em sidebar/cards, sem refatorar páginas internas (fora escopo Fase 1), sem build step novo (Tailwind CDN fica), sem adicionar deps Go (go.mod intocado), sem //nolint, sem --no-verify, sem modificar go.mod/go.sum, sem force-push, sem commit vago (usar feat(ui)/test(ui)/refactor(ui)/style(ui)), sem commitar segredos, or stop after 25 turns. Report turn count, testes passando, files alterados e remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal Refactor admin shell Fase 1 com TDD. Estado final: (1) partials/sidebar.html novo contendo marca, nav vertical (Painel, Tenants, Mensagens, DLQ, Configurações), user card bottom com email+logout; (2) layout.html reescrito com lang=pt-BR, link Google Fonts Inter display=swap, bloco style inline declarando todas CSS vars de DESIGN.md (--bg-app, --bg-sidebar, --fg-strong, --fg-default, --accent, --accent-hover, --border-subtle, --radius-md, --space-*, --dur-*, --ease-out), Tailwind CDN mantido, grid 240px 1fr em ≥1024px, main sem max-w-6xl, reduce-motion media query zerando durações; (3) campo ActivePath na PageData, sidebar marca item ativo com aria-current=page; (4) drawer mobile <768px via Alpine x-data, hamburger button; (5) user card mostra email admin logado e botão logout funcional; (6) variável HideShell na PageData, layout omite sidebar+grid quando true (usado em login.html e pair.html); (7) páginas internas index/wizard/messages/dlq/settings NÃO modificadas além de remover max-w-6xl herdado se quebrar; (8) tests novos: snapshot golden de layout.html renderizado, TestSidebarActiveItem (aria-current correto), TestLayoutHidesShellOnLogin (sem <aside quando HideShell), TestLayoutIncludesInterFont (output contém fonts.googleapis.com), TestLayoutDeclaresDesignTokens (output contém --bg-app e --accent); todos tests existentes continuam verdes. Provar com: `go test ./... 2>&1 | tail -20` mostrando `ok` em todos pacotes zero `--- FAIL`; `go vet ./... 2>&1 | tail -5` sem violação; turno final `golangci-lint run 2>&1 | tail -5` exits 0; `go build -o bridge.exe ./cmd/bridge 2>&1 | tail -3` sem stderr; `ls internal/bridge/web/templates/partials/sidebar.html` retorna o path; `grep -c "fonts.googleapis.com" internal/bridge/web/templates/layout.html` ≥1; `grep -c "\-\-bg-app\|\-\-accent" internal/bridge/web/templates/layout.html` ≥1; `grep -c "aria-current" internal/bridge/web/templates/partials/sidebar.html` ≥1; `grep -c "HideShell" internal/bridge/web/templates/layout.html` ≥1. TDD obrigatório: para cada test novo escrever RED primeiro (commitar), implementar mínimo GREEN (commitar), refatorar (commitar). Sem criar pacote Go novo, sem modificar lógica de handlers além de campos ActivePath e HideShell em PageData, sem tocar em rotas, sem remover htmx/Alpine, sem Tailwind colors hardcoded (bg-slate-*, text-emerald-*) em templates novos/alterados (usar CSS vars), sem #000 ou #fff puros, sem em dash em copy UI, sem shadow em sidebar/cards, sem refatorar páginas internas (fora escopo Fase 1), sem build step novo (Tailwind CDN fica), sem adicionar deps Go (go.mod intocado), sem //nolint, sem --no-verify, sem modificar go.mod/go.sum, sem force-push, sem commit vago (usar feat(ui)/test(ui)/refactor(ui)/style(ui)), sem commitar segredos, or stop after 25 turns. Report turn count, testes passando, files alterados e remaining bound each turn. Claude must echo full output of each verification command.
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars (medido ~3800)
- [x] Comandos concretos (`go test`, `go vet`, `golangci-lint`, `go build`, `grep`, `ls`)
- [x] Output literal (`ok`, `--- FAIL` ausente, grep ≥1, paths)
- [x] Restrições projeto (12) + padrão (6) + TDD (4)
- [x] Bound 25 turns
- [x] Echo obrigatório
- [x] Slug convenção
- [x] Salvo `docs/goals/`
- [x] Falsificável
- [x] Comandos verificados (`make test/lint/build` existem em Makefile)
- [x] TDD red-green-refactor
- [x] Sem deps novas
- [x] v2 com seção "Lições da versão anterior"

## 11. Pré-condições antes de colar `/goal`

1. ✅ `PRODUCT.md` + `DESIGN.md` no repo
2. ⚠️ Criar branch: `git checkout -b feature/admin-shell-phase1`
3. ⚠️ Auto mode ligado (toggle antes do paste)
4. Opcional: screenshot do admin atual como ref "antes"

## 12. Pós-goal (manual)

- Smoke navegar rotas admin
- Screenshot "depois"
- Commit final + merge para `master`
- Iniciar Fase 2 plan
- Atualizar `CHANGELOG.md` Unreleased
