# B3 Data Platform — arquitetura e evolução

Este documento descreve a arquitetura recomendada para a plataforma, considerando o estado atual dos projetos `b3-data-hub` e `market-data-api`. A proposta separa a arquitetura que deve ser operada agora da arquitetura-alvo, evitando introduzir complexidade distribuída antes de existir uma necessidade concreta.

Documentação relacionada:

- [Endpoint `comparisons`](endpoint-comparisons.md): contrato proposto de comparação de dois ou mais tickers, nas variantes GET e POST, incluindo integração com a Nexo.

## 1. Visão executiva

Hoje, a plataforma possui duas responsabilidades principais bem delimitadas:

- `b3-data-hub`: obtém, valida, processa e publica dados históricos da B3.
- `market-data-api`: consulta os dados publicados e os entrega por HTTP.

A recomendação é preservar esses dois serviços e desenvolver analytics, busca e eventos corporativos inicialmente como módulos. Um módulo somente deve se tornar um microsserviço quando precisar de implantação, escala, armazenamento ou ciclo de vida independentes.

## 2. Arquitetura recomendada agora

```mermaid
flowchart TB
    clients[Clientes B2B<br/>Corretoras · Fintechs · Plataformas]

    subgraph platform[B3 Data Platform]
        api[market-data-api<br/>Market Data · Analytics inicial · Busca básica]
        ingestion[b3-data-hub<br/>Download · Parsing · Validação · Publicação]
        db[(PostgreSQL<br/>Dados históricos e metadados)]
        obs[Observabilidade básica<br/>Logs estruturados · Métricas]
    end

    b3[B3 / COTAHIST]

    clients -->|HTTPS / REST / JSON| api
    api -->|Consultas somente leitura| db
    b3 -->|Arquivos históricos| ingestion
    ingestion -->|Staging e publicação| db
    ingestion -.->|Logs e métricas| obs
    api -.->|Logs e métricas| obs
```

### Responsabilidades atuais

| Componente | Responsabilidade | Evitar |
|---|---|---|
| `b3-data-hub` | Download, parsing, validação, versionamento e publicação | Servir consultas para clientes |
| `market-data-api` | Histórico, última cotação, filtros, paginação e metadados | Download ou parsing do COTAHIST |
| PostgreSQL | Fonte estruturada dos dados publicados | Ser acessado sem limites por todos os futuros serviços |
| Observabilidade | Logs, métricas e diagnóstico | Uma plataforma completa antes de haver demanda |

## 3. Fluxo de ingestão

```mermaid
flowchart LR
    schedule[Agendamento] --> source[Download B3]
    source --> store[Armazenamento do arquivo bruto]
    store --> parser[Parser COTAHIST]
    parser --> validation{Arquivo e registros válidos?}
    validation -->|Não| failure[Registrar falha<br/>permitir retry]
    validation -->|Sim| staging[(Dados em staging)]
    staging --> publish[Publicação versionada]
    publish --> published[(published_historical_quotes)]
    publish --> metadata[(historical_imports)]
    published --> available[Dados disponíveis para consulta]
    metadata --> available
```

Princípios do fluxo:

- O arquivo bruto deve ser preservado quando for necessário reproduzir uma importação.
- Validação e parsing acontecem antes da publicação.
- Dados incompletos não devem ficar visíveis como uma versão publicada.
- A importação deve registrar origem, horário, versão do parser e versão do layout.
- Retentativas precisam ser idempotentes para não duplicar dados.

## 4. Fluxo de consulta HTTP

```mermaid
sequenceDiagram
    autonumber
    actor Client as Cliente
    participant Handler as Adapter HTTP
    participant PortIn as Porta inbound
    participant Service as Application Service
    participant Domain as Domínio
    participant PortOut as Porta outbound
    participant Repository as Adapter PostgreSQL
    participant DB as PostgreSQL

    Client->>Handler: GET /quotes/{ticker}
    Handler->>Handler: Converte path e query parameters
    Handler->>PortIn: Execute(GetQuoteInput)
    PortIn->>Service: Executa caso de uso
    Service->>Domain: Normaliza e valida ticker/regras
    Domain-->>Service: Valor válido ou erro de domínio
    Service->>PortOut: FindLatestByTicker(...)
    PortOut->>Repository: Implementação concreta
    Repository->>DB: SQL parametrizado
    DB-->>Repository: Linha consultada
    Repository-->>Service: domain.QuoteRecord
    Service-->>Handler: GetQuoteOutput
    Handler->>Handler: Mapeia domínio para DTO
    Handler-->>Client: 200 JSON + metadata
```

