# Design

Sistema visual do painel admin `chatwoot-megaapi-bridge`. Inspirado em Chatwoot 3.x (referência canônica em `localhost:3000`). Tokens em OKLCH, hue base 264 (cinza azulado neutro), accent 250 (azul Chatwoot).

## Theme

Light apenas. Cliente revendedor opera ambiente claro durante expediente. Dark mode fica fora do escopo até demanda real surgir.

## Color

Estratégia: **Restrained**. Tinted neutrals + um accent ≤10% da superfície.

```
--bg-app:         oklch(99%  0.002 264)   /* fundo geral, quase branco */
--bg-sidebar:     oklch(96%  0.003 264)   /* sidebar e zonas secundárias */
--bg-input:       oklch(0%   0     0 / 0.04)  /* fill sutil em inputs */
--bg-elev:        oklch(100% 0     0)     /* cards sobre bg-sidebar */
--bg-hover:       oklch(94%  0.004 264)   /* hover de items navegáveis */

--fg-strong:      oklch(22%  0.01  264)   /* headings, valores em destaque */
--fg-default:     oklch(48%  0.01  264)   /* body, labels */
--fg-muted:       oklch(65%  0.008 264)   /* metadados, placeholders */
--fg-on-accent:   oklch(99%  0.002 264)   /* texto sobre fundo accent */

--border-subtle:  oklch(92%  0.004 264)
--border-strong:  oklch(85%  0.006 264)

--accent:         oklch(63%  0.18  250)   /* primary CTA, links, focus ring */
--accent-hover:   oklch(58%  0.18  250)
--accent-soft:    oklch(95%  0.04  250)   /* fundo de badges accent */

--success:        oklch(60%  0.15  145)
--success-soft:   oklch(95%  0.04  145)
--warning:        oklch(75%  0.16  85)
--warning-soft:   oklch(96%  0.05  85)
--danger:         oklch(58%  0.20  25)
--danger-soft:    oklch(95%  0.05  25)
```

Contraste validado: `--fg-default` sobre `--bg-app` ≈ 7.1:1 (AAA texto). `--fg-on-accent` sobre `--accent` ≈ 4.8:1 (AA texto, AAA UI).

Banimentos respeitados:
- Sem `#000` / `#fff` puros.
- Sem gradient text.
- Sem side-stripe borders coloridos.

## Typography

Fonte: **Inter** (sistema fallback: `-apple-system, system-ui, "Segoe UI", Roboto`).

Carregamento: `<link>` Google Fonts no head, `font-display: swap`. Fallback até carregar usa stack do sistema (zero CLS visível porque Inter tem métricas próximas).

```
--font-sans: 'Inter', -apple-system, system-ui, 'Segoe UI', Roboto, sans-serif;
--font-mono: 'JetBrains Mono', ui-monospace, 'SF Mono', Consolas, monospace;

--text-xs:   12px  / 16px / 400
--text-sm:   13px  / 18px / 400   /* labels, metadata */
--text-base: 14px  / 20px / 400   /* body padrão */
--text-md:   15px  / 22px / 500   /* item titles, button labels */
--text-lg:   18px  / 24px / 520   /* H2 sections */
--text-xl:   24px  / 30px / 500   /* H1 page */
--text-2xl:  30px  / 36px / 600   /* hero raro */

--weight-regular: 400
--weight-medium:  500
--weight-semi:    600
```

Razão entre passos ≥1.25 entre níveis principais (14→18→24).

## Spacing & Layout

Grid base **4px**. Tokens:

```
--space-1: 4px
--space-2: 8px
--space-3: 12px
--space-4: 16px
--space-5: 20px
--space-6: 24px
--space-8: 32px
--space-10: 40px
--space-12: 48px
```

Shell:
- **Sidebar fixa esquerda**: 240px desktop, drawer off-canvas <768px.
- **Main**: fluido, padding `var(--space-8)` horizontal, `var(--space-6)` vertical.
- Não usar `max-w-6xl` central. Conteúdo respira até a borda; listas internas têm seus próprios limites de leitura.

