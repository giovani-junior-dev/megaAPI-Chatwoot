# Plano de Melhoria Frontend — Fases 2-5

Continuação do shell (Fase 1, mergeado em `feature/admin-shell-phase1`). Fase 1 entregou: sidebar, tokens CSS DESIGN.md, Inter, HideShell, drawer mobile. Páginas internas ainda intocadas (slate hardcoded, tabelas cruas).

## 0. Decisões travadas (sessão 2026-06-01)

1. **Tailwind CDN, sem build step.** Zero deps Go, zero npm. Componentes = padrões HTML puros adaptados aos CSS vars.
2. **Tabelas viram lista de cards <768px.** Desktop tabela limpa; mobile cada linha empilha (padrão `.list-item` DESIGN.md). Sem scroll horizontal.
3. **Ordem: Login + Painel (Fase 2) → Messages/DLQ (Fase 3) → Settings/Wizard (Fase 4) → Polish (Fase 5).**

## 1. Diagnóstico real (todas as páginas internas)

Violações DESIGN.md confirmadas por leitura dos templates:

| Página | Problemas |
|--------|-----------|
| `login.html` | `bg-white shadow-sm`, `bg-slate-900` botão, `text-red-600`, inputs `border-slate-300` crus. Sem identidade. Shadow proibido. |
| `index.html` (Painel) | `<table>` crua `bg-white`, `text-emerald-700`, botão `bg-slate-900` (devia ser accent), empty state = `<p>` simples. Tabela quebra no mobile. |
| `messages.html` | `<table>` crua, filtro sem estilo, paginação `underline` texto puro, `bg-slate-100` thead. Sem badges direção/status. |
| `dlq.html` | `<table>` crua, `text-red-700` erro, botão Retry `bg-slate-900`. Sem confirmação no Retry (Nielsen #5 error prevention). |
| `settings.html` | `bg-white border rounded p-4`, botão slate. Form OK-ish mas fora dos tokens. `text-green-700` saved. |
| `wizard.html` | Stepper inexistente (só `passo X de 4` texto). Inputs crus `border rounded`. `bg-green-700` no submit. Sem indicador visual de progresso (DESIGN.md tem `.stepper` spec). |

Comum a todas: cores Tailwind hardcoded (banido), sem estados loading/erro desenhados, botões fora do accent, tipografia raw (`text-2xl font-semibold`) em vez de escala DESIGN.md.

## 2. Estratégia: partials reutilizáveis

Antes de tocar páginas, criar partials de componente em `templates/partials/` (escopo já liberado na Fase 1):

- `partials/button.html` — `{{define "btn-primary"}}` / `btn-ghost` / `btn-danger` (accent, radius-md, transition)
- `partials/badge.html` — success/warning/danger/neutral (status mensagens, pareado)
- `partials/empty.html` — empty state centralizado com ilustração SVG inline + CTA
- `partials/datatable.html` — wrapper responsivo: `<table>` desktop, `.list-item` cards mobile (CSS-only via media query, mesma marcação semântica)

Componentes viram classes CSS no `<style>` do `layout.html` (`.btn-primary`, `.badge`, `.list-item`, `.data-table`) já especificadas em DESIGN.md seção Components. Reuso = uma definição, N páginas.

## 3. Fases

### Fase 2 — Login + Painel (ESTA, vira /goal)

**Login** (`login.html`):
- Layout split ou centrado com marca, fundo `--bg-app`, card `--bg-elev` + `--border-1` (sem shadow)
- Inputs `.input` (DESIGN.md), label `--fg-default`, botão `.btn-primary` accent full-width
- Erro inline `--danger` com ícone, near field (Nielsen #9)
- Email autofocus, `autocomplete`, input type correto
- Mobile: padding `--space-4`, touch target ≥44px, body ≥16px (sem zoom iOS)

**Painel** (`index.html`):
- Header com título escala `--text-xl` + botão `.btn-primary` "Novo tenant"
- Tabela tenants via `partials/datatable.html`: colunas Slug / Mensagens 24h / Status pareamento / ações
- Status pareado vira `.badge-success` (com JID) ou `.badge-neutral` ("não pareado") — não cor pura
- Mobile <768px: cada tenant vira card (slug título, contagem + badge, ações em linha)
- Empty state via `partials/empty.html`: ilustração + "Nenhum tenant" + CTA

**Tests (TDD, padrão Fase 1)**:
- `TestLoginUsesDesignTokens` — output sem `bg-slate`/`shadow`, contém `.input`/`.btn-primary`
- `TestPainelTableHasResponsiveMarkup` — contém `.data-table` + classe card mobile
- `TestPainelPairedShowsBadge` — tenant pareado renderiza `.badge`
- `TestEmptyStateRendersCTA` — sem tenants → empty partial + link
- Golden snapshot login + painel renderizados
- Todos tests Fase 1 + suite continuam verdes (264+)

### Fase 3 — Messages + DLQ
- Reusa `datatable` partial
- Badges direção (inbound/outbound) + status (delivered/sent/failed)
- Paginação vira `.btn-ghost` com ícones, não underline
- DLQ Retry ganha confirmação (Alpine `x-data` confirm inline, não modal) + estado loading htmx
- Filtro de tenant com `.input` + datalist de slugs conhecidos (recognition > recall, Nielsen #6)

### Fase 4 — Settings + Wizard
- Settings: form `.input`/`.btn-primary`, toast "Salvo" (DESIGN.md `.toast`) em vez de `<p>` verde
- Wizard: stepper vertical real (`.stepper` DESIGN.md), bullets active/completed/upcoming, validação por passo antes de avançar, review final com resumo dos campos

### Fase 5 — Polish
- Toasts globais (htmx events)
- Skeletons em listas (loading >300ms)
- Favicon + meta
- Atalhos teclado (g+d painel, g+m mensagens)
- `prefers-reduced-motion` audit final
- Verificação a11y: foco visível, tab order, contraste AA em todos os badges

## 4. Restrições (herdadas Fase 1 + novas)

- Sem build step, sem npm, `go.mod` intocado
- Sem Tailwind colors hardcoded — só CSS vars
- Sem `#000`/`#fff`, sem em dash, sem shadow em card/sidebar
- Sem React/componentes de lib externa (catálogo ui-libraries-curator é React, incompatível)
- Componentes HTML adaptados de HyperUI/Flowbite/Preline/Meraki como REFERÊNCIA, classes próprias (CSS vars), nunca colar `bg-slate-*` deles
- TDD: RED (golden/unit falhando) → GREEN → REFACTOR, 1 commit cada
- Handlers: só dados de view, sem nova lógica de rota
- Páginas fora da fase atual não mudam

## 5. Bound sugerido por fase

- Fase 2: 25 turns (partials base + login + painel + 5 tests + golden)
- Fase 3: 20 turns
- Fase 4: 20 turns
- Fase 5: 15 turns

## 6. Prova surfaceável (toda fase)

- `go test ./... 2>&1 | tail -20` → `ok` todos pacotes, zero `--- FAIL`
- `go vet ./... 2>&1 | tail -5` → limpo
- `golangci-lint run` → exit 0
- `go build -o bridge.exe ./cmd/bridge` → sem stderr
- `grep -c "bg-slate" templates/<pagina>.html` → `0` (na página da fase)
- Echo obrigatório de cada comando

## 7. Live test (manual, pós cada fase)

- `docker compose build bridge && docker compose up -d bridge` (lição CLAUDE.md: rebuild sempre)
- Abrir `localhost:8090`, login `admin@bridge.local`
- Testar largura 360 / 768 / 1024 / 1440
- Verificar foco teclado + contraste

## 8. Fontes de componente compatíveis (HTML puro)

| Fonte | Uso | URL |
|-------|-----|-----|
| HyperUI | Tabelas, forms, badges, auth | hyperui.dev |
| Flowbite HTML | Sidebar, drawer, tables | flowbite.com |
| Preline UI | App shells, overlays (+Alpine) | preline.co |
| Meraki UI | Auth screens | merakiui.com |

Referência de padrão apenas. Markup adaptado aos nossos tokens.
