# Núcleo Operacional — Market Data API

Comece pelas [instruções para agentes](../../AGENTS.md). Este documento preserva
o escopo e os critérios da Etapa 5, inclusive sugestões anteriores à implementação;
essas propostas não substituem o [contrato HTTP atual](../../openapi.yaml).
Para executar e configurar a implementação atual, use o
[guia operacional](development-guide.md) e o
[checklist de validação](validation-checklist.md). Consulte as
[pendências consolidadas](../pendencias-consolidadas.md) para o trabalho atual.

Status em 10 de setembro de 2026: implementação local concluída. Liveness,
readiness, métricas JSON, logging de falhas HTTP 5xx, graceful shutdown e health
check dos containers foram adicionados. A imagem foi construída e validada com
PostgreSQL local; liveness 200/readiness 503 também foram confirmados com a
dependência isolada. A implantação real continua pendente de validação no
ambiente correspondente.

Resultados e procedimentos:

- [baseline do Asset Intelligence](performance-baseline.md);
- [checklist de validação operacional](validation-checklist.md).

## 1. Objetivo

Completar o núcleo operacional da `market-data-api` necessário para
executar o serviço de forma previsível, observável e segura em um
ambiente equivalente ao de produção.

Esta etapa cobre:

- health checks;
- readiness;
- encerramento gracioso;
- validação da imagem da aplicação;
- validação do ambiente implantado;
- verificação do Asset Intelligence;
- verificação do Swagger/OpenAPI;
- medição de latência;
- medição do custo das consultas de histórico longo;
- métricas operacionais;
- melhoria do logging de erros HTTP inesperados.

Esta etapa não deve introduzir otimizações sem evidência obtida por
medição.

---

## 2. Responsabilidade do serviço

A `market-data-api` é responsável por:

- servir a API HTTP;
- consultar os dados publicados necessários;
- executar os casos de uso da aplicação;
- calcular e expor os indicadores pertencentes ao seu contrato;
- expor informações operacionais sobre sua própria execução;
- encerrar corretamente quando solicitado pelo ambiente de execução.

A aplicação deve permanecer somente leitura em relação ao banco de
dados.

---

## 3. Responsabilidade sobre o banco

As migrations não pertencem à `market-data-api`.

Elas pertencem ao projeto:

`b3-data-hub`

O mesmo princípio se aplica aos processos responsáveis por importar,
publicar e preparar os dados utilizados pela API.

A `market-data-api` não deve:

- executar migrations;
- criar tabelas;
- alterar schemas;
- importar COTAHIST;
- publicar calendário;
- corrigir dados através de comandos de escrita;
- possuir lógica de bootstrap de banco.

O ambiente em que a API for implantada deve possuir previamente as
migrations, calendários, versões e dados exigidos pelo contrato atual.

---

## 4. Acesso somente leitura

A aplicação deve operar conceitualmente como:

market-data-api
       |
       | SELECT
       v
PostgreSQL

e não como:

market-data-api
       |
       | INSERT / UPDATE / DELETE / DDL
       v
PostgreSQL

A Etapa 5 não deve introduzir operações de escrita.

Caso o ambiente permita, recomenda-se que a credencial utilizada pela
API tenha privilégios compatíveis com acesso somente leitura.

A alteração de usuários/permissões do banco não deve ser feita neste
repositório caso pertença à infraestrutura externa.

---

## 5. Health checks

A aplicação deve possuir endpoints operacionais separados para
liveness e readiness.

Sugestão de contrato:

GET /health/live

GET /health/ready

Os nomes finais devem respeitar convenções já existentes no projeto,
caso existam.

---

## 6. Liveness

O liveness responde:

> O processo da aplicação está vivo e capaz de responder HTTP?

O liveness não deve depender de PostgreSQL nem de serviços externos.

Uma falha temporária no banco não significa necessariamente que o
processo HTTP precisa ser reiniciado.

Resposta esperada quando saudável:

HTTP 200

Exemplo conceitual:

{
  "status": "ok"
}

O formato final deve seguir as convenções HTTP existentes no projeto.

---

## 7. Readiness

O readiness responde:

> Esta instância está pronta para atender requisições que dependem de
> suas dependências obrigatórias?

Na versão atual, PostgreSQL é uma dependência obrigatória.

O readiness deve realizar uma verificação leve da conectividade com o
banco.

Não utilizar consultas de negócio pesadas para health check.

Não consultar histórico de ativos para determinar readiness.

Quando a dependência obrigatória estiver indisponível, o endpoint deve
retornar status HTTP não saudável compatível com a convenção adotada
pelo projeto.

Sugestão:

HTTP 503 Service Unavailable

---

## 8. Política inicial de health check

