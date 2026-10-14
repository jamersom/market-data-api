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

## Smoke test da imagem candidata sem dados de mercado

Antes de usar Docker, valide a lógica do script com Bash:

```sh
bash -n .github/scripts/smoke-image.sh
bash -n .github/scripts/smoke-image-test.sh
bash .github/scripts/smoke-image-test.sh
```

Os 12 casos automatizados usam funções que substituem Docker, curl e sleep
somente no processo filho. Cobrem sucesso, retry de conexão transitória, 404
com corpo aparentemente correto, timeout (28), conexão (7), DNS (6), transferência
incompleta (18), corpo incorreto, readiness inicial 503, ausência do 503 na queda,
falha de recuperação e limpeza após falha parcial de criação. Casos de transporte
retornam intencionalmente corpo e status corretos, mas exit code não zero:
nenhuma dessas respostas pode aprovar o teste. A limpeza é verificada por IDs
em sucesso e falha. Esses testes não usam contêineres nem PostgreSQL reais.

O smoke test exige separadamente: exit code zero do curl, status HTTP esperado
e corpo esperado. HTTP 503 é um resultado esperado para readiness durante a
queda; por isso não se usa `curl --fail`. Erros de transporte e respostas fora
do contrato podem ser tentados novamente, mas nunca contam como sucesso.

Em Docker Linux com Bash e curl, execute a partir da raiz:

```sh
docker build --platform linux/amd64 --build-arg VERSION=validation -t market-data-api:validation .
bash .github/scripts/smoke-image.sh market-data-api:validation
```

O script fixa o ID da imagem local, inicia um PostgreSQL descartável sem porta
publicada e uma API acessível somente por loopback. Confirma `/health/live` 200,
`/health/ready` 200 e o health check da imagem; desconecta apenas o banco criado
pelo teste, exige readiness 503 e liveness 200 e reconecta para exigir readiness
200 novamente. Remove seus containers, volumes anônimos e rede ao terminar.
Não usa `.env`, não aplica migrations nem cria massas de mercado. Essa validação
operacional não substitui as integrações com dados publicados descritas acima.

O workflow de publicação repete esse teste na candidata final e envia a mesma
imagem testada, sem rebuild. Falha no teste impede os passos de push.

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

## Publicação, implantação e rollback controlados

Os comandos abaixo são um procedimento para execução autorizada, não parte da
validação local. Publicação e deploy precisam de autorização própria. Execute os
comandos de Swarm em um manager do ambiente aprovado, com Bash.

1. Revise explicitamente os commits que serão publicados. A revisão precisa
   conter `/health/live` e `/health/ready`, o manifesto com digest obrigatório,
   os workflows atualizados, o smoke test corrigido e seus testes, e o Dockerfile
   compatível com essas rotas e com o probe aprovado. Todos precisam pertencer
   ao mesmo commit/revisão aprovada; nomes de branches ou tags não comprovam isso.
   Não faça merge automático entre `develop` e `dev`. Acionar um workflow antigo
   em `dev` não valida as alterações existentes apenas em `develop`.
2. Após autorização de publicação, execute o workflow da revisão aprovada.
   Exija sucesso dos testes Go e do smoke test da candidata final. Registre o
   commit, a execução, a referência `ghcr.io/jamersom/market-data-api@sha256:...`
   e o `IMAGE_DIGEST` informado no resumo. Não reconstrua a imagem entre teste e
   publicação e não use a tag móvel `dev` na implantação.
3. Antes do deploy, registre a imagem atualmente executada e guarde a revisão
   anterior do manifesto e suas configurações em local protegido. Não copie
   dumps completos do serviço, pois podem conter credenciais. Defina
   `STACK_NAME` como o nome real da stack; não há nome padrão neste projeto:

   ```sh
   docker service inspect --format '{{.Spec.TaskTemplate.ContainerSpec.Image}}' "${STACK_NAME:?}_api"
   ```

4. Configure `IMAGE_DIGEST` com o digest aprovado e as variáveis `POSTGRES_*`
   pelo mecanismo seguro do ambiente. O Stack não carrega `.env`
   automaticamente. Confira formato, imagem e manifesto sem imprimir segredos:

   ```sh
   set -euo pipefail
   [[ "${IMAGE_DIGEST:?}" =~ ^sha256:[a-f0-9]{64}$ ]]
   docker pull "ghcr.io/jamersom/market-data-api@${IMAGE_DIGEST}"
   bash .github/scripts/smoke-image.sh "ghcr.io/jamersom/market-data-api@${IMAGE_DIGEST}"
   docker stack config --compose-file docker-stack.yml > /dev/null
   ```

   Execute o smoke test em host de validação com Docker Linux, antes do deploy.
   Confirme também redes externas, arquitetura linux/amd64, acesso ao GHCR e
   migrations/dados publicados. Valide Intelligence em ambiente equivalente
   usando a imagem por digest. Um digest fixa a imagem, mas não prova compatibilidade.