### Fluxos de erro

```mermaid
flowchart TD
    request[Requisição HTTP] --> parse{Parâmetros legíveis?}
    parse -->|Não| badRequest[400 invalid_request]
    parse -->|Sim| rules{Regras do caso de uso válidas?}
    rules -->|Não| validation[400 erro de validação específico]
    rules -->|Sim| query[Consultar repositório]
    query --> found{Registro encontrado?}
    found -->|Não| notFound[404 quote_not_found]
    found -->|Sim| success[200 resposta JSON]
    query -->|Erro inesperado| internal[500 internal_error]
```

## 5. Arquitetura interna do market-data-api

```mermaid
flowchart LR
    subgraph inboundAdapter[Adapter de entrada]
        routes[Routes]
        handlers[HTTP Handlers]
        dto[DTOs e mappers]
        errors[Mapeamento de erros HTTP]
    end

    subgraph application[Aplicação]
        inputPorts[Portas inbound<br/>Interfaces dos casos de uso]
        services[Application Services<br/>Validação e coordenação]
        outputPorts[Portas outbound<br/>Interfaces de repositório]
    end

    subgraph core[Domínio]
        entities[Quote · QuoteRecord · QuotePage]
        rules[Normalização · Constantes · Erros]
    end

    subgraph outboundAdapter[Adapter de saída]
        postgresRepository[QuoteRepository PostgreSQL]
    end

    subgraph infrastructure[Infraestrutura]
        pool[pgxpool · configuração · timeouts]
        postgres[(PostgreSQL)]
    end

    routes --> handlers
    handlers --> inputPorts
    handlers --> dto
    errors --> dto
    inputPorts --> services
    services --> core
    services --> outputPorts
    postgresRepository -. implementa .-> outputPorts
    postgresRepository --> pool
    pool --> postgres
```

A direção importante das dependências é:

```text
Adapters → Application → Domain
```

O domínio não conhece HTTP, JSON, SQL ou `pgx`. Os serviços conhecem interfaces de repositório, mas não conhecem PostgreSQL.

## 6. Fluxo para implementar um novo caso de uso

```mermaid
flowchart TD
    idea[Nova necessidade de negócio] --> domain{Exige novo conceito<br/>ou regra de domínio?}
    domain -->|Sim| domainChange[Adicionar entidade, valor,<br/>regra ou erro no domínio]
    domain -->|Não| inbound
    domainChange --> inbound[Definir Input, Output<br/>e porta inbound]
    inbound --> dependency{Precisa consultar ou<br/>alterar recurso externo?}
    dependency -->|Sim| outbound[Definir porta outbound mínima]
    dependency -->|Não| service
    outbound --> service[Implementar Application Service]
    service --> serviceTests[Testes unitários do caso de uso]
    serviceTests --> adapter{Existe porta outbound?}
    adapter -->|Sim| repository[Implementar adapter PostgreSQL]
    repository --> integration[Teste de integração do repositório]
    adapter -->|Não| handler
    integration --> handler[Criar handler e DTOs HTTP]
    handler --> handlerTests[Testar handler e erros]
    handlerTests --> route[Registrar rota]
    route --> wiring[Conectar dependências no cmd/main.go]
    wiring --> openapi[Atualizar OpenAPI]
    openapi --> verify[Testar fluxo completo]
```

Checklist de cada caso de uso:

1. Identificar regras e tipos de domínio.
2. Definir uma porta de entrada pequena e específica.
3. Definir somente as dependências externas necessárias.
4. Implementar o serviço sem HTTP ou SQL.
5. Testar regras com dependências fake.
6. Implementar o adapter de persistência.
7. Criar handler, DTOs e mapeamento de erros.
8. Registrar rota e montar dependências no `main`.
9. Atualizar o contrato OpenAPI.
10. Executar testes unitários e de integração.

