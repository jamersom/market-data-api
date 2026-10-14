# Baseline operacional do Asset Intelligence

Medição realizada em 10 de setembro de 2026. Estes resultados são um baseline
local e não representam capacidade ou latência de produção.

## Ambiente

- Windows amd64, Intel Core i7-1365U.
- Docker Desktop 4.37.1, engine 27.4.0, containers Linux amd64.
- Imagem local `market-data-api:operational-core`.
- PostgreSQL local do `b3-data-hub`, na rede Docker `b3-data-hub-network`.
- Ativo PETR4, mercado padrão 10, dados publicados disponíveis no ambiente.

## Processamento sem PostgreSQL e HTTP

O benchmark `BenchmarkAssetIntelligenceLongHistory` usa um repositório em
memória, calendário sintético diário e preços constantes. Foram feitas três
execuções do benchmark para cada cenário.

| Cenário | Registros efetivos | Tempo por operação | Memória por operação | Alocações |
|---|---:|---:|---:|---:|
| referência mais recente | 2.500 | 0,598–0,913 ms | aproximadamente 1.007.800 B | 103 |
| `asOf=2021-12-31` | aproximadamente 2.189 | 0,803–0,987 ms | aproximadamente 1.001.700 B | 103 |

Comando:

```powershell
go test ./internal/application/services -run '^$' -bench '^BenchmarkAssetIntelligenceLongHistory$' -benchmem -count=3
```

Esse benchmark mede ordenação, validação, cálculos e avaliação dos sinais. Não
mede SQL, rede, decodificação das linhas pelo `pgx` nem serialização HTTP. O
tempo informado pelo benchmark é tempo decorrido por operação; consumo de CPU
isolado não foi medido.

## Requisição completa com PostgreSQL

Cada cenário foi executado cinco vezes contra o container na porta 18080. O
tempo do cliente inclui HTTP, consulta, processamento e JSON. A duração de banco
e a quantidade de linhas são deltas do endpoint `/metrics` para a operação
`intelligence_history`.

| Cenário | Cliente mínimo | Cliente médio | Cliente máximo | Banco total | Linhas totais | Chamadas |
|---|---:|---:|---:|---:|---:|---:|
| sem `asOf` | 31,63 ms | 45,36 ms | 94,38 ms | 161 ms | 13.275 | 5 |
| `asOf=2023-12-28` | 25,90 ms | 36,32 ms | 61,55 ms | 127 ms | 9.910 | 5 |

Médias derivadas: 32,2 ms de banco e 2.655 linhas por chamada sem `asOf`; 25,4
ms e 1.982 linhas por chamada com `asOf`. Esses totais foram coletados antes da
mudança do acumulador para milissegundos fracionários e possuem truncamento por
chamada; medições futuras não têm essa limitação.

Os números mostram que a consulta longa domina o tempo observado neste ambiente,
mas duas amostras de cinco execuções não justificam cache, materialização ou
mudança de arquitetura. Uma decisão de otimização exige repetição sob carga em
ambiente equivalente ao implantado, com distribuição de latência e uso de CPU.

Após um smoke test da imagem final, `/metrics` informou 3.649.456 bytes de heap
alocado e sete goroutines; `docker stats --no-stream` informou 14,91 MiB de
memória para o container. A leitura pontual de CPU foi 0,00% após a requisição e
não mede o custo de CPU durante o processamento, portanto não deve ser usada
como baseline de CPU sob carga.

## Repetição da medição

1. Construa a imagem e inicie uma instância isolada ligada a um PostgreSQL
   preparado pelo `b3-data-hub`.
2. Confirme `/health/ready` antes de medir.
3. Capture `/metrics`, execute pelo menos cinco chamadas por cenário e capture
   `/metrics` novamente.
4. Subtraia `calls`, `rows` e `total_duration_ms` da operação
   `intelligence_history`.
5. Registre ticker representativo, referência, versão da imagem, versão dos
   dados, CPU, memória, concorrência e ferramenta usada.
6. Repita com histórico recente, histórico longo, com `asOf`, sem `asOf` e sob
   concorrência representativa.

Não inclua ticker, `asOf`, SQL ou mensagens de erro como labels permanentes das
métricas. Esses valores pertencem ao registro controlado do experimento.
