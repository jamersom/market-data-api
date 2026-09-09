> Contrato atualizado: consulte [intelligence-contrato-atual.md](intelligence-contrato-atual.md).
> Os trechos abaixo registram etapas anteriores. Campos futuros foram removidos
> do JSON; complete/partial considera apenas indicadores implementados.
> O calendário observado já está conectado e a cobertura anual exige includeDetails=true.

# Pendências do Asset Intelligence

> Atualização: a integração do calendário consolidado está implementada.
> `cmd/main.go` conecta o provedor observado, com leitura em snapshot e
> metadados `meta.calendar`. A dependência de uma fonte exclusivamente oficial
> foi substituída pela política explícita de cobertura observada. Consulte
> [calendario-intelligence.md](calendario-intelligence.md). Os itens antigos de
> calendário abaixo registram o planejamento anterior; anos ausentes no banco
> continuam indisponíveis. Deploy e evoluções quantitativas seguem pendentes.

Atualizado em: 2026-09-07.

Endpoint previsto: `GET /assets/{ticker}/intelligence`.

URL prevista para PETR4 após implementação e deploy:
`https://marketdata.techcomp.net.br/assets/PETR4/intelligence`.
Essa rota está implementada localmente, mas não foi feito deploy nesta etapa. Este arquivo é documentação do projeto,
não uma página publicada automaticamente no servidor.

## Implementado

- Pacote `internal/domain/analytics` com retorno, volatilidade anualizada,
  drawdown máximo e volume médio, reutilizado por `/comparisons`.
- SMA, distância da SMA, RSI de Wilder e drawdown atual, com testes.
- `GetAssetIntelligenceUseCase` e serviço de aplicação com validação de ticker,
  mercado e data de referência, cálculo das janelas e metadados de ausência.
- Porta de histórico e adaptador PostgreSQL com consulta completa até `asOf`,
  filtros de ticker/mercado e versão dos dados por hash.
- Interface opcional para um provedor de calendário verificado.
- Rota GET, handler com timeout de cinco segundos, DTOs e mapper JSON em
  `snake_case`, conectados em `cmd/main.go`.
- OpenAPI 1.3.0 com o novo contrato e testes HTTP, inclusive integração com
  serviço e PostgreSQL real, sem calendário.
- Testes unitários e teste de integração somente leitura com PostgreSQL.
  Testes, `go vet` e build passaram na conclusão da etapa do adaptador.

## Pendências para disponibilizar a primeira versão

### 1. Calendário de pregões verificado

Definir e implementar a fonte de sessões da B3, com cobertura explícita desde
o primeiro registro histórico até a referência solicitada.

- Implementar `IntelligenceCalendar` e conectá-lo ao adaptador.
- Validar a cobertura histórica, feriados e sessões excepcionais.
- Definir atualização e versionamento do calendário.
- Testar intervalos fora da cobertura e lacunas nas cotações.

Não construir o calendário a partir das próprias cotações nem presumir que todo
dia útil é pregão. O calendário usado para agendamento no `b3-data-hub` não foi
validado como fonte completa para esse histórico.

**Impacto atual:** sem provedor verificado, o serviço retorna a cotação, mas os
indicadores com histórico suficiente ficam indisponíveis por
`calendar_unavailable`. Indicadores com histórico curto continuam indicando
`insufficient_history`. Expor a rota não resolve essa dependência.

### 2. Contrato de resposta HTTP — concluído

Os itens abaixo foram implementados; JSON em `snake_case`, parâmetros em
`asOf`/`marketType`, valores monetários em strings com duas casas.

- Criar DTOs e mapper com envelope `data`/`meta`.
- Consolidar a escolha entre `snake_case` da proposta e `camelCase` usado na API.
- Organizar preço, retornos, tendência, momentum, risco e liquidez nos blocos
  propostos, mapeando os campos do resultado de aplicação.
- Definir representação decimal de SMA e preços, arredondamento na saída e
  unidades dos percentuais.
- Representar métricas indisponíveis com `null` e motivos por campo.
- Respeitar `price.close: invalid_data`: não apresentar fechamento inválido como
  preço válido só porque o registro de origem está presente no resultado.