## 7. Organização modular recomendada

Analytics e busca podem nascer dentro do `market-data-api`, mantendo fronteiras internas explícitas:

```text
internal/
├── marketdata/
│   ├── domain/
│   ├── application/
│   └── adapters/
├── analytics/
│   ├── domain/
│   ├── application/
│   └── adapters/
├── search/
│   ├── domain/
│   ├── application/
│   └── adapters/
└── platform/
    ├── database/
    ├── logging/
    └── http/
```

Essa reorganização é uma direção futura, não uma migração obrigatória imediata. A estrutura atual pode crescer até a necessidade dos módulos ficar clara.

## 8. Arquitetura-alvo

```mermaid
flowchart TB
    clients[Clientes B2B]
    gateway[API Gateway<br/>Roteamento · Rate limit · TLS · Versionamento]
    auth[Autenticação e autorização<br/>OAuth2 · OIDC · API keys · RBAC]

    subgraph business[Serviços e módulos de negócio]
        ingestion[Data Ingestion]
        market[Market Data]
        analytics[Analytics]
        events[Corporate Events]
        search[Search]
        ai[AI Orchestrator]
        notification[Notification]
    end

    eventBus[(Kafka / Event Bus)]
    cache[(Redis)]
    database[(PostgreSQL)]
    objects[(S3 / MinIO)]
    vectors[(Vector Store)]
    sources[B3 e fontes externas]
    telemetry[Prometheus · Grafana · Loki · Tempo]

    clients --> gateway
    gateway <--> auth
    gateway --> market
    gateway --> analytics
    gateway --> events
    gateway --> search
    gateway --> ai

    sources --> ingestion
    ingestion --> objects
    ingestion --> database
    ingestion --> eventBus

    eventBus --> analytics
    eventBus --> search
    eventBus --> notification
    eventBus --> events

    market --> database
    analytics --> database
    events --> database
    search --> database
    market <--> cache
    analytics <--> cache

    ai --> market
    ai --> analytics
    ai --> search
    ai --> events
    ai --> vectors

    ingestion -.->|telemetria| telemetry
    market -.->|telemetria| telemetry
    analytics -.->|telemetria| telemetry
    ai -.->|telemetria| telemetry
```

Este desenho é um destino possível, não uma lista de componentes que precisam ser criados imediatamente.

## 9. Critérios para extrair um microsserviço

Um módulo deve ser extraído apenas quando pelo menos uma necessidade concreta justificar o custo:

- Escala de CPU, memória ou tráfego muito diferente do restante da aplicação.
- Implantação ou frequência de mudanças independente.
- Equipe responsável e ciclo de vida próprios.
- Modelo de dados e regras com fronteira clara.
- Requisitos de disponibilidade ou segurança diferentes.
- Processamento assíncrono que não combina com o processo HTTP.

Somente possuir uma responsabilidade diferente não basta. Um monólito modular já permite separar responsabilidades sem introduzir falhas de rede e consistência distribuída.

## 10. Decisões de tecnologia

| Tecnologia | Usar quando | Situação recomendada |
|---|---|---|
| API Gateway | Existirem vários serviços HTTP ou autenticação/rate limit centralizados | Adiar |
| Redis | Métricas demonstrarem consultas repetidas e caras ou houver rate limit distribuído | Adiar e medir primeiro |
| Kafka | Um evento possuir vários consumidores independentes e assíncronos | Adiar; considerar outbox antes |
| Kubernetes | Houver muitos serviços, escala independente e maturidade operacional | Adiar |
| Vector store | Existir busca semântica sobre documentação e conteúdo não estruturado | Adiar |
| RAG | Respostas precisarem usar manuais, comunicados e relatórios | Usar para documentos, não como fonte de cálculos |
| PostgreSQL | Cotações, ativos, indicadores persistidos e busca inicial | Manter |
| Docker Compose/Swarm | Desenvolvimento e operação atual com poucos serviços | Manter por enquanto |

## 11. Dados estruturados e IA

Perguntas quantitativas devem usar ferramentas e APIs determinísticas:

