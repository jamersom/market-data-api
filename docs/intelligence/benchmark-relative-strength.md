# Benchmark e Força Relativa

> Status: implementado no Asset Intelligence com a janela vigente de sete
> pregões. O contrato efetivo está no
> [contrato atual](../intelligence-contrato-atual.md) e no
> [OpenAPI](../../openapi.yaml).

## 1. Objetivo

Adicionar ao Asset Intelligence a capacidade de comparar o desempenho
de um ativo com um benchmark.

O endpoint de comparações já aceita um benchmark e pode retornar seu
resumo. Esse comportamento, porém, não representa força relativa.

O Asset Intelligence atualmente não aceita `benchmark` em seu contrato.

Esta especificação define o comportamento esperado para a introdução
desse recurso.

---

## 2. Escopo

A implementação deve permitir:

- informar opcionalmente um benchmark no Asset Intelligence;
- calcular o retorno do ativo e do benchmark sobre períodos comparáveis;
- calcular a força relativa entre eles;
- informar quando a força relativa não puder ser calculada;
- preservar os demais indicadores do ativo quando o benchmark estiver
  indisponível.

O benchmark não deve alterar cálculos independentes já existentes.

---

## 3. Benchmark

O Asset Intelligence deve aceitar um parâmetro opcional:

`benchmark`

Exemplo:

GET /assets/PETR4/intelligence?benchmark=IBOV&asOf=2026-09-09

O identificador deve seguir as regras de normalização e validação
compatíveis com a fonte de dados e com os padrões existentes no projeto.

A lista ou conjunto de identificadores válidos depende da cobertura
existente na fonte de dados.

Não se deve assumir que qualquer string representa um benchmark válido.

---

## 4. Benchmark igual ao ativo

O ativo não deve ser comparado com ele mesmo.

Exemplo:

GET /assets/PETR4/intelligence?benchmark=PETR4

Esse cenário deve ser tratado explicitamente.

Não retornar silenciosamente força relativa igual a `0`.

O tratamento HTTP deve seguir o padrão de validação já adotado pelo
Asset Intelligence.

---

## 5. Janelas de comparação

A força relativa deve utilizar as janelas de retorno suportadas pelo
contrato vigente do Asset Intelligence.

Não devem ser criadas janelas adicionais apenas para o benchmark.

A implementação interna deve evitar regras específicas por janela.

Preferir uma abstração que permita calcular a mesma métrica para
diferentes períodos.

---

## 6. Definição de retorno

Para cada janela, devem ser calculados separadamente:

- retorno do ativo;
- retorno do benchmark.

O retorno percentual deve seguir a fórmula e as convenções já utilizadas
pelo pacote compartilhado de cálculos.

A implementação de benchmark não deve introduzir uma segunda fórmula
de retorno.

---

## 7. Definição de força relativa

A força relativa representa a diferença entre o retorno percentual do
ativo e o retorno percentual do benchmark no mesmo intervalo.

Fórmula:

relative_strength_pp =
asset_return_pct - benchmark_return_pct

A unidade é:

pontos percentuais

Exemplo:

Ativo:

+12,50%

Benchmark:

+7,20%

Força relativa:

+5,30 p.p.

Portanto:

relative_strength_pp = 12,50 - 7,20 = 5,30

---

## 8. Interpretação

Valor positivo:

relative_strength_pp > 0

significa que o ativo apresentou retorno superior ao benchmark no
período comparado.

Valor negativo:

relative_strength_pp < 0

significa que o ativo apresentou retorno inferior ao benchmark.

Valor igual a zero significa retornos equivalentes dentro da precisão
utilizada pelo cálculo.

Essa informação não representa recomendação de compra ou venda.

---

## 9. Datas comparáveis

Ativo e benchmark somente podem ser comparados quando os retornos forem
calculados utilizando datas inicial e final compatíveis.

A comparação deve utilizar:

- mesma data inicial;
- mesma data final.

