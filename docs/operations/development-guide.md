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
`POSTGRES_USER`, `POSTGRES_PASSWORD`, `API_PORT`, `API_REPLICAS` e `IMAGE_TAG`.
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
configura o roteamento Traefik para `marketdata.techcomp.net.br`. O repositório
não documenta um nome padrão de stack nem um comando automatizado de implantação.

Dockerfile, Compose e Stack verificam readiness em `/health/ready`. Siga o
[checklist operacional](validation-checklist.md) para validar a imagem,
indisponibilidade da dependência, encerramento e ambiente implantado. Os dados
da [medição de performance](performance-baseline.md) são um baseline local,
não uma validação de capacidade de produção.

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
6. Constrói a imagem Docker Linux/amd64 sem publicá-la.

O workflow [publish-dev.yml](../../.github/workflows/publish-dev.yml) é executado
após pushes para `dev` ou por acionamento manual. Primeiro chama o workflow de
validação; depois constrói e publica imagens `linux/amd64` no GHCR com a tag
imutável `sha-<short-sha>` e a tag atualizável `dev`. Injeta `dev-<short-sha>` em
`main.version` por meio do argumento de build Docker.

Antes de abrir um PR, execute a validação local completa descrita no
[checklist](validation-checklist.md): `gofmt` nos arquivos Go alterados,
`go test ./...`, `go vet ./...` e `go build ./...`.

Não há limite mínimo de cobertura configurado, linter independente, workflow
de release ou workflow automatizado de implantação visível neste repositório.
