package exapi

// spot api interface
type SpotAPI interface {
	// 获取交易所名称
	GetExchangeName() string
	// 设置交易所地址，仅在初始化时设置，不支持后续动态更改
	SetURL(exurl string)
	// 获取交易所地址
	GetURL() string

	// 获取支持的交易对
	GetAllCurrencyPair() (map[string]SymbolSetting, error)
	// 获取此币种是否可以充提币
	GetCurrencyStatus(currency Currency) (CurrencyStatus, error)
	// 获取所有币种是否可以充提币
	GetAllCurrencyStatus() (map[string]CurrencyStatus, error)

	// 公告行情
	// 获取单一币种对的行情
	GetTicker(pair CurrencyPair) (*Ticker, error)
	// 获取所有币种对的行情
	GetAllTicker() ([]Ticker, error)
	//获取单一币种对的深度
	GetDepth(pair CurrencyPair, size int, step int) (*Depth, error)
	// 获取单一币种对的成交记录
	GetTrades(pair CurrencyPair, size int) ([]Trade, error)
	// 获取单一币种对的K线记录，按时间升序排列
	GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error)

	// 交易相关
	// 限价单
	LimitBuy(pair CurrencyPair, price, amount string) (*Order, error)
	LimitSell(pair CurrencyPair, price, amount string) (*Order, error)
	// 市价单，下单后返回的数据不准，需要调用GetOrder获取准确信息
	// 买入金额，单位为计价货币
	MarketBuy(pair CurrencyPair, amount string) (*Order, error)
	// 卖出数量，单位为基础货币
	MarketSell(pair CurrencyPair, amount string) (*Order, error)
	// 撤单
	Cancel(orderId string, pair CurrencyPair) (bool, error)
	// 获取订单详情
	GetOrder(orderId string, pair CurrencyPair) (*Order, error)
	// 获取当前未完成订单列表
	GetPendingOrders(pair CurrencyPair) ([]Order, error)
	// 获取最近完成订单
	GetFinishedOrders(pair CurrencyPair) ([]Order, error)
	// 获取订单的成交明细
	GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error)
	// 获取单一币种对的最近成交明细
	GetUserTrades(pair CurrencyPair) ([]Trade, error)
	// 获取账户余额
	GetAccount() (*Account, error)
}
