# Teste de carga

Objetivo: medir capacidade da nova stack sem alterar o produto e sem disparar cobranças reais.

## Ferramenta

O repositório inclui `cmd/loadgen`, escrito somente com a biblioteca padrão do Go. Ele mede throughput, erros HTTP/rede e latência p50/p95/p99.

Exemplo básico:

```bash
go run ./cmd/loadgen \
  -url http://127.0.0.1:8080/healthz \
  -requests 10000 \
  -concurrency 100
```

Para enviar headers:

```bash
go run ./cmd/loadgen \
  -url https://staging.exemplo/api/public/faturas?telefone=11999999999 \
  -headers 'Authorization: Bearer TOKEN' \
  -requests 10000 \
  -concurrency 200
```

## Perfis sugeridos

Execute os perfis progressivamente e interrompa se erro/latência subir de forma abrupta:

```text
10.000 requests   / concorrência 100
50.000 requests   / concorrência 250
100.000 requests  / concorrência 500
100.000 requests  / concorrência 1.000
```

Esses números são cenários de teste, não garantias de capacidade.

## Alvos de aceitação iniciais

- 1.000 req/s sustentados no cenário de leitura;
- 3.000 req/s de burst curto no cenário sintético;
- p95 abaixo de 250 ms no cenário de produção equivalente;
- p99 abaixo de 500 ms;
- taxa de erro abaixo de 0,1%;
- nenhuma cobrança PIX duplicada para uma mesma chave de idempotência;
- nenhum webhook processado duas vezes.

Os limites finais devem ser definidos pelos resultados medidos na VPS e no banco reais.

## Segurança do teste

Nunca execute carga de geração PIX contra gateways de produção. Para `/pix`, `/api/public/cobranca`, `test-fluxo` ou `test-propix-direto`, use somente adapters fake/sandbox e dados descartáveis. O primeiro teste em VPS deve focar `/healthz`, assets estáticos e consulta de fatura em um ambiente de staging.

## Benchmark in-process

Para separar overhead do Go/HTTP do custo de rede/banco:

```bash
go test ./internal/httpapi -run '^$' -bench 'Benchmark(Health|Invoice)' -benchmem -benchtime=2s
```

Esse benchmark não representa a capacidade do Supabase nem da VPS; ele serve apenas como baseline do handler.
