package exapi

import (
	"strings"
)

// 币对基本信息
type SymbolSetting struct {
	Symbol      string  `json:"symbol"`       // 币对名称
	Base        string  `json:"base"`         // 基础币种
	Quote       string  `json:"quote"`        // 报价币种
	MinSize     float64 `json:"min_size"`     // 最小交易量
	MinPrice    float64 `json:"min_price"`    // 最小价格
	MinNotional float64 `json:"min_notional"` // 最小成交量，价格和数量相乘必须要大于此值
	MakerFee    float64 `json:"maker_fee"`    // 挂单手续费
	TakerFee    float64 `json:"taker_fee"`    // 吃单手续费
}

func GetCurrencyMap(ssm map[string]SymbolSetting) (cm map[string]struct{}) {
	if len(ssm) == 0 {
		return nil
	}

	cm = make(map[string]struct{})
	for _, v := range ssm {
		cm[v.Base] = struct{}{}
		cm[v.Quote] = struct{}{}
	}

	return cm
}

func IntersectSymbols(a, b map[string]SymbolSetting) (symbols map[string]SymbolSetting) {
	symbols = make(map[string]SymbolSetting)
	for ka, _ := range a {
		if _, ok := b[ka]; ok {
			symbols[ka] = a[ka]
		}
	}

	return symbols
}

func IntersectCurrencies(a, b map[string]struct{}) (currencies map[string]struct{}) {
	currencies = make(map[string]struct{})
	for ka, _ := range a {
		if _, ok := b[ka]; ok {
			currencies[ka] = a[ka]
		}
	}

	return currencies
}

func NewCurrencyPairFromString(market string) CurrencyPair {
	symbols := strings.Split(market, "/")
	if len(symbols) != 2 {
		return CurrencyPair{}
	}

	return CurrencyPair{NewCurrency(symbols[0]), NewCurrency(symbols[1])}
}

func NewCurrencyPair(stock Currency, money Currency) CurrencyPair {
	return CurrencyPair{stock, money}
}

type CurrencyPair struct {
	Stock Currency `json:"stock"`
	Money Currency `json:"money"`
}

func (pair CurrencyPair) String() string {
	return pair.ToSymbol("_")
}

func (pair CurrencyPair) Equal(c2 CurrencyPair) bool {
	return pair.String() == c2.String()
}

func (pair CurrencyPair) ToSymbol(sep string) string {
	return pair.Stock.Symbol() + sep + pair.Money.Symbol()
}

func (pair CurrencyPair) ToLowerSymbol(sep string) string {
	return strings.ToLower(pair.ToSymbol(sep))
}

func (pair CurrencyPair) Reverse() CurrencyPair {
	return CurrencyPair{pair.Money, pair.Stock}
}

func (pair CurrencyPair) AdaptUsdtToUsd() CurrencyPair {
	Money := pair.Money
	if pair.Money.Equal(USDT) {
		Money = USD
	}
	return CurrencyPair{pair.Stock, Money}
}

func (pair CurrencyPair) AdaptUsdToUsdt() CurrencyPair {
	Money := pair.Money
	if pair.Money.Equal(USD) {
		Money = USDT
	}
	return CurrencyPair{pair.Stock, Money}
}

//It is currently applicable to binance and zb
func (pair CurrencyPair) AdaptBchToBcc() CurrencyPair {
	Stock := pair.Stock
	if pair.Stock.Equal(BCH) {
		Stock = BCC
	}
	return CurrencyPair{Stock, pair.Money}
}

func (pair CurrencyPair) AdaptBccToBch() CurrencyPair {
	if pair.Stock.Equal(BCC) {
		return CurrencyPair{BCH, pair.Money}
	}
	return pair
}
