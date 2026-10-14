# Pendências consolidadas de implementação

Atualizado em 10 de setembro de 2026.

## Objetivo

Este documento consolida o que ainda precisa ser implementado a partir dos
documentos existentes em `docs/`. A revisão foi feita comparando as propostas e
os roteiros com as rotas, casos de uso, adaptadores, testes, OpenAPI e
infraestrutura presentes no repositório.

Os documentos têm finalidades diferentes: alguns descrevem o contrato atual,
outros registram decisões já implementadas e outros preservam propostas ou uma
visão futura. Portanto, a presença de uma funcionalidade em um exemplo de
resposta não significa que ela já faça parte da API.

## Resultado da revisão

| Documento | Situação encontrada |
|---|---|
| `analytics.md` | Implementado. As funções quantitativas compartilhadas estão no domínio e são usadas pelos casos de uso atuais. |
| `arquitetura-plataforma.md` | Parcialmente implementado. A separação entre ingestão e consulta, PostgreSQL, OpenAPI e CI existem; o documento também contém as fases futuras detalhadas abaixo. |
| `asset-intelligence-application.md` | Implementado para o escopo atual do Asset Intelligence. |
| `asset-intelligence-pendencias.md` | É o backlog específico do endpoint. A primeira entrega e os metadados compactos foram concluídos; as evoluções continuam pendentes. |
| `calendario-intelligence.md` | Implementado com calendário observado. Um calendário oficial independente continua opcional. Há uma frase desatualizada sobre o estado `partial`, registrada na Etapa 0. |
| `comparison-application.md` | Implementado para o contrato atual de comparações. Há uma frase desatualizada dizendo que Intelligence ainda não existe, registrada na Etapa 0. |
| `endpoint-asset-intelligence.md` | Parcialmente implementado. O próprio documento identifica-se como proposta original e inclui recursos futuros que não representam o contrato atual. |
| `endpoint-comparisons.md` | A primeira versão está implementada. O documento adia explicitamente preços ajustados, novas granularidades, métricas dinâmicas, cache e eventual extração do serviço. |
| `intelligence-contrato-atual.md` | Implementado e correspondente ao contrato atual do endpoint. |

O contrato vigente do Asset Intelligence deve continuar sendo consultado em
`intelligence-contrato-atual.md` e em `openapi.yaml`. A proposta preservada em
`endpoint-asset-intelligence.md` não deve ser usada isoladamente para implementar
clientes.

## Etapa 0 — corrigir documentação desatualizada

Esta etapa não exige alteração de código.

- Atualizar `comparison-application.md`: o Asset Intelligence já foi
  implementado e reutiliza o pacote de cálculos compartilhados.
- Atualizar `calendario-intelligence.md`: recursos futuros indisponíveis não
  determinam, por si só, o estado `partial`. Os critérios atuais de `complete` e
  `partial` consideram somente os indicadores oferecidos pelo contrato vigente.
- Manter clara a distinção entre proposta original e contrato atual em futuras
  alterações de documentação.

Critério de aceite: os documentos descrevem o comportamento atual sem mudar o
contrato da API nem antecipar decisões das etapas seguintes.

Implementado como `momentum.rsi14_percentile`, com:

- janela civil configurável por `rsiWindow`; padrão `3y`, sem regra de domínio
  vinculada a uma duração específica;
- RSI da referência excluído da população histórica;
- midrank: `(menores + 0,5 × iguais) / total × 100`;
- mínimo centralizado de 252 observações válidas;
- `window` separado de `coverage`, com período real, número de observações e
  indicação de cobertura completa;
- uso da semente e do cálculo existente de RSI14, sem dados posteriores a
  `asOf`;
- indisponibilidade explícita e participação do novo indicador nos critérios
  atuais de `complete` e `partial`;
- contrato HTTP, OpenAPI, exemplos e testes atualizados.

## Etapa 2 — concluída: benchmark e força relativa

Implementado no Asset Intelligence com:

- parâmetro opcional `benchmark`, normalizado pelas regras de ticker existentes;
- rejeição HTTP 400 quando benchmark e ativo são iguais;
- leitura em lote dos dois históricos e do calendário no mesmo snapshot
  PostgreSQL somente leitura;
- janela vigente de sete pregões, usando as oito sessões reais e consecutivas
  mais recentes do ativo quando todas também existem para o benchmark;
- retornos calculados por `analytics.Return` nas mesmas datas inicial e final;
- força relativa calculada como retorno do ativo menos retorno do benchmark, em
  pontos percentuais;
- ausência, insuficiência ou desalinhamento do benchmark representados por
  `null`, `partial` e `meta.unavailable`, sem interromper indicadores do ativo;
- preservação do comportamento quando `benchmark` não é informado e ausência de
  participação do benchmark em rankings ou percentis;
- contrato HTTP, OpenAPI, exemplos e testes atualizados.

## Etapa 3 — concluída: sinais e alertas determinísticos

Implementado no Asset Intelligence com:

- ruleset `1.0` determinístico no domínio, sem consultas a repositório;
- regras `rsi_overbought`, `rsi_oversold`, `price_above_sma20`,
  `price_below_sma20`, `sma20_above_sma50` e `sma20_below_sma50`;
- thresholds RSI 70 e 30 centralizados no ruleset;
- estados `triggered`, `not_triggered` e `unavailable`, com avaliação
  independente de cada regra;
- evidências numéricas derivadas das mesmas entradas usadas nas comparações;
- SMA50 calculada pelo mecanismo compartilhado antes do evaluator;
- `ruleset_version` nos metadados detalhados, contrato HTTP, exemplos e testes.

Alertas compostos, recomendações, notificações, momentum adicional e uso de LLM
continuam fora do escopo.