A política inicial é:

### `/health/live`

Verifica somente o processo HTTP.

Não verifica banco.

### `/health/ready`

Verifica:

- aplicação inicializada;
- conectividade com PostgreSQL.

Não verificar nesta etapa:

- quantidade de registros;
- existência de determinado ticker;
- calendário de uma data específica;
- disponibilidade de dados de mercado de um ativo específico;
- serviços externos futuros;
- LLM.

A ausência de dados de negócio específicos não deve transformar
automaticamente a aplicação em não saudável.

---

## 9. Timeouts dos health checks

A verificação de readiness deve possuir timeout curto.

O health check não pode ficar indefinidamente bloqueado esperando uma
dependência.

O valor final deve seguir a configuração e os padrões existentes no
projeto.

Se não existir política de timeout atualmente, o valor deve ser
centralizado/configurável em vez de espalhado pelo código.

Não criar retries agressivos dentro do endpoint de health check.

---

## 10. Encerramento gracioso

O servidor HTTP deve suportar graceful shutdown.

A aplicação deve reagir aos sinais apropriados do sistema operacional,
incluindo os utilizados por containers/orquestradores.

Fluxo esperado:

SIGTERM / interrupção
        |
        v
parar de aceitar novo trabalho
        |
        v
aguardar requisições em andamento
        |
        v
encerrar servidor HTTP
        |
        v
liberar recursos
        |
        v
encerrar processo

O shutdown deve possuir timeout.

A aplicação não deve esperar indefinidamente por requisições presas.

---

## 11. Recursos no shutdown

Durante o encerramento, liberar corretamente os recursos pertencentes
ao processo, quando aplicável.

Exemplos:

- HTTP server;
- pool/conexão com PostgreSQL;
- goroutines pertencentes ao ciclo de vida da aplicação;
- outros recursos explicitamente inicializados pelo processo.

Não fechar recursos que não pertencem ao ciclo de vida da aplicação.

---

## 12. Erros durante shutdown

Erros de shutdown devem ser registrados utilizando o logging
estruturado existente.

Um encerramento solicitado normalmente não deve ser registrado como
falha inesperada da aplicação.

Deve ser possível distinguir:

- shutdown solicitado;
- timeout durante shutdown;
- falha inesperada do servidor.

---

## 13. Imagem da aplicação

A imagem utilizada para implantação deve ser validada.

A validação deve confirmar, no mínimo:

- build da imagem;
- inicialização do container;
- configuração necessária;
- conectividade com as dependências;
- health checks;
- Asset Intelligence;
- Swagger/OpenAPI;
- encerramento do container.

Não assumir que sucesso de `go test` significa que a imagem está pronta
para produção.

---

## 14. Ambiente equivalente ao de produção

A Etapa 5 deve validar a aplicação em ambiente suficientemente próximo
do modelo real de implantação.

Esse ambiente deve possuir:

- imagem construída pelo processo previsto de deploy;
- PostgreSQL compatível;
- migrations exigidas;
- calendário publicado exigido;
- histórico publicado exigido;
- configurações equivalentes às utilizadas em produção;
- health checks configurados.

Segredos reais de produção não são necessários para essa validação.

---

## 15. Pré-condições de dados

Antes dos testes de implantação, confirmar que o ambiente possui as
versões de dados exigidas pela aplicação.

Esses dados são responsabilidade do `b3-data-hub`.

Caso faltem migrations ou dados necessários, a `market-data-api` não
deve tentar corrigi-los.

A falha deve ser identificada como problema de preparação do ambiente.

---

## 16. Smoke tests

Após a implantação, executar smoke tests.

No mínimo:

### Liveness

GET /health/live

Esperado:

HTTP 200

### Readiness

GET /health/ready

Esperado:

HTTP 200 quando PostgreSQL estiver disponível.

### Asset Intelligence

Executar uma requisição válida ao endpoint atual de Intelligence.

Utilizar um ativo e uma data existentes no ambiente.

Confirmar:

- HTTP status;
- contrato da resposta;
- indicadores esperados;
- metadata aplicável.

### Swagger/OpenAPI

Confirmar que a interface Swagger e/ou o documento OpenAPI expostos
pela aplicação estão acessíveis conforme o contrato atual.

---

## 17. Teste de indisponibilidade do banco

Validar o comportamento do readiness quando PostgreSQL estiver
indisponível.

Esperado:

/health/live -> saudável

/health/ready -> não saudável

Isso confirma a separação entre liveness e readiness.

O teste não deve exigir alteração permanente nos dados.

---

## 18. Medição antes de otimização

Não implementar cache, materialização ou nova arquitetura de consulta
somente por suspeita de problema de performance.

