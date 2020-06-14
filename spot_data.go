package exapi

type Ticker struct {
	//Symbol string       `json:"symbol"`      // 交易对
	Market CurrencyPair `json:"market"`      // 交易对
	Symbol string       `json:"symbol"`      // 交易对
	Open   float64      `json:"open,string"` // 开盘价
	Last   float64      `json:"last,string"` // 收盘价
	High   float64      `json:"high,string"` // 最高价
	Low    float64      `json:"low,string"`  // 最低价
	Vol    float64      `json:"vol,string"`  // 基础币种成交量
	Buy    float64      `json:"buy,string"`  // 最优买价
	Sell   float64      `json:"sell,string"` // 最优卖家
	TS     int64        `json:"ts"`          // 最新时间，单位为毫秒(millisecond)
}

type DepthRecord struct {
	Price  float64 `json:"price,string"`  // 报价
	Amount float64 `json:"amount,string"` // 数量
}

type DepthRecords []DepthRecord

func (dr DepthRecords) Len() int {
	return len(dr)
}

func (dr DepthRecords) Swap(i, j int) {
	dr[i], dr[j] = dr[j], dr[i]
}

func (dr DepthRecords) Less(i, j int) bool {
	return dr[i].Price < dr[j].Price
}

type Depth struct {
	ContractType string       `json:"contract_type"` // for future
	Market       CurrencyPair `json:"market"`        // 交易对
	Symbol       string       `json:"symbol"`        // 交易对
	TS           int64        `json:"ts"`            // 时间，单位为毫秒(millisecond)
	AskList      DepthRecords `json:"asks"`          // 卖方订单列表，价格从低到高排序
	BidList      DepthRecords `json:"bids"`          // 买方订单列表，价格从高到底排序
}

type Trade struct {
	Tid    int64        `json:"tid"`           // 交易id
	Side   TradeSide    `json:"type"`          // 交易方向
	Amount float64      `json:"amount,string"` // 成交量
	Price  float64      `json:"price,string"`  // 成交价
	TS     int64        `json:"ts"`            // 时间，单位为毫秒(millisecond)
	Market CurrencyPair `json:"market"`        // 交易对
	Symbol string       `json:"symbol"`        // 交易对
}

type Kline struct {
	Market CurrencyPair `json:"market"`       // 交易对
	Symbol string       `json:"symbol"`       // 交易对
	TS     int64        `json:"ts"`           // 开盘时间，单位为秒(second)
	Open   float64      `json:"open,string"`  // 开盘价
	Close  float64      `json:"close,string"` // 收盘价
	High   float64      `json:"high,string"`  // 最高价
	Low    float64      `json:"low,string"`   // 最低价
	Vol    float64      `json:"vol,string"`   // 报价币种成交量
}

type Order struct {
	OrderID    string       `json:"order_id"`           // 订单id
	Price      float64      `json:"price,string"`       // 委托价格
	Amount     float64      `json:"amount,string"`      // 委托量
	AvgPrice   float64      `json:"avg_price,string"`   // 平均成交价
	DealAmount float64      `json:"deal_amount,string"` // 成交量
	Fee        float64      `json:"fee,string"`         // 手续费
	TS         int64        `json:"ts"`                 // 时间，单位为毫秒(millisecond)
	Status     TradeStatus  `json:"status"`             // 订单状态
	Market     CurrencyPair `json:"market"`             // 交易对
	Symbol     string       `json:"symbol"`             // 交易对
	Side       TradeSide    `json:"side"`               // 交易方向
}

type OrderDeal struct {
	OrderID          string       `json:"order_id"`                  // 订单id
	DealID           string       `json:"deal_id"`                   // 本次成交id
	TS               int64        `json:"ts"`                        // 时间，单位为毫秒(millisecond)
	Price            float64      `json:"price,string"`              // 委托价格
	FilledAmount     float64      `json:"filled_amount,string"`      // 本次成交量
	FilledCashAmount float64      `json:"filled_cash_amount,string"` // 本次成交金额
	UnFilledAmount   float64      `json:"unfilled_amount,string"`    // 未成交的量
	Side             TradeSide    `json:"side"`                      // 交易方向
	Market           CurrencyPair `json:"market"`                    // 交易对
	Symbol           string       `json:"symbol"`                    // 交易对
}

type SubAccount struct {
	Currency     Currency // 币种
	Amount       float64  `json:"amount,string"`        // 可用余额
	FrozenAmount float64  `json:"frozen_amount,string"` // 冻结余额
	LoanAmount   float64  `json:"loan_amount,string"`   // 借贷余额
}

type Account struct {
	Exchange    string                  `json:"exchange,string"` // 交易所名字
	Asset       float64                 `json:"asset,string"`    //总资产
	NetAsset    float64                 `json:"netasset,string"` //净资产
	SubAccounts map[Currency]SubAccount `json:"sub_accounts"`    // 每个币种账本
}

// k线取字段集合api
func Open(klines []Kline) (data []float64) {
	data = make([]float64, 0)
	for _, v := range klines {
		data = append(data, v.Open)
	}
	return data
}

func Close(klines []Kline) (data []float64) {
	data = make([]float64, 0)
	for _, v := range klines {
		data = append(data, v.Close)
	}
	return data
}

func High(klines []Kline) (data []float64) {
	data = make([]float64, 0)
	for _, v := range klines {
		data = append(data, v.High)
	}
	return data
}

func Low(klines []Kline) (data []float64) {
	data = make([]float64, 0)
	for _, v := range klines {
		data = append(data, v.Low)
	}
	return data
}
