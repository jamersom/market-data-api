# Execução, configuração e publicação

Este guia reúne os procedimentos atuais de desenvolvimento local, configuração
e publicação da `market-data-api`. Comece pelas [instruções para agentes](../../AGENTS.md)
e consulte as [convenções de desenvolvimento](../desenvolvimento.md) para arquitetura,
tecnologias e contratos. Os comandos de teste estão no
[checklist de validação](validation-checklist.md).

## Compilação e execução local

Execute os comandos a partir da raiz do repositório. Não há script próprio de
automação dos comandos nem `Makefile`.

Baixe as dependências:

```sh
go mod download
```

Execute a aplicação:

```sh
go run ./cmd
```

`DATABASE_URL` é obrigatória. O [ponto de entrada](../../cmd/main.go) tenta carregar
o `.env` da raiz e utiliza as variáveis de ambiente do processo quando esse arquivo
não está disponível. O servidor escuta em `SERVER_ADDRESS`, cujo padrão é `:8080`.

Compile todos os pacotes:

```sh
go build ./...
```

Compile o executável com a variável de versão utilizada pelas imagens de produção:

```sh
go build -trimpath -ldflags="-X main.version=development" -o market-data-api ./cmd
```

Não há ferramenta de lint separada configurada. O repositório e o CI usam
`go vet` para análise estática. Consulte os comandos e os critérios de execução
no [checklist de validação](validation-checklist.md).

## Configuração

O [`.env.example`](../../.env.example) é o modelo documentado. Nunca inclua `.env`
ou credenciais em commits; não exponha segredos em comandos, logs ou documentação.

Variáveis da aplicação lidas diretamente pelo código Go:

| Variável | Finalidade | Padrão e tratamento |
|---|---|---|
| `DATABASE_URL` | String de conexão PostgreSQL | Obrigatória. |
| `SERVER_ADDRESS` | Endereço de escuta HTTP | `:8080`. |
| `APP_ENV` | Ambiente incluído na configuração do logger | Sem padrão documentado aqui. |
| `LOG_LEVEL` | Nível dos logs | Sem padrão documentado aqui. |
| `LOG_FORMAT` | Formato de saída dos logs | Sem padrão documentado aqui. |
| `DB_MAX_CONNECTIONS` | Máximo de conexões do pool | `10`; valores inválidos usam o padrão. |
| `DB_MIN_CONNECTIONS` | Mínimo de conexões do pool | `2`; valores inválidos usam o padrão. |
| `DB_MAX_CONN_LIFETIME` | Tempo de vida das conexões, no formato de duração de Go | `30m`; valores inválidos usam o padrão. |
| `DB_CONNECT_TIMEOUT` | Timeout de conexão, no formato de duração de Go | `5s`; valores inválidos usam o padrão. |
| `HEALTH_CHECK_TIMEOUT` | Prazo do Ping de readiness, no formato de duração de Go | `2s`; valores inválidos usam o padrão. |
| `SHUTDOWN_TIMEOUT` | Prazo do graceful shutdown HTTP, no formato de duração de Go | `15s`; valores inválidos usam o padrão. |

Os manifestos de contêiner e implantação também interpolam `POSTGRES_DB`,
`POSTGRES_USER`, `POSTGRES_PASSWORD`, `API_PORT` e `API_REPLICAS`. `IMAGE_TAG`
identifica apenas o build local no Compose. O Stack exige `IMAGE_DIGEST`, no
formato `sha256:<64 caracteres hexadecimais>`, de uma imagem validada no GHCR.
Consulte os arquivos versionados antes de executar o ambiente desejado.

## Banco de dados e preparação do ambiente

A API realiza somente leituras. Schema, ingestão, publicação e migrations
pertencem ao projeto separado `b3-data-hub`. O ambiente precisa fornecer os dados
publicados e, para o calendário observado de Intelligence, as migrations 003/004
desse projeto. A API não deve tentar corrigir a preparação do banco.

