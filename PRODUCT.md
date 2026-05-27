# Product

## Register

product

## Users

Dois perfis hoje, ambos operam o painel admin do bridge:

1. **Super admin (Script7 / Giovani)** — opera tenants, debuga DLQ, faz onboarding. Contexto: dia útil, navegador desktop, foco em precisão e velocidade. Domina megaAPI e Chatwoot por dentro.
2. **Clientes revendedores** (futuro próximo) — empresas que revendem Chatwoot + megaAPI como SaaS. Acessam apenas os tenants que possuem. Contexto: também desktop, mas menos familiaridade técnica. Esperam UI no nível do Chatwoot que já operam.

Job-to-be-done compartilhado: provisionar tenants, parear WhatsApp, monitorar mensagens, retomar DLQ. Tarefas curtas, alta frequência.

## Product Purpose

Painel admin do bridge `chatwoot-megaapi-bridge`. Permite criar tenants (par instância megaAPI ↔ inbox Chatwoot), gerar links self-service de pareamento, acompanhar mensagens e DLQ, ajustar configurações.

Sucesso = um operador novo consegue criar tenant + gerar link pareamento em menos de 2 minutos, sem ler documentação. DLQ não vira buraco negro — falhas reaparecem com contexto e retry inline.

## Brand Personality

Acessível, amigável, claro.

- **Voz**: PT-BR direto, sem jargão desnecessário. Erros explicam o que aconteceu e o próximo passo.
- **Tom**: profissional sem ser frio. Próximo ao Chatwoot (que cliente revendedor já conhece).
- **Emoção alvo**: confiança calma. Operador sente que o sistema sabe o que está fazendo, sem precisar provar com efeitos.

## Anti-references

Especificamente NÃO pode parecer:

- **Painel WordPress / admin PHP genérico** — sidebar azul saturada, ícones aleatórios, hierarquia confusa.
- **Template Tailwind UI clichê SaaS** — hero gradient, cards repetidos, paleta lavanda/rosa pastel, ícones outline lucide aplicados em tudo.
- **Bootstrap admin dashboard** — botões 3D, sombras pesadas, badges coloridos por toda parte, mistura roxo + verde + laranja.
- **Material Design pesado** — FABs, ripples, Roboto, elevação alta, estética Google.

Positivo: **Chatwoot** (já rodando em `localhost:3000`) é a referência primária. Mesma família visual, sem cópia 1:1.

## Design Principles

1. **Continuidade com Chatwoot.** Cliente revendedor opera os dois lados (Chatwoot + admin bridge). Tokens visuais quase idênticos reduzem fricção cognitiva. Não é cópia, é dialeto da mesma língua.
2. **Densidade calibrada.** Listas, não tabelas decorativas. Cada linha mostra o suficiente pra decidir (status, contexto, ação) sem clicar.
3. **Affordances inline.** Ação principal de cada item fica visível na linha. Modal só quando inevitável.
4. **Copy explica, nunca decora.** Cada label, descrição, mensagem de erro tem função informacional. Zero placeholder de marketing.
5. **Tudo em PT-BR.** Sem traduções, sem fallback inglês. Único idioma cliente alvo.

## Accessibility & Inclusion

- **WCAG 2.2 AA** como linha base, validado em CI quando viável.
- Contraste mínimo 4.5:1 para texto, 3:1 para UI grande / focus rings.
- Focus visible em todo controle interativo (ring 2px accent).
- Navegação por teclado funcional em todos fluxos (Tab + Shift+Tab + Enter + Esc).
- Reduce-motion respeitado: animações via `@media (prefers-reduced-motion: reduce)` viram fade ou nada.
- Cor nunca é único sinal (sempre + ícone / texto).
- `aria-label` em ícones-only buttons; `lang="pt-BR"` no `<html>`; landmarks `<header>`, `<nav>`, `<main>`, `<aside>`.