Primeiro medir.

Fluxo:

consulta atual
     |
     v
medição
     |
     v
evidência
     |
     v
decisão arquitetural
     |
     v
otimização, se necessária

---

## 19. Consulta de histórico longo

Medir o comportamento da consulta utilizada pelo Asset Intelligence
quando uma janela longa de histórico for necessária.

Registrar pelo menos:

- duração total da requisição;
- duração da consulta ao banco, quando mensurável;
- quantidade de registros retornados/processados;
- comportamento com `asOf`;
- consumo de memória do processo;
- consumo de CPU quando a ferramenta/ambiente permitir.

Utilizar cenários representativos.

Evitar benchmark baseado somente em uma única execução.

---

## 20. Cenários de medição

A medição deve incluir cenários representativos do contrato atual.

Exemplos:

- histórico recente;
- histórico longo;
- requisição com `asOf`;
- requisição sem `asOf`, quando suportada;
- ativo com volume de histórico representativo.

Os cenários finais devem ser derivados do comportamento real da
aplicação e dos dados disponíveis.

---

## 21. Baseline

Registrar um baseline antes de qualquer otimização.

Exemplo conceitual:

scenario: intelligence-long-history
duration_ms: ...
db_duration_ms: ...
rows: ...
memory: ...
as_of: ...

O formato final pode ser documentação, resultado de benchmark ou
artefato equivalente.

O objetivo é permitir comparação futura.

---

## 22. Cache

Cache não faz parte automaticamente da Etapa 5.

Somente propor cache após identificar evidência de que:

- existe custo relevante;
- existe repetição que pode ser aproveitada;
- a semântica de invalidação pode ser definida;
- a consistência esperada pode ser preservada.

Não implementar Redis ou cache em memória apenas para cumprir esta
etapa.

---

## 23. Materialização

Materialização de cálculos ou consultas também não deve ser introduzida
sem medição.

Caso a medição indique necessidade, produzir uma proposta separada
contendo:

- problema observado;
- métricas;
- alternativas;
- impacto de consistência;
- estratégia de atualização/invalidação;
- responsabilidade entre `market-data-api` e `b3-data-hub`.

Não implementar silenciosamente uma tabela materializada na
`market-data-api`.

---

## 24. Métricas operacionais

A aplicação deve possuir uma estratégia de métricas operacionais.

Métricas mínimas desejadas:

### HTTP

- quantidade de requisições;
- duração das requisições;
- status HTTP;
- rota/handler.

### Dependências

- duração das consultas relevantes ao PostgreSQL;
- erros de acesso ao PostgreSQL.

### Asset Intelligence

- duração do caso de uso;
- resultado operacional quando apropriado;
- indisponibilidades relevantes quando puderem ser agregadas com
  cardinalidade controlada.

### Processo

Quando suportado pela solução de métricas utilizada:

- memória;
- CPU;
- goroutines;
- runtime Go.

---

## 25. Cardinalidade

Labels/tags de métricas devem possuir cardinalidade controlada.

Não utilizar diretamente como label:

- ticker;
- `asOf`;
- request ID;
- mensagem de erro;
- SQL;
- usuário;
- valores financeiros.

Rotas devem utilizar identificadores estáveis/template da rota, e não
URLs contendo parâmetros dinâmicos.

Exemplo:

/assets/{ticker}/intelligence

em vez de:

/assets/PETR4/intelligence

quando utilizado como label de métrica.

---

## 26. Endpoint de métricas

Caso o projeto ainda não possua solução equivalente, a implementação
pode expor métricas em endpoint operacional dedicado.

Exemplo convencional:

GET /metrics

A escolha da biblioteca e formato deve respeitar as dependências e
padrões atuais do projeto.

Não adicionar uma nova stack de observabilidade sem antes verificar o
que já existe.

---

## 27. Logging estruturado

Preservar o mecanismo de logging estruturado existente.

Não substituir a solução atual apenas para implementar esta etapa.

Use o `*slog.Logger` injetado, com chaves estáveis para ticker, tipo de mercado,
quantidade de registros e duração. Nunca registre credenciais ou séries
completas de dados de mercado.

Os logs de erro HTTP inesperado devem fornecer contexto suficiente para
investigação sem expor dados sensíveis.

Quando disponíveis, registrar campos estruturados apropriados como:

- método HTTP;
- rota;
- status;
- duração;
- request/correlation ID existente;
- categoria do erro.

Evitar registrar:

- secrets;
- credenciais;
- tokens;
- payloads sensíveis;
- SQL completo sem necessidade;
- stack traces indiscriminadamente.

---

## 28. Erros HTTP inesperados

Melhorar a observabilidade principalmente de erros 5xx inesperados.

