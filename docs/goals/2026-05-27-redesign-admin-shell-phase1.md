# Goal Plan — redesign-admin-shell-phase1

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.23+, html/template, htmx 1.9, Alpine.js 3.14, Tailwind CDN
- Estado atual: admin com top nav `max-w-6xl`, paleta `slate-*`, fonte system default. Templates: `layout.html`, `index.html`, `wizard.html`, `messages.html`, `dlq.html`, `settings.html`, `login.html`, `pair.html`
- Comando teste: `go test ./...`
- Comando lint: `go vet ./...` + `golangci-lint run`
- Comando build: `go build -o bridge.exe ./cmd/bridge`
- Tokens canônicos: `DESIGN.md` (raiz). Estratégia: Restrained, hue 264, accent 250
- Princípios: `PRODUCT.md` (raiz). Register `product`, continuidade com Chatwoot

## 2. Estado final mensurável

Fase 1 = **shell + tokens + Inter + sidebar funcional**. Páginas internas viram alvo nas fases 2-4. Critérios:

1. Arquivo novo `internal/bridge/web/templates/partials/sidebar.html` com nav vertical 240px contendo: marca (logo+nome), grupo nav (Painel, Tenants, Mensagens, DLQ, Configurações), user card bottom (email + logout)
2. `layout.html` reescrito:
   - `<html lang="pt-BR">` confirmado
   - `<link>` Google Fonts Inter com `display=swap`
   - Bloco `<style>` inline declarando TODAS as CSS vars de `DESIGN.md` (color, typography, spacing, radius, motion)
   - Tailwind CDN mantido (Fase 1 não migra build)
   - Body `display: grid; grid-template-columns: 240px 1fr` em ≥1024px
   - Inclusão de `{{template "sidebar" .}}` antes do `<main>`
   - `<main>` sem `max-w-6xl`, padding `var(--space-8) var(--space-8)`
   - Reduce-motion respeitado via `@media (prefers-reduced-motion)` zerando durações
3. Item ativo da sidebar usa `aria-current="page"` baseado em `.Title` ou path passado pelo handler (campo novo `ActivePath` em `web.PageData`)
4. Drawer mobile: hamburger button visível `<768px`, sidebar vira off-canvas controlada por Alpine `x-data="{open:false}"`
5. User card bottom mostra email do admin logado (já disponível em sessão), botão logout funcional
6. Páginas internas (`index.html`, `wizard.html`, `messages.html`, `dlq.html`, `settings.html`, `login.html`, `pair.html`) **NÃO modificadas além de remover container `max-w-6xl` herdado se quebrar layout** — refactor delas é Fase 2+
7. Login e Pair continuam SEM sidebar (excluídos do shell). `layout.html` recebe variável `.HideShell bool` e omite sidebar+grid quando true
8. Tests:
   - Snapshot (golden) de `layout.html` renderizado com fixture: novo, commitado
   - Test unit `TestSidebarActiveItem`: dado `ActivePath=/messages`, item Mensagens tem `aria-current="page"`, outros não
   - Test unit `TestLayoutHidesShellOnLogin`: dado `HideShell=true`, output não contém `<aside`
   - Test unit `TestLayoutIncludesInterFont`: output contém `fonts.googleapis.com/css2?family=Inter`
   - Test unit `TestLayoutDeclaresDesignTokens`: output contém `--bg-app` e `--accent`
   - Test unit existente (`dashboard_test.go`, `pairing_test.go`, etc) continuam verdes
9. `go test ./...` exits 0
10. `go vet ./... && golangci-lint run` exits 0
11. `go build` produz `bridge.exe`
12. Cobertura ≥80% nos arquivos novos (`partials/sidebar.html` é template, não conta; foco em `web.go` se alterado)

## 3. Prova surfaceável

Comandos a cada turno:
- `go test ./... 2>&1 | tail -20`
- `go vet ./... 2>&1 | tail -5`

Comandos no turno final:
- `golangci-lint run 2>&1 | tail -5`
- `go build -o bridge.exe ./cmd/bridge 2>&1 | tail -3`
- `ls internal/bridge/web/templates/partials/sidebar.html`
- `grep -c "fonts.googleapis.com" internal/bridge/web/templates/layout.html`
- `grep -c "\-\-bg-app\|\-\-accent" internal/bridge/web/templates/layout.html`
- `grep -c "aria-current" internal/bridge/web/templates/partials/sidebar.html`
- `grep -c "HideShell" internal/bridge/web/templates/layout.html`

Output literal esperado:
- `ok` em todos pacotes
- `--- FAIL` ausente
- Cada `grep -c` ≥ 1
- Build sem stderr
- `ls` retorna o path

Echo obrigatório: sim.

## 4. Restrições

### Específicas do projeto