## Etapa 4 — contexto de mercado e score

Não há definição suficiente para implementar esses campos atualmente.

### Definições necessárias antes do código

- Definir o universo de ativos e as fontes usadas para representar o mercado.
- Definir calendário e requisitos mínimos de cobertura.
- Definir componentes, pesos, normalização, limites e tratamento de dados
  ausentes do score.
- Definir a versão da metodologia e os componentes que devem acompanhar o
  total.

### Implementação após as definições

- Calcular contexto com dados do universo definido, sem inferi-lo a partir de
  um único ticker.
- Calcular score de forma determinística e reproduzível.
- Retornar componentes, total e versão da metodologia.
- Cobrir os cálculos com fixtures independentes e atualizar o contrato HTTP.

Não devem ser inventados pesos, fontes ou critérios durante a implementação.

## Etapa 5 — núcleo operacional implementado localmente

- `/health/live` verifica somente o processo HTTP.
- `/health/ready` executa `Ping` leve no PostgreSQL com timeout.
- `/metrics` expõe contagens e durações HTTP por rota normalizada/status,
  operações PostgreSQL agregadas e métricas do runtime Go.
- SIGINT/SIGTERM iniciam graceful shutdown com timeout e fechamento do pool.
- Falhas HTTP inesperadas são registradas com método, rota, status e categoria.
- Dockerfile, Compose e Stack possuem health check e prazo de encerramento.
- Imagem e smoke tests locais foram validados, inclusive liveness 200 e readiness
  503 quando somente o container de teste perdeu acesso ao banco.
- O baseline local do histórico longo foi registrado em
  `operations/performance-baseline.md`; não justificou cache ou materialização.

A implantação em ambiente equivalente a produção ainda precisa seguir
`operations/validation-checklist.md`. Ela não foi inferida a partir da validação
local.

As migrations pertencem ao `b3-data-hub`. O ambiente implantado deve conter as
migrations e importações exigidas pela versão atual do calendário e dos dados;
este repositório deve permanecer somente leitura em relação ao banco.

## Etapa 6 — evoluções condicionais dos endpoints atuais

Estas tarefas só devem começar após definição funcional ou evidência de
necessidade:

- Preços ajustados e retorno total: exigem fonte de eventos corporativos,
  metodologia e versão próprias. Séries ajustadas e não ajustadas não devem ser
  misturadas.
- Calendário oficial independente: pode complementar a validação do calendário
  observado, mas sua fonte e política ainda precisam ser definidas.
- Séries semanais e mensais em `/comparisons`.
- Métricas escolhidas dinamicamente pelo cliente em `/comparisons`.
- Cache ou materialização: somente após medição. A chave deve considerar os
  parâmetros da consulta e as versões dos dados, calendário, cálculos e regras.
- Extração para um `analytics-service`: somente se escala, processamento,
  implantação ou ciclo de vida independentes justificarem o custo.

## Etapa 7 — produto de dados da plataforma

Itens da Fase 2 da arquitetura que não foram encontrados neste repositório:

- Endpoints adicionais de ativos e metadados.
- Busca SQL por ticker, nome e ISIN.
- Métricas operacionais da API.
- Autenticação, API keys e rate limiting.

O escopo e os contratos desses recursos ainda precisam ser detalhados. Um API
Gateway e Redis devem continuar adiados até que o número de serviços ou as
medições demonstrem a necessidade.

## Etapa 8 — comunicação assíncrona

Itens da Fase 3 da arquitetura ainda não implementados:

- Definir eventos estáveis e versionados.
- Implementar transactional outbox no serviço de ingestão `b3-data-hub`.
- Introduzir Kafka ou outro barramento somente quando existirem múltiplos
  consumidores reais.
- Processar analytics ou notificações de forma assíncrona quando houver um caso
  de uso definido.

Esta etapa envolve principalmente a plataforma e o repositório de ingestão. Ela
não deve fazer o `market-data-api` escrever livremente nas tabelas publicadas.

## Etapa 9 — camada de inteligência com IA

Itens da Fase 4 da arquitetura ainda não implementados:

- Criar um orquestrador que consuma as APIs sem acessar as tabelas diretamente.
- Oferecer ferramentas estruturadas para cálculos financeiros.
- Adicionar RAG apenas para documentação e conteúdo não estruturado.
- Definir auditoria, contexto, memória e guardrails.

Os cálculos financeiros e as regras determinísticas continuam sendo
responsabilidade do backend. A IA deve interpretar resultados já calculados.

## Ordem recomendada

1. Corrigir as duas inconsistências documentais da Etapa 0.
2. Validar a Etapa 5 no ambiente implantado usando o checklist operacional.
3. Especificar contexto e score antes da Etapa 4.
4. Priorizar as Etapas 6 a 9 somente a partir de demanda e métricas concretas.

Conforme o backlog do Asset Intelligence, cada etapa funcional deve ser entregue
separadamente e interrompida para revisão antes do início da próxima.

## Definição de pronto para cada etapa

- A menor mudança necessária foi implementada nas camadas existentes.
- Fórmulas, unidades, arredondamentos e indisponibilidades estão documentados.
- Não há acesso a dados posteriores à data de referência.
- DTOs, OpenAPI e exemplos concordam com o comportamento real.
- Testes unitários, HTTP e de integração relacionados cobrem sucesso, limites e
  falhas relevantes.
- `gofmt`, `go vet ./...` e `go test ./...` passam quando houver alteração em
  código Go.
- Mudanças de schema são feitas por novas migrations no repositório responsável;
  migrations existentes não são reescritas.
- A documentação informa claramente o que foi entregue e o que continua fora do
  contrato atual.
