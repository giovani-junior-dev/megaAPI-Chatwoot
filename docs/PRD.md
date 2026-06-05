# PRD — Bridge: WhatsApp + Chatwoot autohospedado

> Documento de produto. Foco no **o quê** e no **para quem**, não no como técnico.

## 1. Visão geral

O **Bridge** conecta o **WhatsApp** (via instâncias da megaAPI) ao **Chatwoot**,
permitindo que equipes atendam clientes do WhatsApp dentro do Chatwoot — com
instalação de **um comando**, autohospedado, multi-tenant e seguro por padrão.

O produto resolve a parte mais dolorosa de montar esse atendimento: subir e
ligar todas as ferramentas. Em vez de horas de configuração manual, o usuário
roda um instalador guiado e tem tudo no ar em minutos.

## 2. Problema

Quem quer atender WhatsApp pelo Chatwoot hoje precisa:

- Subir e configurar Chatwoot, banco, fila e proxy reverso
- Expor os serviços com HTTPS (DNS, certificados, portas)
- Conectar cada número de WhatsApp e ligar ao Chatwoot
- Manter os webhooks dos dois lados sincronizados e autenticados

Isso exige conhecimento técnico, é demorado e fácil de errar — especialmente
para quem não é desenvolvedor.

## 3. Objetivos

1. **Instalação em 1 comando** numa VPS limpa, sem conhecimento técnico profundo.
2. **Tempo até o primeiro atendimento** curto (poucos minutos após o comando).
3. **Seguro por padrão**: HTTPS sem abrir portas, segredos protegidos.
4. **Multi-tenant**: vários clientes/instâncias na mesma instalação.
5. **Operação simples**: painel para criar tenants, parear e acompanhar.

## 4. Público-alvo

- **Agências e prestadores** que gerenciam atendimento de vários clientes.
- **PMEs** que querem WhatsApp + Chatwoot próprios, sem depender de SaaS de terceiros.
- **Operadores não-técnicos** que conseguem seguir um passo a passo simples.

## 5. Proposta de valor

> Conecte o WhatsApp ao Chatwoot com um comando — autohospedado, multi-tenant e
> seguro, pronto em minutos.

- Sem montar a stack na mão
- Sem abrir portas nem mexer com certificados
- Dono dos próprios dados e conversas
- Onboarding guiado de ponta a ponta

## 6. Escopo / Funcionalidades

### 6.1 Instalador guiado
- Um comando sobe toda a stack (banco compartilhado, tunnel, Chatwoot, painel, backup).
- Pergunta o mínimo (endereços, token do tunnel, senhas) e **gera o resto sozinho**.
- Cria os administradores automaticamente; ferramentas ficam editáveis no painel de orquestração.

### 6.2 Painel de administração (Bridge)
- Login de administrador.
- Cadastro de **tenants** ligando uma caixa de entrada do Chatwoot a uma instância de WhatsApp.
- Validação amigável (ex.: identificador do tenant, existência da caixa de entrada).
- Geração de **link de pareamento** por tenant.

### 6.3 Pareamento de WhatsApp
- Link de pareamento por tenant (com validade).
- QR Code e código de pareamento; status de conexão em tempo real.

### 6.4 Mensageria
- Recebimento (WhatsApp → Chatwoot) e envio (Chatwoot → WhatsApp).
- Suporte a **texto e mídia/arquivos**.
- Webhooks autenticados configurados automaticamente no cadastro do tenant.

### 6.5 Segurança
- HTTPS via tunnel, sem abrir portas no servidor.
- Segredos sensíveis criptografados.
- Webhooks autenticados por token.

### 6.6 Backup
- Rotina de backup do banco com retenção configurável.

## 7. Tutorial de instalação

### Pré-requisitos
- VPS Ubuntu 24.04 com acesso root
- Um domínio na Cloudflare
- Conta no Cloudflare Zero Trust

### Comando de instalação
```bash
bash <(curl -fsSL https://raw.githubusercontent.com/giovani-junior-dev/megaAPI-Chatwoot/master/install-pack/bootstrap.sh)
```

### Passo a passo
1. **Crie o Tunnel no Cloudflare** (Zero Trust → Networks → Tunnels → Create,
   tipo Cloudflared). Copie o **token** (começa com `eyJ...`).
2. **Rode o comando** acima na VPS (como root). Responda:
   - hostnames: `chatwoot.SEUDOMINIO`, `bridge.SEUDOMINIO`,
     `portainer.SEUDOMINIO`, `backup.SEUDOMINIO` (use **1 nível** de subdomínio)
   - token do tunnel
   - e-mail e senha do admin do painel
   - senha do admin do orquestrador (mín. 12 caracteres)
3. **Crie os 4 Public Hostnames** no tunnel (tipo **HTTP**), apontando para os
   serviços internos.
4. **Configure o Chatwoot**: crie a conta admin e a **caixa de entrada do tipo API**
   (anote o Inbox ID, Account ID e o token de acesso).
5. **Crie o tenant** no painel do Bridge: informe a URL pública base, os dados do
   WhatsApp e os dados da caixa de entrada. O webhook é configurado
   automaticamente. (Crie a caixa de entrada **antes** do tenant.)
6. **Pareie o WhatsApp**: gere o link de pareamento, escaneie o QR e teste o
   envio e o recebimento (texto e mídia).

> **Dica:** subdomínio de **1 nível** (`chatwoot.seudominio.com`) é obrigatório —
> 2 níveis quebram o certificado HTTPS gratuito.

## 8. Requisitos não-funcionais

- **Usabilidade:** operável por não-técnicos; mensagens de erro claras em PT-BR.
- **Confiabilidade:** restart automático dos serviços; recuperação de mensagens pendentes.
- **Segurança:** sem portas expostas; segredos criptografados; webhooks autenticados.
- **Manutenção:** stacks editáveis pelo painel de orquestração; backups com retenção.
- **Portabilidade:** roda em uma única VPS; instalação reprodutível.

## 9. Métricas de sucesso

- Tempo do comando ao primeiro atendimento (meta: minutos).
- % de instalações concluídas sem suporte humano.
- Mensagens entregues nos dois sentidos sem falha.
- Nº de tenants ativos por instalação.

## 10. Fora de escopo (por enquanto)

- App mobile dedicado.
- Faturamento/cobrança embutidos (o produto pago é o serviço de WhatsApp à parte).
- Relatórios avançados além do que o Chatwoot já oferece.

## 11. Riscos e mitigações

| Risco | Mitigação |
|-------|-----------|
| Usuário erra a ordem (tenant antes da caixa de entrada) | Validação que bloqueia com mensagem clara |
| Subdomínio de 2 níveis quebra SSL | Aviso explícito no instalador e na documentação |
| Servidor sem Docker | Instalador instala automaticamente |
| Perda de segredos | Arquivo de segredos protegido; orientação de guardar em local seguro |
