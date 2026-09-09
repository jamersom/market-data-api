# Pendências do Asset Intelligence

Atualizado em: 2026-09-09.

A primeira versão de `GET /assets/{ticker}/intelligence` está implementada.
O [contrato atual](intelligence-contrato-atual.md) e o [OpenAPI](../openapi.yaml)
descrevem o comportamento disponível. Este documento lista o estado atual,
substituindo as pendências das etapas anteriores.

## Concluído

- Cálculos determinísticos: retorno de sete pregões, SMA20, distância da SMA20,
  RSI14 de Wilder, volatilidade de 30 retornos anualizada, drawdowns de 252
  pregões e volume financeiro médio de 20 pregões.
- Reutilização de cálculos por `/comparisons`.
- Caso de uso, portas, adaptador PostgreSQL, handler, mapper e registro da rota.
- Calendário observado consolidado do `b3-data-hub`, conectado em `cmd/main.go`.
  Histórico e calendário são lidos no mesmo snapshot somente leitura.
- Validação de cobertura e lacunas por janela, sem preencher preços ausentes.
- Contrato com apenas indicadores implementados: `complete` quando todos foram
  calculados; `partial` e motivos por campo quando há indisponibilidade.
- Remoção dos placeholders de recursos futuros do JSON.
- Metadados compactos: `includeDetails=true` inclui calendário, versões dos
  cálculos e dados e semente do RSI; `requested_as_of` aparece somente quando
  `asOf` é informado.
- OpenAPI 1.4.0, testes unitários, HTTP e integração somente leitura com PostgreSQL.
  Testes, vet e build passaram na validação anterior ao commit `eacbb08`.
- Commit `eacbb08` enviado para `origin/develop`.

O calendário observado satisfaz a política atual; uma fonte oficial independente
é uma evolução opcional. Veja [calendario-intelligence.md](calendario-intelligence.md).

## Próximas implementações, em ordem

1. **Percentil do RSI em três anos:** definir janela temporal, cobertura mínima,
   inclusão do valor atual e tratamento de empates; calcular a série histórica
   de RSI preservando a semente original e sem usar dados posteriores a `asOf`.
   Adicionar cálculo, integração, motivo de indisponibilidade, testes e OpenAPI.
2. **Benchmark e força relativa:** aceitar ticker de referência disponível na
   fonte, alinhar datas e calcular diferença de retornos em pontos percentuais.
3. **Sinais e alertas:** definir regras quantitativas versionadas com evidências.
   Notificações são uma implementação separada.
4. **Contexto e score:** definir universo, fontes, componentes, pesos,
   normalização e tratamento de ausências antes de escrever os cálculos.

Após cada implementação, parar para revisão antes de avançar.
A IA interpreta os resultados; não preenche lacunas nem substitui cálculos.

## Publicação e verificação operacional

O push foi confirmado; o deploy e a imagem em execução não foram verificados
nesta revisão. Confirmar a publicação da imagem e sua atualização no servidor,
validar o endpoint abaixo e o Swagger em `/docs/` e medir latência sob carga:

`https://marketdata.techcomp.net.br/assets/PETR4/intelligence`

O PostgreSQL de produção precisa das migrations 003/004 do `b3-data-hub` e de
importações publicadas com cobertura suficiente. Não confundir falta de dados
em um ambiente com falta de implementação do calendário.

## Evoluções condicionadas a fontes ou medições

- **Preços ajustados:** obter eventos corporativos e definir metodologia.
- **Cache/materialização:** medir necessidade; considerar versões dos dados,
  calendário e cálculos na chave. O hash dos dados não inclui o calendário.
- **Calendário oficial independente:** pode ampliar a verificação de cobertura;
  o observado não prova que todo dia ausente seja feriado.

## Limitações atuais

- `asOf` limita pregões, mas usa a versão atualmente publicada do histórico.
- Correções ou importações anteriores podem mudar o RSI e a versão dos dados.
- O serviço carrega todo o histórico para manter a inicialização do RSI.
- Preços são não ajustados; os indicadores não representam retorno total.

## Documentos relacionados

- [Contrato atual](intelligence-contrato-atual.md).
- [Caso de uso e PostgreSQL](asset-intelligence-application.md).
- [Convenções dos cálculos](analytics.md).
- [Proposta original, incluindo evoluções futuras](endpoint-asset-intelligence.md).
