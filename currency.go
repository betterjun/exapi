package exapi

import "strings"

type CurrencyStatus struct {
	Deposit  bool // 是否可以充值
	Withdraw bool // 是否可以提币
}

func NewCurrency(name string) Currency {
	return Currency{Name: strings.ToUpper(name)}
}

type Currency struct {
	Name string `json:"name"`
}

func (c Currency) Symbol() string {
	return c.Name
}

func (c Currency) LowerSymbol() string {
	return strings.ToLower(c.Name)
}

func (c Currency) Equal(c2 Currency) bool {
	return c.Name == c2.Symbol()
}

func (c Currency) String() string {
	return c.Symbol()
}

var (
	UNKNOWN = Currency{"UNKNOWN"}
	CNY     = Currency{"CNY"}
	USD     = Currency{"USD"}
	USDT    = Currency{"USDT"}
	PAX     = Currency{"PAX"}
	USDC    = Currency{"USDC"}

	BCC = Currency{"BCC"}
	BCH = Currency{"BCH"}

	UNKNOWN_PAIR = CurrencyPair{UNKNOWN, UNKNOWN}
)

// BCC就是BCH的别名，很多交易所已经移除了BCC的叫法。
func (c Currency) AdaptBchToBcc() Currency {
	if c.Name == "BCH" || c.Name == "bch" {
		return BCC
	}
	return c
}

func (c Currency) AdaptBccToBch() Currency {
	if c.Name == "BCC" || c.Name == "bcc" {
		return BCH
	}
	return c
}
