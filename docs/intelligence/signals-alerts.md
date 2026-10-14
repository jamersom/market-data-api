# Sinais e Alertas Determinísticos

Status: Etapa 3 implementada em 10 de setembro de 2026 para as seis regras da
V1 e ruleset `1.0`. Alertas compostos e notificações permanecem fora do escopo.

## 1. Objetivo

Adicionar ao Asset Intelligence uma camada determinística de interpretação
dos indicadores técnicos já calculados pela aplicação.

Essa camada deve identificar condições técnicas observáveis e retornar
as evidências numéricas utilizadas em cada avaliação.

O objetivo não é produzir recomendações de investimento.

A implementação deve permanecer independente de LLMs.

---

## 2. Princípio arquitetural

O fluxo esperado é:

historical data
    ↓
calculations
    ↓
indicators
    ↓
rules evaluator
    ↓
signals / alerts
    ↓
HTTP response
    ↓
LLM (opcional, fora desta etapa)

As regras devem consumir indicadores já calculados pelo Asset Intelligence.

O mecanismo de sinais não deve recalcular:

- RSI;
- médias móveis;
- retornos;
- volatilidade;
- força relativa;
- outros indicadores técnicos existentes.

O handler HTTP não deve conter regras de negócio.

---

## 3. Escopo da V1

A primeira versão deve implementar somente regras técnicas:

- determinísticas;
- explicáveis;
- testáveis;
- baseadas em indicadores existentes.

Não implementar na V1:

- `buy`;
- `sell`;
- `strong_buy`;
- `strong_sell`;
- preço-alvo;
- recomendação de investimento;
- decisão produzida por LLM;
- notificações externas;
- assinatura de alertas;
- envio de e-mail, push, SMS ou webhook.

---

## 4. Definição de sinal

Um sinal representa uma condição técnica diretamente observável a partir
dos indicadores disponíveis.

Exemplos:

- RSI em região elevada;
- RSI em região baixa;
- preço acima de uma média;
- preço abaixo de uma média;
- relação entre médias móveis;
- momentum positivo ou negativo.

Um sinal não representa recomendação de compra ou venda.

---

## 5. Definição de alerta

Um alerta representa uma condição de maior relevância que pode ser
formada por uma ou mais regras/sinais.

Alertas podem possuir severidade.

A V1 deve priorizar sinais simples.

Alertas compostos somente devem ser adicionados quando suas regras
estiverem explicitamente especificadas.

Não inferir automaticamente alertas compostos a partir da combinação
de sinais.

---

## 6. Estados de avaliação

Cada regra deve possuir um estado explícito.

Estados:

- `triggered`;
- `not_triggered`;
- `unavailable`.

### `triggered`

A regra pôde ser avaliada e sua condição foi satisfeita.

### `not_triggered`

A regra pôde ser avaliada, mas sua condição não foi satisfeita.

### `unavailable`

A regra não pôde ser avaliada por ausência ou insuficiência dos dados
necessários.

É importante distinguir:

not_triggered != unavailable

A ausência de ocorrência de um sinal não pode ser confundida com a
impossibilidade de avaliá-lo.

---

## 7. Evidências

Toda regra avaliada deve retornar evidências numéricas suficientes para
explicar o resultado.

Exemplo:

{
  "id": "rsi_overbought",
  "status": "triggered",
  "severity": "warning",
  "evidence": {
    "rsi14": 74.8,
    "threshold": 70
  }
}

Outro exemplo:

{
  "id": "sma20_above_sma50",
  "status": "triggered",
  "severity": "info",
  "evidence": {
    "sma20": 48.22,
    "sma50": 46.53
  }
}

As evidências devem ser derivadas dos mesmos valores utilizados na
avaliação da regra.

Não retornar uma justificativa numérica diferente do valor efetivamente
utilizado pelo evaluator.

---

## 8. Ruleset

As regras devem pertencer a um conjunto versionado.

Versão inicial:

ruleset_version = "1.0"

A versão deve ser exposta nos metadados do Asset Intelligence quando
os detalhes correspondentes fizerem parte do contrato.

Exemplo conceitual:

