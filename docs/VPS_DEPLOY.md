# Deploy em VPS — Claro Fatura

Esta configuração mantém o produto e as regras existentes. Ela troca apenas a forma de execução: um binário Go estático atrás do Nginx.

## Layout

```text
Internet
  ↓
Nginx :80/:443
  ↓
127.0.0.1:8080
  ↓
/opt/claro-fatura/current/claro-fatura
  ↓
Supabase/PostgreSQL existente
```

Arquivos na VPS:

```text
/opt/claro-fatura/releases/<release-id>/claro-fatura
/opt/claro-fatura/current -> releases/<release-id>
/opt/claro-fatura/previous -> releases/<release-id-anterior>
/etc/claro-fatura/claro-fatura.env
/etc/systemd/system/claro-fatura.service
/etc/nginx/sites-enabled/claro-fatura.conf   # Debian/Ubuntu
```

## 1. Preparar a VPS

Exemplo para Ubuntu/Debian:

```bash
sudo apt update
sudo apt install -y nginx curl ca-certificates
```

Clone o repositório apenas para obter os arquivos de deploy, ou copie a pasta `deploy/` para a VPS. Em seguida:

```bash
sudo bash deploy/bootstrap-vps.sh fatura.seudominio.com
```

O bootstrap:

- cria o usuário de sistema `claro-fatura`, sem login;
- cria `/opt/claro-fatura/releases`;
- cria `/etc/claro-fatura/claro-fatura.env` se ainda não existir;
- instala a unit do systemd;
- instala e valida a configuração do Nginx;
- habilita o serviço para iniciar com a máquina, mas não publica uma release vazia.

Edite o env:

```bash
sudo nano /etc/claro-fatura/claro-fatura.env
```

No mínimo, preencha `SUPABASE_URL` e `SUPABASE_SERVICE_ROLE_KEY`. Mantenha `ADDR=127.0.0.1:8080` para que apenas o Nginx exponha a aplicação publicamente.

## 2. Obter o binário

O GitHub Actions gera o artefato `claro-fatura-linux-amd64` contendo:

```text
claro-fatura
claro-fatura.sha256
```

Confira o hash antes de publicar:

```bash
sha256sum -c claro-fatura.sha256
```

O servidor de produção não precisa ter Go, Node.js ou npm instalados.

## 3. Publicar uma release

```bash
sudo bash deploy/release.sh ./claro-fatura <commit-ou-release-id>
```

O script:

1. copia o binário para uma pasta imutável em `releases/`;
2. registra a release atual como `previous`;
3. troca o symlink `current`;
4. reinicia o systemd;
5. consulta `http://127.0.0.1:8080/healthz`;
6. se o health check não ficar saudável, restaura automaticamente a release anterior.

Verificação manual:

```bash
curl -fsS http://127.0.0.1:8080/healthz
sudo systemctl status claro-fatura --no-pager
sudo journalctl -u claro-fatura -n 100 --no-pager
sudo nginx -t
```

## 4. Rollback manual

```bash
sudo bash deploy/rollback.sh
```

O script troca `current` e `previous`, reinicia o serviço e exige health check saudável.

## 5. TLS

A configuração versionada do Nginx escuta HTTP para permanecer independente do provedor de certificado. Em produção, use uma destas opções sem alterar a aplicação:

- Cloudflare proxy/TLS na borda, com TLS apropriado também no origin conforme sua política; ou
- Certbot/Let's Encrypt no Nginx da VPS.

`SITE_URL` deve usar a URL pública HTTPS final, porque ela é usada na composição de URLs de webhook/cobrança.

## 6. Teste de carga seguro

Não rode carga contra criação PIX real ou gateways reais. Use `/healthz` para capacidade do runtime e um ambiente de staging/fake para fluxos de pagamento.

Compilar o load generator:

```bash
go build -o loadgen ./cmd/loadgen
```

Exemplo contra o origin local:

```bash
./loadgen \
  -url http://127.0.0.1:8080/healthz \
  -requests 100000 \
  -concurrency 500
```

Para medir o caminho de consulta real ao Supabase, use um telefone de teste em um ambiente autorizado e comece em degraus. Registre RPS, erros e p50/p95/p99 em cada degrau.

Perfil recomendado de validação:

```text
100 req/s
250 req/s
500 req/s
1000 req/s
2000 req/s
```

Pare o aumento quando ocorrer qualquer um destes sinais:

- erros sustentados;
- p95 ultrapassando o SLO definido para o produto;
- CPU saturada de forma contínua;
- memória/FDs/conexões de banco próximos do limite;
- degradação observável no Supabase ou nas gateways.

Os números sintéticos de loopback não devem ser usados como promessa de capacidade de produção.

## 7. Observação da VPS durante carga

Em outro terminal:

```bash
watch -n1 'systemctl show claro-fatura -p MemoryCurrent -p TasksCurrent; echo; ss -s'
```

E para CPU/RAM do processo:

```bash
pid=$(pidof claro-fatura)
top -p "$pid"
```

Para logs:

```bash
sudo journalctl -fu claro-fatura
```

A escolha final do tamanho da VPS deve ser feita depois do teste envolvendo o Supabase real/staging, pois o benchmark de loopback mede principalmente Go/TCP/HTTP e não a latência do banco e dos gateways.
