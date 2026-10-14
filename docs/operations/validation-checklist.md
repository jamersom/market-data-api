# Validação local e operacional reproduzível

Siga as [instruções para agentes](../../AGENTS.md) e execute os comandos a partir
da raiz do repositório. Consulte o [guia operacional](development-guide.md) para
compilação, execução, configuração, Docker e CI/CD, e as
[convenções de desenvolvimento](../desenvolvimento.md) para organização e
cobertura dos testes.

## Validação Go

Após alterar código Go, execute `gofmt` nos arquivos alterados, `go test ./...`
e `go vet ./...`, ou informe claramente o que não pôde ser executado. Execute
também `go build ./...` quando a mudança puder afetar a compilação ou a conexão
das dependências. Antes de abrir um PR, execute os quatro passos:

```sh
gofmt -w <arquivos-go-alterados>
go test ./...
go vet ./...
go build ./...
```

A suíte regular é `go test ./...`. Ela pula os testes dependentes do banco,
a menos que as variáveis de habilitação estejam definidas. Registre quais
verificações foram executadas, seus resultados e as limitações; um teste
ignorado não deve ser reportado como aprovado.

## Integração PostgreSQL

Os testes de integração são habilitados explicitamente e usam dados publicados
existentes. Eles não criam massas de dados nem modificam o banco. Exigem banco
configurado e históricos publicados de PETR4 e VALE3:

```powershell
$env:DATABASE_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -count=1 -v
Remove-Item Env:DATABASE_INTEGRATION_TEST
```

O teste de integração do repositório de cotações exige especificamente o `.env`
da raiz. O teste de comparação aceita `.env` ou uma `DATABASE_URL` já definida.

## Integração do calendário observado

A integração com o calendário observado é habilitada separadamente:

```powershell
$env:CALENDAR_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -run '^TestObservedIntelligenceIntegration$' -count=1 -v
Remove-Item Env:CALENDAR_INTEGRATION_TEST
```

Ela exige histórico de PETR4 e as views de calendário observado fornecidas pelas
migrations 003/004 do `b3-data-hub`. Também realiza somente leituras; a preparação
do schema e dos dados pertence ao `b3-data-hub`. Consulte o
[contrato atual de Intelligence](../intelligence-contrato-atual.md) e o
[calendário observado](../calendario-intelligence.md) para detalhes.

## Pré-condições da validação operacional

Execute em ambiente descartável ou equivalente a produção, com migrations,
calendário e dados previamente publicados pelo `b3-data-hub`.

## Build e inicialização

```powershell
docker build --build-arg VERSION=validation -t market-data-api:validation .
docker run -d --name market-data-api-validation --env-file .env -p 18080:8080 market-data-api:validation
docker inspect --format '{{json .State.Health}}' market-data-api-validation
```

O `DATABASE_URL` usado dentro do container precisa apontar para um hostname
alcançável na rede Docker. Não grave credenciais no comando ou no repositório.

## Smoke tests

```powershell
curl.exe -i http://127.0.0.1:18080/health/live
curl.exe -i http://127.0.0.1:18080/health/ready
curl.exe -i "http://127.0.0.1:18080/assets/PETR4/intelligence?includeDetails=true"
curl.exe -i http://127.0.0.1:18080/openapi.yaml
curl.exe -i http://127.0.0.1:18080/docs/
curl.exe -i http://127.0.0.1:18080/metrics
```

Substitua PETR4 por um ativo publicado no ambiente. Intelligence e readiness
realizam somente leituras.

## PostgreSQL indisponível

Use apenas a instância descartável da API. Mantenha uma segunda rede para acesso
HTTP, remova temporariamente a rede que alcança o PostgreSQL e teste:

```powershell
docker network connect bridge market-data-api-validation
docker network disconnect <rede-do-banco> market-data-api-validation
curl.exe -sS -o NUL -w "live=%{http_code}\n" http://127.0.0.1:18080/health/live
curl.exe -sS -o NUL -w "ready=%{http_code}\n" http://127.0.0.1:18080/health/ready
docker network connect <rede-do-banco> market-data-api-validation
```

O resultado esperado é liveness 200 e readiness 503. Não interrompa um banco
compartilhado apenas para executar esse teste.

## Encerramento

```powershell
docker logs -f market-data-api-validation
docker stop -t 20 market-data-api-validation
```

Confirme os logs `HTTP shutdown requested`, `HTTP graceful shutdown completed`
e `PostgreSQL connection pool closed`. O timeout passado ao runtime deve ser
maior ou igual a `SHUTDOWN_TIMEOUT`.

## Ambiente implantado

Repita os smoke tests pela URL e pelo roteamento reais. Registre a versão
imutável da imagem, o ambiente, os status HTTP, horários e resultados. Verifique
TLS, roteamento, readiness usado pelo orquestrador, atualização e rollback. A
execução local não valida esses itens do ambiente implantado.