As regras de responsabilidade estão no [núcleo operacional](operational-core.md).
As [convenções de desenvolvimento](../desenvolvimento.md) descrevem pool,
objetos consultados e garantias dos repositórios; o
[checklist de validação](validation-checklist.md) descreve os pré-requisitos das
integrações somente leitura.

## Docker

O [Dockerfile](../../Dockerfile) usa build em múltiplas etapas com
`golang:1.26-alpine` e `alpine:3.22`. Ele compila um binário Linux estático e o
executa com o usuário `app`, sem privilégios de root, na porta 8080.

Construa a imagem diretamente:

```sh
docker build --build-arg VERSION=development -t market-data-api .
```

O [Compose](../../docker-compose.yml) versionado constrói a API e exige uma rede
Docker externa existente chamada `b3-data-hub-network`, com PostgreSQL acessível
pelo nome `postgres`. Com esse pré-requisito e um `.env` configurado, execute:

```sh
docker compose up --build
```

O [manifesto de Stack](../../docker-stack.yml) implanta a imagem do GHCR no Docker
Swarm, exige redes externas chamadas `b3-data-hub_b3_data_hub` e `DockerNet` e
configura o roteamento Traefik para `marketdata.techcomp.net.br`. A referência é
`ghcr.io/jamersom/market-data-api@${IMAGE_DIGEST}`: não há tag padrão nem fallback
para `dev` ou `IMAGE_TAG`. Digest ausente ou vazio impede a renderização do Stack.
Não basta escolher qualquer digest: a imagem precisa ter passado pelos testes
operacionais e corresponder à revisão do manifesto usada na implantação.

