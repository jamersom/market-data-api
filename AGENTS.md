# Instruções para agentes — market-data-api

API HTTP somente leitura de dados publicados do B3 COTAHIST: última cotação,
histórico paginado, comparações e indicadores determinísticos de ativos.
O `b3-data-hub` é responsável pela ingestão, publicação e schema dos dados.
Este arquivo é o ponto de entrada do projeto; não há `README.md`.

## Leitura obrigatória por tarefa

Antes de alterar comportamentos, leia este arquivo, o [OpenAPI](openapi.yaml)
e os documentos aplicáveis abaixo. As regras desses documentos são obrigatórias
no respectivo escopo. O OpenAPI é o contrato público atual, incorporado ao binário
por [docs.go](docs.go). Propostas antigas não comprovam comportamento implementado.

| Tarefa | Documentação a consultar |
|---|---|
| Alterar código, camadas, portas, HTTP ou SQL | [Desenvolvimento e arquitetura atual](docs/desenvolvimento.md) |
| Alterar cálculos ou representação financeira | [Cálculos compartilhados](docs/analytics.md) |
| Alterar comparações | [Aplicação de comparações](docs/comparison-application.md) |
| Alterar Asset Intelligence | [Contrato atual](docs/intelligence-contrato-atual.md) e [caso de uso](docs/asset-intelligence-application.md) |
| Alterar histórico, calendário ou consistência de Intelligence | [Calendário](docs/calendario-intelligence.md) |
| Executar, configurar, trabalhar com Docker ou CI/CD | [Guia operacional](docs/operations/development-guide.md) |
| Alterar health checks, logs, métricas ou shutdown | [Núcleo operacional](docs/operations/operational-core.md) |
| Validar alterações ou preparar um PR | [Checklist de validação](docs/operations/validation-checklist.md) |
| Planejar evolução | [Pendências consolidadas](docs/pendencias-consolidadas.md) e [pendências de Intelligence](docs/asset-intelligence-pendencias.md) |

Prefira documentos identificados como contrato atual ou pendências atuais.
As ressalvas sobre propostas e trechos desatualizados estão no [guia técnico](docs/desenvolvimento.md).
Informe inconsistências não resolvidas; não deduza implementações a partir de propostas.

## Arquitetura e escopo

- Preserve a direção das dependências: adaptadores → aplicação/portas → domínio.
  O domínio não depende de HTTP, JSON, PostgreSQL ou `pgx`.
- Regras de negócio e fórmulas pertencem ao domínio; serviços validam e orquestram.
  Handlers cuidam do transporte HTTP e repositórios do acesso aos dados.
  Não coloque regras de negócio em handlers ou adaptadores SQL.
- Repositórios retornam tipos do domínio/aplicação, nunca DTOs HTTP.
- Reutilize portas, interfaces, DTOs, mappers, cálculos, validações e projeções.
  Defina portas de escopo restrito; verifique as existentes antes de criar abstrações.
- Use as camadas existentes; evite novas estruturas na raiz sem necessidade demonstrada.
  Preserve a composição manual das dependências em [cmd/main.go](cmd/main.go).
- Faça mudanças pequenas e incrementais, sem refatorações fora do escopo.
  Só adicione dependências se o comportamento exigir e os recursos existentes não atenderem.
- Siga a nomenclatura, os erros, logs e estilo de testes do código próximo.

## Cálculos e contratos HTTP

- Reutilize [analytics](internal/domain/analytics) para cálculos puros e determinísticos.
  Não duplique fórmulas em handlers/serviços nem use LLM para calcular ou preencher valores financeiros.
- Não crie preços sintéticos para pregões ausentes; preserve insuficiência e indisponibilidade explícitas.
- Mantenha valores monetários armazenados em centavos inteiros e a saída monetária
  em strings decimais, conforme o contrato; preserve a precisão dos resultados calculados.
- Percentuais usam unidades percentuais (`20` = 20%); arredonde somente no mapper
  quando previsto no contrato. Use ponteiros para opcionais quando zero for válido.
- Preserve slices/mapas do chamador quando o código os copia antes de ordenar ou normalizar.
- Preserve os endpoints, salvo alteração de contrato explicitamente solicitada.
  Alterações públicas exigem atualizar juntos OpenAPI, DTOs, mappers, handlers e testes.
- Preserve `camelCase` em cotações/comparações e `snake_case` em Intelligence;
  não uniformize sem alteração explícita do contrato.
- Datas HTTP usam `YYYY-MM-DD`; preserve a normalização civil para meia-noite UTC
  nos serviços de comparação e Intelligence, conforme o comportamento existente.
- Propague `context.Context` entre handlers, serviços e repositórios, respeitando cancelamento e timeouts.
- Preserve o envelope central de erros, o encadeamento de erros e respostas genéricas para falhas internas.

## Banco de dados e segurança

- Mantenha acesso somente leitura, salvo alteração explícita do escopo e da responsabilidade pelo schema.
  Ingestão, publicação e migrations pertencem ao `b3-data-hub`.
- Não edite migrations já aplicadas. Mudanças de schema exigem nova migration no
  repositório responsável, coordenada com a API; informe responsabilidade incerta antes de alterar código.
- Use SQL parametrizado e preserve a ordenação garantida por cada porta.
  Feche resultados e propague erros de leitura/iteração conforme o guia técnico.
- Mantenha histórico de Intelligence e calendário no mesmo snapshot somente leitura `REPEATABLE READ`.
- Use o `*slog.Logger` injetado e chaves estáveis; não registre credenciais ou séries completas.
  Falhas HTTP inesperadas incluem método, rota normalizada, status e categoria; a resposta permanece genérica.
- Não inclua `.env` ou credenciais em commits, nem segredos de logs, strings de conexão
  ou credenciais de CI em código, testes, documentação ou respostas.

## Fluxo de trabalho e entrega

1. Entenda a solicitação e os critérios de aceite; identifique arquivos e contratos envolvidos.
2. Verifique `git status` antes de editar. Preserve alterações do usuário e arquivos
   modificados ou não rastreados que não estejam relacionados à tarefa.
3. Consulte as leituras aplicáveis e implementações/testes semelhantes; avalie impactos
   na arquitetura, API, dados e compatibilidade antes de implementar a menor mudança necessária.
4. Valide a alteração conforme o [checklist](docs/operations/validation-checklist.md):
   após alterar Go, execute `gofmt`, `go test ./...` e `go vet ./...`;
   execute `go build ./...` quando houver impacto na compilação ou composição.
   Antes de abrir PR, execute todas essas verificações, formatando somente os arquivos Go alterados.
5. Habilite integrações PostgreSQL/calendário explicitamente, com os pré-requisitos do checklist.
   Elas usam dados publicados existentes, sem criar massas nem modificar o banco.
6. Informe o que mudou, por quê, as verificações executadas, as não executadas e as limitações pendentes.