{
  "meta": {
    "ruleset_version": "1.0"
  }
}

O nome final do campo deve seguir as convenções existentes do contrato.

---

## 9. Política de versionamento

A evolução do ruleset deve ser explícita.

### Major

Alterações incompatíveis na interpretação de uma regra existente.

Exemplos:

- alteração significativa da fórmula;
- mudança de significado;
- remoção de regra;
- mudança incompatível de threshold.

Exemplo:

1.x -> 2.0

### Minor

Inclusão de novas regras sem alterar o significado das regras
existentes.

Exemplo:

1.0 -> 1.1

### Patch

Correções que não alterem a intenção funcional das regras.

Exemplo:

1.0.0 -> 1.0.1

A implementação não precisa necessariamente utilizar uma biblioteca
SemVer, mas deve preservar essa política conceitual.

---

## 10. Regras da V1

### 10.1 RSI elevado

ID:

rsi_overbought

Condição:

RSI(14) >= 70

Tipo:

signal

Severidade:

warning

Evidências:

- `rsi14`;
- `threshold`.

---

### 10.2 RSI baixo

ID:

rsi_oversold

Condição:

RSI(14) <= 30

Tipo:

signal

Severidade:

warning

Evidências:

- `rsi14`;
- `threshold`.

---

### 10.3 Preço acima da SMA20

ID:

price_above_sma20

Condição:

price > sma20

Tipo:

signal

Severidade:

info

Evidências:

- `price`;
- `sma20`.

---

### 10.4 Preço abaixo da SMA20

ID:

price_below_sma20

Condição:

price < sma20

Tipo:

signal

Severidade:

info

Evidências:

- `price`;
- `sma20`.

---

### 10.5 SMA20 acima da SMA50

ID:

sma20_above_sma50

Condição:

sma20 > sma50

Tipo:

signal

Severidade:

info

Evidências:

- `sma20`;
- `sma50`.

---

### 10.6 SMA20 abaixo da SMA50

ID:

sma20_below_sma50

Condição:

sma20 < sma50

Tipo:

signal

Severidade:

info

Evidências:

- `sma20`;
- `sma50`.

---

## 11. Momentum

Os sinais:

- `positive_momentum`;
- `negative_momentum`;

não fazem parte da V1 até que a janela utilizada para definir momentum
seja aprovada.

Não assumir silenciosamente uma janela como:

- 7 pregões;
- 30 dias;
- 30 pregões;
- 90 dias.

A inclusão dessas regras exige definição funcional explícita da janela.

---

## 12. Thresholds

Os thresholds pertencem ao ruleset.

Na V1:

RSI_OVERBOUGHT = 70
RSI_OVERSOLD = 30

Esses valores não devem ficar espalhados pelo código como números
mágicos.

A implementação deve centralizá-los na representação apropriada do
ruleset.

Alterações futuras nesses thresholds devem respeitar a política de
versionamento.

---

## 13. Igualdade nas regras

As condições devem ser explicitamente definidas.

RSI:

rsi >= 70 -> rsi_overbought
rsi <= 30 -> rsi_oversold

Preço e médias:

price > sma20 -> price_above_sma20
price < sma20 -> price_below_sma20

Quando:

price == sma20

nenhuma das duas condições deve ser marcada como `triggered`.

O mesmo princípio se aplica a:

sma20 == sma50

Nesse cenário:

- `sma20_above_sma50` = `not_triggered`;
- `sma20_below_sma50` = `not_triggered`.

---

## 14. Indicadores indisponíveis

Quando um indicador necessário estiver indisponível, a regra dependente
dele deve ser:

status = unavailable

Exemplo:

RSI(14) indisponível:

{
  "id": "rsi_overbought",
  "status": "unavailable"
}

e:

{
  "id": "rsi_oversold",
  "status": "unavailable"
}

O motivo da indisponibilidade deve utilizar os mecanismos de metadata
já existentes no Asset Intelligence.

Não duplicar desnecessariamente informações de indisponibilidade em
diferentes partes do contrato.

---

## 15. Independência das regras

A indisponibilidade de uma regra não deve impedir a avaliação das
demais.