- Informar data solicitada/efetiva, versão do cálculo, versão dos dados e base
  de preços não ajustados.
- Distinguir regras ainda não executadas de listas vazias após avaliação.

### 3. Handler e registro da rota — concluído

- Criar handler para `GET /assets/{ticker}/intelligence`.
- Validar `asOf` e `marketType`, inclusive valores explicitamente inválidos.
- Definir comportamento para parâmetros repetidos ou desconhecidos, seguindo
  o padrão dos handlers existentes.
- Aplicar timeout e propagar cancelamento do contexto.
- Mapear validações, ausência de cotação e falhas de infraestrutura para o
  envelope de erros existente.
- Conectar repositório, serviço e handler em `cmd/main.go`.
- Registrar a rota em `internal/adapters/inbound/http/routes.go`.
- Não aceitar `benchmark` silenciosamente nesta entrega: o caso de uso ainda
  não oferece essa opção.

### 4. OpenAPI e validação HTTP — implementados

Contrato disponível em `openapi.yaml`, servido pelo Swagger existente.
Testes HTTP e integração somente leitura com PostgreSQL passaram. A revisão
visual do Swagger após deploy permanece pendente.

- Atualizar `openapi.yaml` com parâmetros, resposta, unidades e erros.
- Incluir exemplos de resposta parcial e calendário indisponível.
- Adicionar testes do handler e mapper, incluindo limites temporais, dados
  ausentes, erros e cancelamento.
- Verificar a rota pelo Swagger em `/docs/` e testar com o adaptador real.
- Executar testes, `go vet` e build, preservando `/comparisons`.

### 5. Publicação e verificação

- Gerar e publicar a imagem com a implementação revisada.
- Aplicar o deploy no servidor quando autorizado.
- Verificar a URL do endpoint e sua documentação pelo domínio publicado.
- Conferir latência com históricos longos e resposta parcial sem calendário.

## Evoluções posteriores à primeira versão

| Recurso | Pendência |
|---|---|
| Percentil de RSI em três anos | Consolidar cobertura mínima e implementar distribuição histórica e testes |
| Benchmark | Adicionar parâmetro, validar disponibilidade da fonte e consultar histórico |
| Força relativa | Alinhar datas e calcular diferença de retornos contra o benchmark |
| Contexto de mercado | Definir universo, fontes, métricas e regras |
| Sinais | Especificar condições e evidências numéricas versionadas |
| Alertas | Definir regras quantitativas; notificações são um escopo separado |
| Score | Definir componentes, pesos, normalização e tratamento de ausências |
| Preços ajustados | Definir fonte e metodologia antes de oferecer séries ajustadas |
| Cache/materialização | Medir necessidade e considerar versões de dados, calendário e cálculos |

Esses blocos estão marcados como `not_implemented` no serviço. GPT/Gemini não
devem preencher as lacunas nem substituir os cálculos determinísticos.

## Limitações que precisam permanecer explícitas

- `asOf` limita a data do pregão, mas consulta a versão atualmente publicada dos
  dados. Não reconstrói automaticamente o que era conhecido naquela época.
- O RSI começa no primeiro registro publicado disponível; importações de dados
  mais antigos ou correções podem mudar o valor e a versão dos dados.
- A consulta carrega todo o histórico para manter essa inicialização. Medir o
  custo antes de disponibilizar o serviço em escala.
- O hash atual não inclui o calendário; não usá-lo sozinho como chave de cache
  dos indicadores quando o calendário puder mudar.
- O resultado permanece `partial` enquanto houver blocos não implementados.

## Ordem de continuidade

O próximo passo é fornecer o calendário verificado, necessário para obter
indicadores disponíveis com o adaptador real. A rota já está conectada e retorna
resposta parcial sem esse provedor. Deploy e verificação no domínio continuam
pendentes. As evoluções de percentil, benchmark e regras são etapas posteriores.
Após cada implementação, parar para revisão antes de avançar, conforme solicitado.

## Documentos relacionados

- [Proposta do endpoint](endpoint-asset-intelligence.md).
- [Caso de uso e adaptador PostgreSQL](asset-intelligence-application.md).
- [Convenções dos indicadores](analytics.md).
- [Cálculos compartilhados com comparações](comparison-application.md).
