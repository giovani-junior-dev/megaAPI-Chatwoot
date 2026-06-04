# Prompt de instalação por agente de IA

Cole o bloco abaixo no **Claude Code** (ou outro agente de terminal) rodando
**na sua VPS** (Ubuntu, acesso root). O agente vai conduzir a instalação,
pedindo só o que for necessário. Você não precisa saber Docker.

> **Antes:** tenha em mãos (1) seu **domínio** já na Cloudflare e (2) o
> **token do Cloudflare Tunnel** (o agente te explica como pegar, ou veja o
> passo 5 do `00-LEIA-PRIMEIRO.md`).

---

```
Você é um instalador. Objetivo: subir o pack Chatwoot + bridge (megaAPI) nesta
VPS usando Docker Swarm, via o instalador oficial do repositório.

Faça, nesta ordem:

1. Pergunte ao usuário, UMA pergunta por vez:
   - o domínio raiz (ex: minhaempresa.com.br)
   - o token do Cloudflare Tunnel (começa com eyJ...)
   - o e-mail do admin do painel bridge
   - a senha do admin do painel bridge (não ecoe em logs)

2. Rode o instalador, passando as respostas por variáveis de ambiente em modo
   não-interativo:

   DOMINIO=<resp> TOKEN_DO_TUNNEL=<resp> ADMIN_EMAIL=<resp> \
   ADMIN_PASSWORD=<resp> NONINTERACTIVE=1 \
   bash <(curl -fsSL https://raw.githubusercontent.com/giovani-junior-dev/megaAPI-Chatwoot/master/install-pack/bootstrap.sh)

3. O instalador gera os demais segredos sozinho, sobe todas as stacks, prepara
   o banco do Chatwoot e cria o admin do bridge. Acompanhe a saída.

4. Depois, INSTRUA o usuário a criar 2 Public Hostnames no Cloudflare Zero
   Trust (Networks > Tunnels > o tunnel > Public Hostnames):
   - chat.<dominio>   -> tipo HTTP -> chatwoot_admin:3000
   - bridge.<dominio> -> tipo HTTP -> bridge:8080

5. Valide: rode `docker service ls` e confirme que TODOS os serviços estão 1/1.
   Se algum não subir, leia os logs com `docker service logs <nome>` e ajude o
   usuário a corrigir (causa comum: senha do Postgres divergente, ou token do
   tunnel errado — veja a tabela de troubleshooting do 00-LEIA-PRIMEIRO.md).

6. Confirme o smoke final: https://chat.<dominio> e https://bridge.<dominio>
   abrem.

REGRAS:
- NUNCA escreva os segredos em arquivos versionados nem os comite. O arquivo
  install-pack/.env (gerado pelo instalador) contém segredos e fica fora do git.
- NÃO abra portas 80/443; o acesso é só pelo Cloudflare Tunnel.
- Se um comando falhar, diagnostique e proponha a correção antes de repetir.
```

---

## Alternativa sem IA

Se preferir, rode você mesmo o one-liner (o instalador pergunta o que precisa):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/giovani-junior-dev/megaAPI-Chatwoot/master/install-pack/bootstrap.sh)
```