```mermaid
flowchart LR
    question[Pergunta do usuário] --> interpretation[Interpretação estruturada]
    interpretation --> tool[Chamada de ferramenta/API]
    tool --> marketAPI[Market Data / Analytics API]
    marketAPI --> structured[(Dados estruturados)]
    structured --> answer[Resposta em linguagem natural]
```

Exemplos: maior preço, volatilidade, médias móveis e comparação de ativos.

RAG deve ser reservado principalmente para documentação não estruturada:

```mermaid
flowchart LR
    docs[Manuais · Comunicados · Relatórios] --> chunks[Extração e segmentação]
    chunks --> embeddings[Embeddings]
    embeddings --> vector[(Vector Store)]
    question[Pergunta] --> retrieval[Busca semântica]
    vector --> retrieval
    retrieval --> contextual[Resposta com contexto e fontes]
```

## 12. Evolução do banco compartilhado

No estágio atual, é aceitável que ingestão e API usem o mesmo PostgreSQL se a propriedade for respeitada:

```mermaid
flowchart LR
    ingestion[b3-data-hub<br/>proprietário da escrita] --> staging[(ingestion / staging)]
    staging --> publication[Publicação atômica]
    publication --> market[(market_data / published)]
    market --> api[market-data-api<br/>consumidor de leitura]
```

Regras recomendadas:

- A ingestão é proprietária da escrita dos dados históricos.
- A API consulta apenas dados publicados.
- Novos serviços não devem escrever livremente nas mesmas tabelas.
- Separar schemas antes de separar bancos pode ser uma boa etapa intermediária.
- Bancos independentes só são necessários quando a autonomia operacional justificar.

## 13. Roadmap

```mermaid
flowchart LR
    phase1[Fase 1<br/>Núcleo confiável] --> phase2[Fase 2<br/>Produto de dados]
    phase2 --> phase3[Fase 3<br/>Eventos e assincronismo]
    phase3 --> phase4[Fase 4<br/>Inteligência]

    phase1 --- p1[Ingestão · API · índices · testes<br/>OpenAPI · health checks · CI]
    phase2 --- p2[Ativos · indicadores · busca SQL<br/>métricas · autenticação]
    phase3 --- p3[Outbox · eventos · Kafka se necessário<br/>notificações · analytics assíncrono]
    phase4 --- p4[AI Orchestrator · ferramentas<br/>RAG documental · guardrails]
```

### Fase 1 — núcleo confiável

- Consolidar a ingestão do COTAHIST.
- Consolidar as consultas do `market-data-api`.
- Criar índices e testes de integração.
- Manter o OpenAPI atualizado.
- Adicionar health checks e encerramento gracioso.
- Padronizar migrations, CI e configuração de credenciais.

### Fase 2 — produto de dados

- Adicionar endpoints de ativos e metadados.
- Implementar indicadores como casos de uso internos.
- Usar PostgreSQL para busca inicial por ticker, nome e ISIN.
- Adicionar métricas, autenticação, API keys e rate limiting.
- Introduzir cache somente após medir gargalos.

### Fase 3 — comunicação assíncrona

- Definir eventos estáveis e versionados.
- Implementar transactional outbox na ingestão.
- Introduzir Kafka quando houver múltiplos consumidores reais.
- Processar analytics e notificações de forma assíncrona.

### Fase 4 — inteligência

- Criar um orquestrador de IA que consuma APIs, sem acessar tabelas diretamente.
- Usar ferramentas estruturadas para cálculos financeiros.
- Introduzir RAG para documentação.
- Adicionar auditoria, contexto, memória e guardrails.

## 14. Decisão resumida

```text
Agora:
  b3-data-hub + market-data-api + PostgreSQL + observabilidade básica

Em seguida:
  analytics e busca como módulos + autenticação + métricas

Quando houver demanda comprovada:
  gateway + Redis + eventos/outbox + Kafka + serviços extraídos

Depois:
  AI Orchestrator + RAG documental + infraestrutura distribuída
```

A arquitetura-alvo orienta decisões, mas a arquitetura implantada deve permanecer proporcional ao produto, à carga e à capacidade operacional atuais.