O health check mantém intervalo de 30s, timeout de 3s, três tentativas e período
inicial de 5s. Atualização e rollback monitoram as tarefas por 120s, acima da
estimativa conservadora de detecção `5 + 3 × (30 + 3) = 104s`. Essa margem não
substitui a validação no Swarm; mudanças nos probes exigem revisar os dois
monitores. `HEALTH_CHECK_TIMEOUT` deve permanecer menor que os 3s do probe.
O procedimento de implantação e rollback está no
[checklist](validation-checklist.md#publicação-implantação-e-rollback-controlados).

Dockerfile, Compose e Stack verificam readiness em `/health/ready`. Siga o
[checklist operacional](validation-checklist.md) para validar a imagem,
indisponibilidade da dependência, encerramento e ambiente implantado. Os dados
da [medição de performance](performance-baseline.md) são um baseline local,
não uma validação de capacidade de produção.

### Recomendação de health check para o Swarm — decisão pendente

O health check do contêiner atua sobre seu ciclo de vida; não é apenas uma
sondagem de roteamento. Hoje, falhas persistentes de `/health/ready` causadas por
PostgreSQL podem tornar a tarefa não saudável e provocar substituição/reinícios.
Reiniciar a API não recupera o banco compartilhado. Com uma réplica, a substituição
pode prolongar a indisponibilidade e alcançar o limite de tentativas configurado.
O período inicial de 5s só protege a inicialização, não uma queda posterior.

Recomenda-se avaliar `/health/live` para o health check do contêiner e manter
`/health/ready` como verificação de disponibilidade no Traefik e no aceite de
implantação. Assim, a falha do banco retira o backend do roteamento sem exigir
reinício de um processo HTTP saudável. Com uma réplica não há backend alternativo:
continuará havendo indisponibilidade de negócio enquanto o banco estiver fora,
mas a instância poderá recuperar readiness sem uma substituição forçada.

Essa recomendação não foi aplicada ao Dockerfile, Compose ou Stack. A alteração
exige aprovação e validação com a versão/provider reais do Traefik. Seu health
check HTTP deve consultar `/health/ready`, exigir 200 e poder alcançar cada backend
na rede compartilhada. Não assumir sondagem individual se o provider usar o VIP
do Swarm. Após a troca proposta, um contêiner `healthy` só comprovaria liveness;
o aceite de publicação/deploy precisaria continuar exigindo readiness real.
A API exige PostgreSQL na inicialização: antes de iniciar HTTP, falha de conexão
ainda encerra o processo, mesmo usando liveness como probe do contêiner.

Referências: [ciclo de vida no Swarm](https://docs.docker.com/engine/swarm/services/),
[health check HTTP do Traefik](https://doc.traefik.io/traefik/reference/routing-configuration/http/load-balancing/service/#health-check)
e [configuração do provider Swarm](https://doc.traefik.io/traefik/reference/routing-configuration/other-providers/swarm/).

## CI/CD

O workflow [ci.yml](../../.github/workflows/ci.yml) é executado para pull requests
destinados à branch `dev`, por acionamento manual e por chamadas como workflow
reutilizável. Seu job de validação:

1. Faz o checkout do repositório.
2. Seleciona a versão de Go a partir de [go.mod](../../go.mod) e habilita o cache
   de dependências.
3. Executa `go mod download`.
4. Executa `go test ./...`.
5. Executa `go vet ./...`.
6. Verifica a sintaxe Bash e executa os testes de regressão do script de smoke,
   com Docker/HTTP simulados. Não são integrações PostgreSQL.
7. Valida o Stack com valores fictícios e exige digest não vazio, sem imprimir
   a configuração que contém credenciais.
8. Constrói e carrega a imagem Docker Linux/amd64 sem publicá-la.
9. Executa [smoke-image.sh](../../.github/scripts/smoke-image.sh) nessa imagem:
   liveness 200, readiness 200, health check da imagem saudável, readiness 503
   com PostgreSQL isolado, liveness ainda 200 e recuperação para readiness 200.

O workflow [publish-dev.yml](../../.github/workflows/publish-dev.yml) é executado
após pushes para `dev` ou por acionamento manual. Primeiro chama o workflow de
validação; depois constrói uma candidata `linux/amd64`, carrega-a no Docker e
repete os testes operacionais sobre seu ID local. Só então envia essa mesma
imagem ao GHCR, sem reconstruí-la, com as tags `sha-<short-sha>` e `dev`.
Injeta `dev-<short-sha>` em `main.version` pelo argumento de build Docker.
O resumo da execução informa a referência por digest e o valor de `IMAGE_DIGEST`
para implantação. Tags são referências de conveniência; a identidade usada pelo
Stack é o digest registrado na execução aprovada.

O smoke test usa PostgreSQL descartável, sem porta de banco publicada, em uma
rede própria. Não cria dados de mercado, não aplica migrations e não usa `.env`
nem banco compartilhado. Ele testa conectividade/Ping, não o schema COTAHIST.
Requer Docker Linux, Bash, curl e acesso à imagem `postgres:17-alpine`.

Os gatilhos continuam apontando para `dev`; não há merge automático de `develop`.
Antes de publicar, a revisão aprovada precisa conter tanto o núcleo operacional
quanto estes workflows e scripts. Um workflow antigo não adquire essa proteção
retroativamente. Publicação de imagem não atualiza o serviço Swarm.

### Concorrência de publicação — proteção proposta, não implementada

O workflow não define `concurrency`, portanto execuções simultâneas podem enviar
imagens e sobrescrever a tag móvel `dev` em ordens diferentes. O digest aprovado
continua sendo obrigatório no Stack. Propõe-se a menor proteção: `concurrency`
no job `publish`, com grupo fixo para `ghcr.io/jamersom/market-data-api` e
`cancel-in-progress: false`. O grupo deve ser comum a todos os refs que podem
publicar nesse destino; um grupo por branch não protege a tag compartilhada.
Manter o job em andamento evita cancelá-lo entre os dois pushes, que não são
atômicos. O grupo serializa publicações, mas não garante ordem por data de commit
nem impede sobrescrita por uma publicação manual antiga. A revisão e o digest
de cada execução ainda precisam ser aprovados.

O bloco de concorrência é somente uma proposta nesta etapa. Referência:
[concorrência no GitHub Actions](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency).

Antes de abrir um PR, execute a validação local completa descrita no
[checklist](validation-checklist.md): `gofmt` nos arquivos Go alterados,
`go test ./...`, `go vet ./...` e `go build ./...`.

Não há limite mínimo de cobertura configurado, linter independente, workflow
de release ou workflow automatizado de implantação visível neste repositório.
