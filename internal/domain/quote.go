package domain

import "time"

type Quote struct {
	// TradingDate representa a data do pregão em que a negociação ocorreu.
	// Campo DATA PREGAO (DATAPREG) do COTAHIST.
	TradingDate time.Time

	// BDICode identifica a classificação BDI do ativo na B3.
	// Exemplos: lote padrão, direito, recibo, fundo etc.
	// Campo CODBDI.
	BDICode string

	// Ticker é o código de negociação do ativo na B3.
	// Exemplos: PETR4, VALE3, ITUB4.
	// Campo CODNEG.
	Ticker string

	// MarketType identifica o tipo de mercado em que o ativo foi negociado.
	// Exemplos: mercado à vista, exercício de opções, termo etc.
	// Campo TPMERC.
	MarketType int

	// ShortName contém o nome resumido da empresa ou ativo.
	// Exemplo: PETROBRAS.
	// Campo NOMRES.
	ShortName string

	// Specification contém a especificação do papel.
	// Pode indicar características como ON, PN, PNA, UNT etc.
	// Campo ESPECI.
	Specification string

	// Term representa o prazo em dias para operações do mercado a termo.
	// Para outros tipos de mercado normalmente não possui informação relevante.
	// Campo PRAZOT.
	Term string

	// Currency identifica a moeda utilizada nos valores financeiros.
	// Normalmente BRL no COTAHIST.
	// Campo MODREF.
	Currency string

	// OpenPriceCents representa o preço de abertura do ativo no pregão.
	// Armazenado em centavos para evitar operações monetárias com float.
	// Campo PREABE.
	OpenPriceCents int64

	// HighPriceCents representa o maior preço negociado durante o pregão.
	// Armazenado em centavos.
	// Campo PREMAX.
	HighPriceCents int64

	// LowPriceCents representa o menor preço negociado durante o pregão.
	// Armazenado em centavos.
	// Campo PREMIN.
	LowPriceCents int64

	// AveragePriceCents representa o preço médio das negociações realizadas
	// durante o pregão.
	// Armazenado em centavos.
	// Campo PREMED.
	AveragePriceCents int64

	// ClosePriceCents representa o preço da última negociação do ativo
	// realizada no pregão.
	// Armazenado em centavos.
	// Campo PREULT.
	ClosePriceCents int64

	// BestBidPriceCents representa o preço da melhor oferta de compra
	// disponível ao final do pregão.
	// Armazenado em centavos.
	// Campo PREOFC.
	BestBidPriceCents int64

	// BestAskPriceCents representa o preço da melhor oferta de venda
	// disponível ao final do pregão.
	// Armazenado em centavos.
	// Campo PREOFV.
	BestAskPriceCents int64

	// TradeCount representa a quantidade total de negócios realizados
	// com o ativo durante o pregão.
	// Campo TOTNEG.
	TradeCount int

	// TradedQuantity representa a quantidade total de títulos ou contratos
	// negociados durante o pregão.
	// Campo QUATOT.
	TradedQuantity int64

	// TradedVolumeCents representa o volume financeiro total negociado
	// com o ativo durante o pregão.
	// Armazenado em centavos.
	// Campo VOLTOT.
	TradedVolumeCents int64

	// StrikePriceCents representa o preço de exercício da opção.
	// É relevante principalmente para instrumentos do mercado de opções.
	// Armazenado em centavos.
	// Campo PREEXE.
	StrikePriceCents int64

	// OptionIndicator identifica características relacionadas ao preço
	// de exercício de uma opção.
	// Campo INDOPC.
	OptionIndicator string

	// ExpirationDate representa a data de vencimento do instrumento,
	// principalmente opções e outros ativos com vencimento.
	// Campo DATVEN.
	ExpirationDate *time.Time

	// QuoteFactor representa o fator de cotação utilizado pela B3 para o ativo.
	// É usado na interpretação da unidade em que o preço é cotado.
	// Campo FATCOT.
	QuoteFactor int

	// StrikePointsScaled representa o preço de exercício expresso em pontos,
	// com a escala decimal utilizada pelo layout do COTAHIST.
	// Campo PTOEXE.
	StrikePointsScaled int64

	// ISIN contém o código ISIN (International Securities Identification Number),
	// identificador internacional único do instrumento financeiro.
	// Campo CODISI.
	ISIN string

	// DistributionNumber representa o número de distribuição do papel.
	// Permite identificar a distribuição associada ao instrumento.
	// Campo DISMES.
	DistributionNumber int
}