Não é permitido comparar retornos produzidos sobre intervalos diferentes.

### Exemplo inválido

Ativo:

2026-08-01 -> 2026-09-09

Benchmark:

2026-08-04 -> 2026-09-09

Os dois retornos não devem ser diretamente comparados.

A implementação deve determinar um intervalo comum antes de calcular a
força relativa.

---

## 10. Datas ausentes

Não interpolar preços.

Não criar artificialmente observações para:

- finais de semana;
- feriados;
- pregões sem dados;
- datas ausentes na fonte.

Somente observações reais disponíveis na fonte podem participar do
cálculo.

Quando não houver datas suficientes para produzir um intervalo
comparável, a força relativa correspondente deve ficar indisponível.

---

## 11. `asOf`

O benchmark deve respeitar o mesmo `asOf` utilizado para o ativo.

Nenhuma observação com:

trading_date > asOf

pode participar:

- da seleção das datas;
- do cálculo do retorno;
- do alinhamento entre ativo e benchmark;
- do cálculo da força relativa.

Essa restrição também se aplica a consultas históricas.

Exemplo:

GET /assets/PETR4/intelligence?benchmark=IBOV&asOf=2023-12-28

O cálculo deve utilizar exclusivamente informações disponíveis até
`2023-12-28`.

---

## 12. Consulta de histórico

A implementação deve reutilizar, sempre que possível, a porta de
histórico existente.

Deve-se evitar uma consulta ao banco para cada:

- indicador;
- janela;
- cálculo de força relativa.

O objetivo é obter o histórico necessário do ativo e do benchmark e
utilizar o pacote compartilhado de cálculos para derivar os indicadores.

A introdução do benchmark não deve criar uma arquitetura paralela de
consulta de dados.

---

## 13. Cobertura insuficiente

A ausência ou insuficiência de dados do benchmark não deve impedir o
cálculo dos indicadores que dependem somente do ativo.

Por exemplo, quando aplicável ao contrato vigente:

- RSI;
- médias móveis;
- volatilidade;
- retornos do ativo;
- demais indicadores independentes.

Se o benchmark não possuir dados suficientes para determinada janela,
somente os indicadores dependentes dele devem ficar indisponíveis.

---

## 14. Estado da resposta

A implementação deve respeitar os critérios vigentes de:

- `complete`;
- `partial`;
- `unavailable`.

A introdução do benchmark não deve redefinir silenciosamente esses
conceitos.

Somente indicadores pertencentes ao contrato vigente devem participar
da determinação desses estados.

Recursos futuros não implementados não devem tornar uma resposta
`partial`.

---

## 15. Metadados de indisponibilidade

Quando uma força relativa não puder ser calculada, a resposta deve
utilizar o mecanismo de indisponibilidade já existente no Asset
Intelligence.

Os motivos devem permitir distinguir, quando aplicável, situações como:

- benchmark inexistente;
- histórico insuficiente;
- ausência de datas comparáveis;
- benchmark igual ao ativo.

Os nomes finais dos motivos devem seguir as convenções existentes no
projeto.

Não expor detalhes internos de banco de dados ou implementação.

---

## 16. Arredondamento e precisão

Não arredondar valores durante etapas intermediárias do cálculo.

Os retornos devem ser calculados utilizando a precisão adotada pelo
pacote compartilhado de cálculos.

A força relativa deve ser calculada a partir desses valores sem
arredondamento intermediário.

O arredondamento para exposição deve seguir a convenção já utilizada
pelo contrato HTTP.

Não introduzir uma nova política de arredondamento exclusivamente para
o benchmark.

---

## 17. Contrato HTTP

O formato final deve seguir as convenções atuais dos DTOs do Asset
Intelligence.

Conceitualmente, a resposta poderá representar:

{
  "data": {
    "ticker": "PETR4",
    "benchmark": {
      "ticker": "IBOV",
      "relative_strength": {
        "30d": 5.30
      }
    }
  }
}