Exemplo:

RSI indisponível
       ↓
rsi_overbought = unavailable
rsi_oversold   = unavailable

SMA20 disponível
       ↓
price_above_sma20 continua sendo avaliada

Cada regra deve depender somente das entradas necessárias para sua
avaliação.

---

## 16. `asOf`

Os sinais devem representar exatamente os indicadores calculados para
o `asOf` da requisição.

Nenhum sinal pode utilizar dados posteriores ao `asOf`.

O rules evaluator não deve realizar consultas independentes ao histórico.

Ele deve consumir os indicadores já produzidos pelo Asset Intelligence.

Dessa forma, a responsabilidade de respeitar o corte temporal permanece
na camada responsável pelos cálculos.

---

## 17. Estrutura conceitual da resposta

Exemplo:

{
  "signals": [
    {
      "id": "rsi_overbought",
      "status": "not_triggered",
      "severity": "warning",
      "evidence": {
        "rsi14": 63.2,
        "threshold": 70
      }
    },
    {
      "id": "sma20_above_sma50",
      "status": "triggered",
      "severity": "info",
      "evidence": {
        "sma20": 48.22,
        "sma50": 46.53
      }
    }
  ]
}

Esse JSON é conceitual.

O DTO final deve seguir as convenções existentes no Asset Intelligence.

---

## 18. Resultado sem sinais acionados

Uma avaliação válida pode resultar em zero sinais acionados.

Isso não representa erro.

Também não representa indisponibilidade.

Exemplo conceitual:

{
  "signals": []
}

pode significar que todas as regras foram avaliadas e nenhuma condição
relevante ocorreu, caso o contrato opte por retornar apenas sinais
acionados.

Caso o contrato exponha todas as avaliações, as regras devem aparecer
como `not_triggered`.

A escolha entre:

- retornar todas as avaliações;
- retornar somente regras acionadas;

deve ser definida antes da implementação do DTO final.

### Recomendação para a V1

Retornar todas as avaliações.

Isso permite distinguir diretamente:

- `triggered`;
- `not_triggered`;
- `unavailable`;

e torna o resultado auditável.

---

## 19. Severidade

Severidades inicialmente suportadas:

- `info`;
- `warning`.

Na V1:

| Regra | Severidade |
|---|---|
| `rsi_overbought` | `warning` |
| `rsi_oversold` | `warning` |
| `price_above_sma20` | `info` |
| `price_below_sma20` | `info` |
| `sma20_above_sma50` | `info` |
| `sma20_below_sma50` | `info` |

Não utilizar `critical` na V1.

A introdução de `critical` exige definição funcional específica.

---

## 20. Arquitetura

As regras devem permanecer fora do adapter HTTP.

Estrutura conceitual:

internal/
├── domain/
│   └── signals/
│       ├── rule.go
│       ├── evaluator.go
│       └── ruleset_v1.go
│
├── application/
│
└── adapters/
    └── inbound/
        └── http/

A estrutura final deve respeitar a organização existente no projeto.

Não criar diretórios apenas para reproduzir este exemplo caso exista
uma localização mais adequada na arquitetura atual.

---

## 21. Determinismo

Para as mesmas entradas e mesma versão do ruleset, o resultado deve ser
o mesmo.

Formalmente:

indicators + ruleset_version
              ↓
       deterministic result

Nenhuma regra pode depender de:

- LLM;
- prompt;
- resposta textual;
- serviço generativo;
- aleatoriedade.

---

## 22. LLM

Modelos de linguagem podem futuramente receber o resultado dos sinais
para produzir explicações.

Exemplo:

Asset Intelligence
       ↓
signals + evidence
       ↓
Nexo / LLM
       ↓
explicação em linguagem natural

O LLM não deve:

- calcular RSI;
- calcular médias;
- determinar thresholds;
- decidir se uma regra foi acionada;
- alterar severidade;
- criar sinais não produzidos pelo ruleset.

---

## 23. Notificações

Este endpoint GET apenas avalia e retorna o estado atual.

Estão fora do escopo:

