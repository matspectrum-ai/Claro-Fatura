# Migração para Go + frontend vanilla

Objetivo: reimplementar o produto atual sem alterar regras de negócio, UX ou schema do banco.

## Stack de destino

- Frontend público e administrativo: HTML + CSS + JavaScript vanilla.
- Backend: Go 1.23, biblioteca padrão (`net/http`).
- Banco: Supabase/PostgreSQL existente.
- Assets: embutidos no binário Go com `go:embed`.
- Planilhas: SheetJS 0.18.5 pinado no build e embutido no binário; nenhuma dependência de CDN/Node em runtime.

## Milestones concluídos

- Consulta pública de fatura por telefone.
- Geração de PIX, idempotência e Payment Router.
- CashinPay, ProPix, M2 Pay, NowBanks, PIX estático e REST genérico.
- Webhooks, confirmação e consulta de status.
- Frontend público vanilla.
- Autenticação/admin e recuperação de senha.
- Dashboard e métricas.
- Clientes/faturas e importação CSV/XLS/XLSX.
- Pagamentos, transações e logs.
- Configuração/roteamento de gateways.

## Regra de migração

O projeto antigo é o oracle de comportamento. A migração não deve alterar regras existentes de gateways, dados, importação, roteamento ou cobrança. Mudanças funcionais devem ser tratadas separadamente da migração de stack.

## Próximo corte

1. Fechar paridade de aliases/endpoints legados ainda expostos pelo projeto original.
2. Executar verificação final de contratos e build de produção.
3. Rodar testes de carga e dimensionar a VPS a partir dos resultados medidos.