No comportamento atual, erros HTTP inesperados são registrados com método,
rota normalizada, status e categoria; a resposta HTTP permanece genérica.

Deve ser possível diferenciar:

- erro esperado de domínio/validação;
- recurso não encontrado;
- indisponibilidade conhecida;
- erro interno inesperado.

Erros esperados não devem necessariamente ser registrados com o mesmo
nível de severidade de falhas internas.

A resposta HTTP não deve expor detalhes internos apenas porque o log
foi enriquecido.

---

## 29. Relação entre logs e métricas

Logs e métricas possuem objetivos diferentes.

Métricas:

> Existe um problema? Com que frequência? Qual o impacto?

Logs:

> O que aconteceu nesta execução específica?

Evitar transformar logs em substitutos para métricas ou criar labels de
alta cardinalidade para reproduzir detalhes que pertencem aos logs.

---

## 30. Segurança operacional

Endpoints operacionais não devem expor informações sensíveis.

Health checks não devem retornar:

- connection string;
- host interno desnecessário;
- usuário do banco;
- senha;
- SQL;
- stack trace;
- detalhes internos de infraestrutura.

O readiness precisa informar estado, não credenciais ou detalhes
sensíveis da dependência.

---

## 31. Compatibilidade

A Etapa 5 não deve alterar os cálculos financeiros existentes.

Preservar:

- Asset Intelligence;
- indicadores;
- regras determinísticas existentes;
- `asOf`;
- `includeDetails`;
- critérios atuais de `complete`/`partial`;
- contrato de indisponibilidade;
- endpoints existentes.

Alterações operacionais não devem mudar silenciosamente regras de
negócio.

---

## 32. Testes automatizados

Adicionar testes apropriados para:

- liveness saudável;
- readiness saudável;
- readiness com dependência indisponível;
- liveness independente do banco;
- comportamento dos endpoints operacionais;
- graceful shutdown quando testável de forma determinística;
- logging/middleware apenas quando houver comportamento verificável sem
  acoplar testes à implementação interna;
- regressão dos endpoints existentes.

Não criar testes frágeis baseados em `sleep` arbitrário quando houver
forma determinística de sincronização.

---

## 33. Validação manual/ambiente

Alguns critérios desta etapa dependem de ambiente implantado e não
podem ser considerados aprovados somente por testes unitários.

Registrar separadamente:

- testes automatizados;
- testes de integração;
- smoke tests;
- validações de container;
- validações do ambiente implantado;
- medições de performance.

Um teste ignorado não deve ser reportado como aprovado.

---

## 34. Critérios de aceite

A Etapa 5 estará concluída quando:

1. existir liveness independente do PostgreSQL;
2. existir readiness verificando a dependência obrigatória;
3. health checks possuírem timeout apropriado;
4. o servidor suportar graceful shutdown com timeout;
5. recursos pertencentes à aplicação forem liberados no shutdown;
6. a imagem da aplicação for construída e executada com sucesso;
7. o ambiente equivalente ao de produção possuir migrations e dados
   necessários fornecidos pelo `b3-data-hub`;
8. a API permanecer somente leitura;
9. smoke tests validarem health, Intelligence e Swagger/OpenAPI;
10. o comportamento com PostgreSQL indisponível for validado;
11. existir baseline da consulta de histórico longo;
12. cache ou materialização não forem introduzidos sem evidência;
13. métricas operacionais mínimas estiverem definidas e implementadas
    conforme a stack adotada;
14. labels de métricas tiverem cardinalidade controlada;
15. erros HTTP inesperados possuírem logging estruturado adequado;
16. informações sensíveis não forem expostas;
17. os cálculos e contratos de negócio existentes permanecerem
    compatíveis;
18. testes automatizados aplicáveis passarem;
19. validações que dependem de ambiente forem documentadas com seu
    resultado real.

---

## 35. Fora de escopo

Não implementar nesta etapa:

- migrations;
- importação de COTAHIST;
- publicação de calendário;
- correção de dados;
- escrita no banco;
- Redis sem evidência;
- cache sem medição;
- materialização sem medição;
- alteração dos cálculos financeiros;
- alteração das regras de sinais;
- LLM;
- sistema de notificações;
- nova plataforma de observabilidade sem avaliar a stack existente.

---

## 36. Evoluções futuras

Após obter métricas reais, podem ser propostas etapas específicas para:

- cache;
- Redis;
- materialização;
- otimização de queries;
- índices adicionais no PostgreSQL;
- tracing distribuído;
- SLOs;
- alertas operacionais;
- dashboards;
- autoscaling.

Essas decisões devem ser baseadas nas medições produzidas nesta etapa.