- monitoramento contínuo;
- assinatura de alertas;
- envio de e-mail;
- SMS;
- push notification;
- WhatsApp;
- webhook;
- filas de notificação;
- agendamento.

Essas funcionalidades podem ser implementadas futuramente por serviços
que consumam o resultado determinístico do Asset Intelligence.

---

## 24. Compatibilidade

A introdução dos sinais não deve alterar os valores existentes de:

- preço;
- RSI;
- médias móveis;
- retornos;
- volatilidade;
- força relativa, quando existente;
- percentis;
- demais indicadores.

O rules evaluator é consumidor desses valores, não produtor deles.

---

## 25. Testes obrigatórios

### RSI acima de 70

Esperado:

rsi_overbought = triggered
rsi_oversold = not_triggered

### RSI exatamente 70

Esperado:

rsi_overbought = triggered

### RSI abaixo de 30

Esperado:

rsi_oversold = triggered
rsi_overbought = not_triggered

### RSI exatamente 30

Esperado:

rsi_oversold = triggered

### RSI indisponível

Esperado:

rsi_overbought = unavailable
rsi_oversold = unavailable

### Preço acima da SMA20

Esperado:

price_above_sma20 = triggered
price_below_sma20 = not_triggered

### Preço abaixo da SMA20

Esperado:

price_below_sma20 = triggered
price_above_sma20 = not_triggered

### Preço igual à SMA20

Esperado:

price_above_sma20 = not_triggered
price_below_sma20 = not_triggered

### SMA20 acima da SMA50

Esperado:

sma20_above_sma50 = triggered
sma20_below_sma50 = not_triggered

### SMA20 abaixo da SMA50

Esperado:

sma20_below_sma50 = triggered
sma20_above_sma50 = not_triggered

### SMA20 igual à SMA50

Esperado:

ambas = not_triggered

### Indicadores parcialmente disponíveis

Confirmar que uma regra indisponível não impeça a avaliação das demais.

### `asOf`

Confirmar que os sinais correspondem aos indicadores calculados para
a data solicitada.

### Regressão

Confirmar que a introdução do rules evaluator não modifica os cálculos
existentes.

---

## 26. OpenAPI e documentação

Atualizar em conjunto:

- DTOs;
- OpenAPI;
- exemplos;
- estados possíveis;
- severidades;
- evidências;
- versão do ruleset;
- motivos de indisponibilidade aplicáveis.

A documentação deve deixar explícito que sinais são avaliações técnicas
determinísticas e não recomendações de investimento.

---

## 27. Fora de escopo

Não implementar nesta etapa:

- momentum sem janela aprovada;
- regras compostas não especificadas;
- recomendação de compra ou venda;
- score de investimento;
- previsão de preço;
- LLM;
- geração de texto;
- notificações;
- persistência de alertas;
- scheduler;
- mensageria de alertas;
- alterações nos cálculos existentes.

---

## 28. Critérios de aceite

A etapa estará concluída quando:

1. existir um ruleset determinístico e versionado;
2. a versão inicial for identificada como `1.0`;
3. as seis regras da V1 estiverem implementadas;
4. cada avaliação distinguir `triggered`, `not_triggered` e `unavailable`;
5. avaliações executadas retornarem evidências numéricas;
6. thresholds estiverem centralizados no ruleset;
7. regras não estiverem no handler HTTP;
8. regras consumirem indicadores já calculados;
9. uma regra indisponível não impedir as demais;
10. nenhum dado posterior ao `asOf` influenciar os sinais;
11. o resultado for independente de LLM;
12. nenhuma funcionalidade de notificação for introduzida;
13. DTOs, OpenAPI e exemplos estiverem atualizados;
14. os cálculos existentes permanecerem inalterados;
15. todos os testes aplicáveis passarem.

---

## 29. Evoluções futuras

Possíveis evoluções, fora da V1:

- `positive_momentum`;
- `negative_momentum`;
- sinais baseados em força relativa;
- regras compostas;
- diferentes níveis de severidade;
- múltiplas versões de ruleset;
- persistência de avaliações;
- serviço independente de alertas;
- notificações;
- interpretação por LLM.

Cada evolução deve possuir requisito funcional próprio antes da
implementação.