5. Somente após autorização de deploy, aplique o manifesto da revisão aprovada:

   ```sh
   docker stack deploy --with-registry-auth --compose-file docker-stack.yml "${STACK_NAME:?}"
   docker service ps "${STACK_NAME}_api"
   docker service inspect --format '{{json .UpdateStatus}}' "${STACK_NAME}_api"
   docker service inspect --format '{{.Spec.TaskTemplate.ContainerSpec.Image}}' "${STACK_NAME}_api"
   ```

   Aguarde a conclusão da atualização e toda a janela de monitoramento de 120s.
   Confirme o digest efetivo, tarefas saudáveis e smoke tests pela URL real.
   Valide previamente em Swarm descartável que uma candidata com readiness
   falhando provoca rollback dentro da janela; não provoque falhas em produção.

6. Se a atualização falhar, verifique primeiro se houve rollback automático.
   Quando necessário e autorizado, restaure a especificação anterior completa:

   ```sh
   docker service update --rollback "${STACK_NAME:?}_api"
   docker service ps "${STACK_NAME}_api"
   docker service inspect --format '{{json .UpdateStatus}}' "${STACK_NAME}_api"
   ```

   Confirme o digest anterior e o comportamento esperado daquela versão.
   Não aplique apenas um digest antigo ao manifesto novo: a versão anterior pode
   não oferecer `/health/ready`. O rollback precisa restaurar também os probes e
   demais configurações compatíveis. O Swarm guarda a especificação da atualização
   anterior, não um histórico ilimitado. Na primeira implantação não há versão
   anterior; após múltiplas atualizações, use o manifesto anterior preservado e
   seu digest, com nova autorização. Nenhuma migration é revertida pela API.

## Registro da validação local do P0-01

Em 14 de outubro de 2026, no checkout local de `develop`:

| Verificação | Resultado |
|---|---|
| `go test ./... -count=1`, `go vet ./...`, `go build ./...` | Aprovados; integrações PostgreSQL/calendário desabilitadas. |
| `docker stack config` com digest e valores fictícios | Aprovado; atualização e rollback renderizados com monitor de 2 minutos. |
| Digest ausente ou vazio, inclusive com `IMAGE_TAG=dev` | Renderização rejeitada, conforme esperado. |
| Sintaxe Bash do smoke test e `git diff --check` | Aprovados. |
| Fluxo do script com Docker/HTTP simulados | Aceitou 200/503/recuperação e rejeitou rota 404. Não é teste de container. |
| Imagem candidata, PostgreSQL real disponível/indisponível e rollback Swarm | Não executados: daemon Docker indisponível, inclusive fora do sandbox. |
| Workflows no GitHub e publicação | Não executados; dependem de autorização. |

Os testes existentes dos handlers confirmam liveness 200, readiness 200/503 e
timeout usando pinger simulado. Isso não comprova conectividade PostgreSQL real.
Nenhuma imagem foi construída ou publicada nesta validação; não há novo digest
aprovado para implantação. Execute o smoke test real e a validação de Swarm em
ambiente descartável antes de considerar a correção validada operacionalmente.

### Continuação: correção de transporte do smoke test

O `|| true` que descartava o exit code do curl foi removido. A resposta só é
analisada após transferência bem-sucedida; status e corpo são comparados
separadamente. As 30 tentativas foram preservadas, assim como o trap que limpa
os recursos criados pelo script. O CI passa a executar testes automatizados de
regressão dessa lógica antes do smoke com a imagem real.

A recomendação de separar liveness do contêiner e readiness do roteamento, e a
proposta de serializar o job de publicação, estão no guia operacional. Nenhum
probe ou bloco de concorrência foi alterado nesta etapa. Testes simulados não
substituem os pendentes de Docker/PostgreSQL, GitHub Actions e Swarm/Traefik.

Na validação desta continuação, sintaxe Bash, os 12 casos de regressão simulados
e `git diff --check` passaram. O cliente Docker 27.4.0 não encontrou o daemon
Linux, inclusive fora do sandbox; build e smoke da imagem exata, PostgreSQL real
disponível/indisponível, roteamento Traefik e rollback Swarm não foram executados.
Não houve alteração Go que justificasse repetir a suíte, vet ou build nesta etapa.
Nenhum workflow remoto foi acionado e não há novo digest aprovado para publicação.