- NÃO criar pacote Go novo. `partials/` é só diretório de templates
- NÃO modificar lógica de handlers além de adicionar campo `ActivePath string` e `HideShell bool` em `web.PageData` (ou equivalente)
- NÃO tocar em rotas (`web.go` mounting permanece)
- NÃO remover htmx ou Alpine
- NÃO usar Tailwind colors hardcoded (`bg-slate-*`, `text-emerald-*`, etc) em nenhum template novo/alterado — usar CSS vars de `DESIGN.md`
- NÃO usar `#000` ou `#fff` puros
- NÃO usar em dash (`—`) em copy de UI
- NÃO adicionar shadow em sidebar ou cards
- NÃO refatorar páginas internas (Fase 2+)
- NÃO criar build step novo (Tailwind CDN fica)
- NÃO adicionar dependências Go (`go.mod` intocado)
- NÃO modificar i18n keys existentes; pode adicionar novas em `i18n/pt-BR.json` para sidebar
- Tokens DESIGN.md viram source-of-truth. Qualquer cor/spacing inline DEVE referenciar `var(--*)`

### TDD obrigatório (code-craftsman)

- RED primeiro: snapshot/golden tests falhando antes de tocar template
- GREEN: implementação mínima pra passar
- REFACTOR: limpar mantendo verde
- Snapshot test inline ≤50 linhas (golden file externo permitido para layout completo se passar 50 linhas)
- 1 assert por test preferencial
- Coverage ratchet (nunca diminuir)

### Padrão (sempre)

- NÃO `--no-verify`
- NÃO desabilitar lint (`//nolint`)
- NÃO modificar `go.mod`/`go.sum`
- NÃO commitar segredos
- NÃO force-push
- Mensagens descritivas: `feat(ui)`, `test(ui)`, `refactor(ui)`, `style(ui)` (nunca `wip`/`update`)
- Sem suprimir warnings

## 5. Bound

- **25 turnos** — justificativa: shell + tokens + sidebar partial + drawer mobile + user card + 5 tests novos + golden snapshot. Páginas internas fora do escopo da Fase 1 limitam blast radius.

## 6. Modo de execução recomendado

- Auto mode: ligar antes do `/goal`
- Headless: opcional, `claude -p "/goal <condição>"`
- Branch: `feature/admin-shell-phase1`

## 7. Condição final (cole no /goal)