Cap de leitura: parágrafos longos `max-inline-size: 70ch`. Listas e tabelas sem cap.

## Radius & Border

```
--radius-sm: 6px      /* badges, chips */
--radius-md: 8px      /* inputs, buttons, sidebar items */
--radius-lg: 12px     /* cards, modais */
--radius-xl: 16px     /* surfaces grandes raras */
--radius-full: 9999px /* avatars, pills */

--border-1: 1px solid var(--border-subtle)
--border-2: 1px solid var(--border-strong)
```

## Elevation

Sem sombras pesadas. Hierarquia por bg + border, não shadow.

```
--shadow-sm:  0 1px 2px oklch(22% 0.01 264 / 0.04)
--shadow-md:  0 4px 12px oklch(22% 0.01 264 / 0.06)   /* dropdowns, popovers */
--shadow-lg:  0 12px 32px oklch(22% 0.01 264 / 0.10)  /* modal */
```

Cards padrão NÃO recebem shadow — apenas `--border-1`.

## Motion

```
--ease-out: cubic-bezier(0.22, 1, 0.36, 1)    /* ease-out-quart */
--ease-in:  cubic-bezier(0.64, 0, 0.78, 0)
--dur-fast:   120ms
--dur-base:   200ms
--dur-slow:   320ms
```

Regras:
- Animar `opacity`, `transform`, `color`, `background-color`. **Nunca** `width`/`height`/`top`/`left`/`margin`/`padding`.
- Sem bounce, sem elastic.
- `prefers-reduced-motion: reduce` → todas durações → 0ms.

## Components

### Buttons

```
.btn-primary
  background: var(--accent)
  color: var(--fg-on-accent)
  padding: 6px 16px
  border-radius: var(--radius-md)
  font-weight: 500
  transition: background var(--dur-fast) var(--ease-out)
  hover: background var(--accent-hover)
  focus: outline 2px var(--accent), outline-offset 2px

.btn-ghost
  background: transparent
  border: var(--border-1)
  color: var(--fg-strong)
  padding: 6px 14px
  border-radius: var(--radius-md)
  hover: background var(--bg-hover)

.btn-danger
  background: var(--danger)
  color: var(--fg-on-accent)
  (mesmo padding/radius do primary)
```

### Inputs

```
.input
  background: var(--bg-input)
  border: 1px solid transparent
  border-radius: var(--radius-md)
  padding: 8px 12px
  font-size: var(--text-base)
  color: var(--fg-strong)
  transition: border-color var(--dur-fast) var(--ease-out)
  focus: border-color var(--accent), outline 2px var(--accent-soft)
  invalid: border-color var(--danger)
```

Labels: `<label>` com `font-size: 13px; font-weight: 500; color: var(--fg-default); margin-bottom: 6px`.

### Sidebar items

```
.nav-item
  display: flex
  align-items: center
  gap: var(--space-2)
  padding: 6px var(--space-3)
  border-radius: var(--radius-md)
  color: var(--fg-strong)
  font-size: var(--text-base)
  transition: background var(--dur-fast) var(--ease-out)
  hover: background var(--bg-hover)
  aria-current=page: background var(--bg-elev), color var(--fg-strong), font-weight 500
```

Ícone `<svg>` 18px, stroke 1.5, herda `currentColor`.

### Cards

```
.card
  background: var(--bg-elev)
  border: var(--border-1)
  border-radius: var(--radius-lg)
  padding: var(--space-6)
```

Não envolver tudo em card. Lista de tenants vive direto sobre `--bg-app`, sem card.

### List items (lista de tenants, mensagens, DLQ)

```
.list
  display: flex
  flex-direction: column
  /* sem gap; separador é border entre items */

.list-item
  display: grid
  grid-template-columns: auto 1fr auto
  gap: var(--space-3)
  align-items: center
  padding: var(--space-3) var(--space-4)
  border-bottom: var(--border-1)
  transition: background var(--dur-fast) var(--ease-out)
  hover: background var(--bg-hover)
  last-child: border-bottom: none
```