Esse JSON é ilustrativo e não define, isoladamente, o DTO final.

A implementação deve primeiro avaliar o contrato vigente e preservar sua
consistência.

A unidade da força relativa deve estar documentada como pontos
percentuais, utilizando nomenclatura compatível com as convenções da API.

---

## 18. Independência dos indicadores

A introdução do benchmark não deve modificar os resultados dos
indicadores existentes.

Sem `benchmark`, o Asset Intelligence deve continuar apresentando o
mesmo comportamento anterior a esta funcionalidade.

Com `benchmark`, os cálculos independentes do ativo devem continuar
utilizando exatamente as mesmas regras.

---

## 19. Rankings e percentis

O benchmark não deve entrar automaticamente em:

- rankings;
- populações utilizadas em rankings;
- percentis;
- cálculos estatísticos não relacionados à força relativa.

A presença do parâmetro `benchmark` não deve modificar silenciosamente
esses cálculos.

Qualquer utilização futura do benchmark em rankings ou percentis exige
requisito funcional separado.

---

## 20. Cenários obrigatórios de teste

### Benchmark válido

Ativo e benchmark possuem histórico suficiente e datas comparáveis.

Validar:

relative_strength_pp =
asset_return_pct - benchmark_return_pct

### Benchmark inexistente

Validar o comportamento definido pelo contrato.

Os indicadores independentes não devem produzir resultados incorretos
por causa da ausência do benchmark.

### Benchmark sem cobertura suficiente

Não produzir força relativa utilizando períodos incompatíveis.

Os indicadores independentes devem permanecer disponíveis quando
possível.

### Benchmark igual ao ativo

O cenário deve ser tratado explicitamente.

Não retornar silenciosamente força relativa `0`.

### Datas desalinhadas

Criar históricos em que ativo e benchmark possuam datas diferentes.

Confirmar que somente intervalos efetivamente comparáveis sejam
utilizados.

### `asOf`

Disponibilizar nos dados de teste observações posteriores ao `asOf`.

Confirmar que nenhuma delas participe do resultado.

### Regressão dos indicadores existentes

Confirmar que a introdução do benchmark não altere os resultados dos
indicadores atuais.

### Rankings

Confirmar que o benchmark não entre em rankings ou altere suas
populações.

---

## 21. Fora de escopo

Esta etapa não deve:

- modificar o cálculo do RSI;
- modificar fórmulas de médias móveis;
- modificar o cálculo existente de volatilidade;
- redefinir retornos existentes;
- alterar rankings;
- incluir benchmark em rankings;
- introduzir chamadas a LLM;
- interpolar preços;
- utilizar dados posteriores ao `asOf`;
- criar consultas individuais por indicador;
- antecipar funcionalidades futuras não aprovadas.

---

## 22. Documentação

A implementação deve atualizar conjuntamente:

- DTOs;
- OpenAPI;
- exemplos;
- descrição do parâmetro `benchmark`;
- unidade da força relativa;
- comportamento de indisponibilidade;
- comportamento relacionado a `asOf`.

A documentação deve distinguir claramente o contrato implementado de
possíveis evoluções futuras.

---

## 23. Critérios de aceite

A implementação estará concluída quando:

1. O Asset Intelligence aceitar `benchmark`.
2. O benchmark for validado segundo as regras do projeto.
3. Ativo e benchmark forem comparados somente em intervalos com datas
   compatíveis.
4. A força relativa for calculada como diferença entre os retornos em
   pontos percentuais.
5. Nenhum dado posterior ao `asOf` participar do cálculo.
6. Indicadores independentes permanecerem disponíveis quando o benchmark
   estiver indisponível, conforme o contrato vigente.
7. As indisponibilidades relacionadas ao benchmark forem representadas
   pelos metadados existentes.
8. O benchmark não modificar rankings, percentis ou outros cálculos
   independentes.
9. DTOs, OpenAPI e exemplos refletirem o novo contrato.
10. Os testes aplicáveis passarem.
