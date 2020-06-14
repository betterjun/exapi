package exapi

import (
	"net/http"
)

type APIConfig struct {
	HttpClient    *http.Client
	Endpoint      string
	ApiKey        string
	ApiSecretKey  string
	ApiPassphrase string //for okex.com v3 api
	ClientId      string //for bitstamp.net , huobi.pro

	Lever int //杠杆倍数 , for future
}

type FutureTicker struct {
	*Ticker
	ContractType string  `json:"omitempty"`
	ContractId   int     `json:"contractId"`
	LimitHigh    float64 `json:"limitHigh,string"`
	LimitLow     float64 `json:"limitLow,string"`
	HoldAmount   float64 `json:"hold_amount,string"`
	UnitAmount   float64 `json:"unitAmount,string"`
}

type FutureKline struct {
	*Kline
	Vol2 float64 //个数
}

type FutureSubAccount struct {
	Currency      Currency
	AccountRights float64 //账户权益
	KeepDeposit   float64 //保证金
	ProfitReal    float64 //已实现盈亏
	ProfitUnreal  float64
	RiskRate      float64 //保证金率
}

type FutureAccount struct {
	FutureSubAccounts map[Currency]FutureSubAccount
}

type FutureOrder struct {
	OrderID2     string //请尽量用这个字段替代OrderID字段
	Price        float64
	Amount       float64
	AvgPrice     float64
	DealAmount   float64
	OrderID      int64
	OrderTime    int64
	Status       TradeStatus
	Currency     CurrencyPair
	OType        int     //1：开多 2：开空 3：平多 4： 平空
	LeverRate    int     //倍数
	Fee          float64 //手续费
	ContractName string
}

type FuturePosition struct {
	BuyAmount      float64
	BuyAvailable   float64
	BuyPriceAvg    float64
	BuyPriceCost   float64
	BuyProfitReal  float64
	CreateDate     int64
	LeverRate      int
	SellAmount     float64
	SellAvailable  float64
	SellPriceAvg   float64
	SellPriceCost  float64
	SellProfitReal float64
	Symbol         CurrencyPair //btc_usd:比特币,ltc_usd:莱特币
	ContractType   string
	ContractId     int64
	ForceLiquPrice float64 //预估爆仓价
}