```
Refactor admin shell Fase 1 com TDD. Estado final: (1) partials/sidebar.html novo contendo marca, nav vertical (Painel, Tenants, Mensagens, DLQ, Configurações), user card bottom com email+logout; (2) layout.html reescrito com lang=pt-BR, link Google Fonts Inter display=swap, bloco style inline declarando todas CSS vars de DESIGN.md (--bg-app, --bg-sidebar, --fg-strong, --fg-default, --accent, --accent-hover, --border-subtle, --radius-md, --space-*, --dur-*, --ease-out), Tailwind CDN mantido, grid 240px 1fr em ≥1024px, main sem max-w-6xl, reduce-motion media query zerando durações; (3) campo ActivePath na PageData, sidebar marca item ativo com aria-current=page; (4) drawer mobile <768px via Alpine x-data, hamburger button; (5) user card mostra email admin logado e botão logout funcional; (6) variável HideShell na PageData, layout omite sidebar+grid quando true (usado em login.html e pair.html); (7) páginas internas index/wizard/messages/dlq/settings NÃO modificadas além de remover max-w-6xl herdado se quebrar; (8) tests novos: snapshot golden de layout.html renderizado, TestSidebarActiveItem (aria-current correto), TestLayoutHidesShellOnLogin (sem <aside quando HideShell), TestLayoutIncludesInterFont (output contém fonts.googleapis.com), TestLayoutDeclaresDesignTokens (output contém --bg-app e --accent); todos tests existentes continuam verdes. Provar com: `go test ./... 2>&1 | tail -20` mostrando `ok` em todos pacotes zero `--- FAIL`; `go vet ./... 2>&1 | tail -5` sem violação; turno final `golangci-lint run 2>&1 | tail -5` exits 0; `go build -o bridge.exe ./cmd/bridge 2>&1 | tail -3` sem stderr; `ls internal/bridge/web/templates/partials/sidebar.html` retorna o path; `grep -c "fonts.googleapis.com" internal/bridge/web/templates/layout.html` ≥1; `grep -c "\-\-bg-app\|\-\-accent" internal/bridge/web/templates/layout.html` ≥1; `grep -c "aria-current" internal/bridge/web/templates/partials/sidebar.html` ≥1; `grep -c "HideShell" internal/bridge/web/templates/layout.html` ≥1. TDD obrigatório: para cada test novo escrever RED primeiro (commitar), implementar mínimo GREEN (commitar), refatorar (commitar). Sem criar pacote Go novo, sem modificar lógica de handlers além de campos ActivePath e HideShell em PageData, sem tocar em rotas, sem remover htmx/Alpine, sem Tailwind colors hardcoded (bg-slate-*, text-emerald-*) em templates novos/alterados (usar CSS vars), sem #000 ou #fff puros, sem em dash em copy UI, sem shadow em sidebar/cards, sem refatorar páginas internas (fora escopo Fase 1), sem build step novo (Tailwind CDN fica), sem adicionar deps Go (go.mod intocado), sem //nolint, sem --no-verify, sem modificar go.mod/go.sum, sem force-push, sem commit vago (usar feat(ui)/test(ui)/refactor(ui)/style(ui)), sem commitar segredos, or stop after 25 turns. Report turn count, testes passando, files alterados e remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal Refactor admin shell Fase 1 com TDD. Estado final: (1) partials/sidebar.html novo contendo marca, nav vertical (Painel, Tenants, Mensagens, DLQ, Configurações), user card bottom com email+logout; (2) layout.html reescrito com lang=pt-BR, link Google Fonts Inter display=swap, bloco style inline declarando todas CSS vars de DESIGN.md (--bg-app, --bg-sidebar, --fg-strong, --fg-default, --accent, --accent-hover, --border-subtle, --radius-md, --space-*, --dur-*, --ease-out), Tailwind CDN mantido, grid 240px 1fr em ≥1024px, main sem max-w-6xl, reduce-motion media query zerando durações; (3) campo ActivePath na PageData, sidebar marca item ativo com aria-current=page; (4) drawer mobile <768px via Alpine x-data, hamburger button; (5) user card mostra email admin logado e botão logout funcional; (6) variável HideShell na PageData, layout omite sidebar+grid quando true (usado em login.html e pair.html); (7) páginas internas index/wizard/messages/dlq/settings NÃO modificadas além de remover max-w-6xl herdado se quebrar; (8) tests novos: snapshot golden de layout.html renderizado, TestSidebarActiveItem (aria-current correto), TestLayoutHidesShellOnLogin (sem <aside quando HideShell), TestLayoutIncludesInterFont (output contém fonts.googleapis.com), TestLayoutDeclaresDesignTokens (output contém --bg-app e --accent); todos tests existentes continuam verdes. Provar com: `go test ./... 2>&1 | tail -20` mostrando `ok` em todos pacotes zero `--- FAIL`; `go vet ./... 2>&1 | tail -5` sem violação; turno final `golangci-lint run 2>&1 | tail -5` exits 0; `go build -o bridge.exe ./cmd/bridge 2>&1 | tail -3` sem stderr; `ls internal/bridge/web/templates/partials/sidebar.html` retorna o path; `grep -c "fonts.googleapis.com" internal/bridge/web/templates/layout.html` ≥1; `grep -c "\-\-bg-app\|\-\-accent" internal/bridge/web/templates/layout.html` ≥1; `grep -c "aria-current" internal/bridge/web/templates/partials/sidebar.html` ≥1; `grep -c "HideShell" internal/bridge/web/templates/layout.html` ≥1. TDD obrigatório: para cada test novo escrever RED primeiro (commitar), implementar mínimo GREEN (commitar), refatorar (commitar). Sem criar pacote Go novo, sem modificar lógica de handlers além de campos ActivePath e HideShell em PageData, sem tocar em rotas, sem remover htmx/Alpine, sem Tailwind colors hardcoded (bg-slate-*, text-emerald-*) em templates novos/alterados (usar CSS vars), sem #000 ou #fff puros, sem em dash em copy UI, sem shadow em sidebar/cards, sem refatorar páginas internas (fora escopo Fase 1), sem build step novo (Tailwind CDN fica), sem adicionar deps Go (go.mod intocado), sem //nolint, sem --no-verify, sem modificar go.mod/go.sum, sem force-push, sem commit vago (usar feat(ui)/test(ui)/refactor(ui)/style(ui)), sem commitar segredos, or stop after 25 turns. Report turn count, testes passando, files alterados e remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal <colar string da seção 7>"
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars condição (medido ~3700)
- [x] Comandos concretos (`go test`, `go vet`, `golangci-lint`, `go build`, `grep`, `ls`)
- [x] Output literal (`ok`, `--- FAIL` ausente, grep ≥1, paths)
- [x] Restrições projeto (11) + padrão (7) + TDD (6)
- [x] Bound 25 turns
- [x] Echo obrigatório
- [x] Slug convenção (`redesign-<area>-<fase>`)
- [x] Salvo `docs/goals/`
- [x] Falsificável (exit + grep + ls)
- [x] Comandos verificados
- [x] TDD red-green-refactor exigido
- [x] Sem deps novas

## 11. Pré-condições antes de colar `/goal`

1. `PRODUCT.md` + `DESIGN.md` confirmados no repo (✓ feito nesta sessão)
2. Branch `feature/admin-shell-phase1` criada a partir de `master`
3. Auto mode ligado
4. Backup screenshot do admin atual (referência "antes")
5. Stack Chatwoot rodando em `localhost:3000` para referência visual

## 12. Pós-goal (manual)

- Smoke navegar todas rotas admin, verificar não quebrou
- Screenshot "depois" comparando com ref Chatwoot
- Commit final + merge para `master`
- Iniciar Fase 2 plan (`/impeccable shape` para `index.html` + `wizard.html`)
- Atualizar `CHANGELOG.md` Unreleased com nota UI

## 13. Roadmap Fases 2-5 (não cobertas neste goal)

- **Fase 2**: refactor páginas internas (index, messages, dlq, settings) — lista, badges, headers
- **Fase 3**: wizard com stepper vertical
- **Fase 4**: pair.html standalone redesign
- **Fase 5**: polish (empty states, toasts, skeletons, atalhos teclado, favicon)