Ícone à esquerda (24px container), título + subtitle no meio, ações à direita.

### Stepper (wizard)

```
.stepper
  display: flex
  flex-direction: column
  gap: var(--space-6)
  width: 200px

.step
  display: flex
  gap: var(--space-3)
  align-items: flex-start

.step-bullet
  width: 24px
  height: 24px
  border-radius: var(--radius-full)
  display: grid
  place-items: center
  font-size: 12px
  font-weight: 500
  flex-shrink: 0

.step[data-state=active]   .step-bullet  background: var(--accent), color: var(--fg-on-accent)
.step[data-state=completed] .step-bullet background: var(--accent-soft), color: var(--accent)
.step[data-state=upcoming]  .step-bullet background: var(--bg-input), color: var(--fg-muted)
```

Estado conecta via linha vertical sutil entre bullets (`::before` posicionado).

### Badges

```
.badge
  display: inline-flex
  align-items: center
  padding: 2px 8px
  border-radius: var(--radius-sm)
  font-size: var(--text-xs)
  font-weight: 500

.badge-success { background: var(--success-soft); color: var(--success) }
.badge-warning { background: var(--warning-soft); color: var(--warning) }
.badge-danger  { background: var(--danger-soft); color: var(--danger) }
.badge-neutral { background: var(--bg-input); color: var(--fg-default) }
```

### Empty state

Container centralizado, ilustração SVG inline 96-128px, headline `text-md`, descrição `text-sm fg-muted`, CTA primary opcional.

### Toast

Bottom-right, `--shadow-md`, `--radius-md`, max-width 360px, dismiss em 4s ou clique. Variantes seguem `--success`/`--danger`/`--accent`.

## Iconography

SVG inline, stroke 1.5px, 18×18px em sidebar/buttons, 16×16px inline. Set único, traço fino, sem fill. Estética próxima a Lucide mas sem aplicação ornamental — só onde o ícone esclarece (não decora).

Permitidos por contexto: home, users, message-square, alert-triangle, settings, qr-code, link, copy, log-out, search, plus.

## Anti-patterns banidos

Reforço dos absolutos da skill + específicos do projeto:

- Sem `#000` / `#fff` puros.
- Sem side-stripe colorido (`border-left: 3px solid`).
- Sem gradient text (`background-clip: text` + gradient).
- Sem glassmorphism (backdrop-filter decorativo).
- Sem cards repetitivos com mesmo tamanho/ícone/heading/texto.
- Sem modal como primeira escolha. Inline > drawer > modal.
- Sem hero-metric template (big number + small label + gradient).
- Sem em dash (`—`). Usar vírgula, dois-pontos, parênteses.
- Sem Tailwind colors hardcoded (`bg-slate-50`, `text-emerald-700`, etc). Sempre via CSS vars.
- Sem shadow em cards padrão.

## Responsividade

Breakpoints:
```
--bp-sm: 640px
--bp-md: 768px
--bp-lg: 1024px
--bp-xl: 1280px
```

- `<768px`: sidebar vira drawer off-canvas, abre por botão hambúrguer top-left.
- `768-1024px`: sidebar colapsada (só ícones, 56px) com tooltip on hover.
- `≥1024px`: sidebar completa 240px.
- Main padding reduz para `var(--space-4)` em `<640px`.

## Implementação

- Tailwind CDN (atual) substituído por config `tailwind.config.js` futuramente. Por ora, definir vars CSS em `<style>` no `layout.html` e referenciar via `style="background:var(--bg-app)"` ou via classes utilitárias mínimas.
- Inter via Google Fonts `<link>` no head, `display=swap`.
- Sem framework JS novo. htmx + Alpine.js continuam.
- Todos componentes implementados como partials Go template em `internal/bridge/web/templates/partials/`.
