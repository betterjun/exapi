package exapi

/*
K线周期。
*/
type KlinePeriod int

const (
	KLINE_M1    KlinePeriod = iota // k线，周期1分钟
	KLINE_M5                       // k线，周期5分钟
	KLINE_M15                      // k线，周期15分钟
	KLINE_M30                      // k线，周期30分钟
	KLINE_H1                       // k线，周期1小时
	KLINE_H4                       // k线，周期4小时
	KLINE_DAY                      // k线，周期1天
	KLINE_WEEK                     // k线，周期1星期
	KLINE_MONTH                    // k线，周期1个月
)

/*
深度挡位。
有些挡位，在一些交易所不支持时，会置为Depth_Default，按交易所实际的挡位输出。
*/
const (
	Depth_Default = 0   // 默认深度
	Depth5        = 5   // 5档
	Depth10       = 10  // 10档
	Depth20       = 20  // 20档
	Depth50       = 50  // 50档
	Depth100      = 100 // 100档
)

/*
深度聚合。
挡位价格的聚合。
*/
const (
	Depth_Aggregate_Default = 0 // 默认深度
	Depth_Aggregate1        = 1 // 5档
	Depth_Aggregate2        = 2 // 10档
	Depth_Aggregate3        = 3 // 20档
	Depth_Aggregate4        = 4 // 50档
	Depth_Aggregate5        = 5 // 100档
)

type TradeSide int

const (
	BUY TradeSide = 1 + iota
	SELL
	BUY_MARKET
	SELL_MARKET
)

func (ts TradeSide) String() string {
	switch ts {
	case 1:
		return "BUY"
	case 2:
		return "SELL"
	case 3:
		return "BUY_MARKET"
	case 4:
		return "SELL_MARKET"
	default:
		return "UNKNOWN"
	}
}

type TradeStatus int

func (ts TradeStatus) String() string {
	return tradeStatusSymbol[ts]
}

var tradeStatusSymbol = [...]string{"UNFINISH", "PART_FINISH", "FINISH", "CANCEL", "REJECT", "CANCEL_ING"}

const (
	ORDER_UNFINISH TradeStatus = iota
	ORDER_PART_FINISH
	ORDER_FINISH
	ORDER_CANCEL
	ORDER_REJECT
	ORDER_CANCEL_ING
	ORDER_FAIL
)

const (
	OPEN_BUY   = 1 + iota //开多
	OPEN_SELL             //开空
	CLOSE_BUY             //平多
	CLOSE_SELL            //平空
)

//k线周期
const (
	KLINE_PERIOD_1MIN = 1 + iota
	KLINE_PERIOD_3MIN
	KLINE_PERIOD_5MIN
	KLINE_PERIOD_15MIN
	KLINE_PERIOD_30MIN
	KLINE_PERIOD_60MIN
	KLINE_PERIOD_1H
	KLINE_PERIOD_2H
	KLINE_PERIOD_4H
	KLINE_PERIOD_6H
	KLINE_PERIOD_8H
	KLINE_PERIOD_12H
	KLINE_PERIOD_1DAY
	KLINE_PERIOD_3DAY
	KLINE_PERIOD_1WEEK
	KLINE_PERIOD_1MONTH
	KLINE_PERIOD_1YEAR
)

var (
	THIS_WEEK_CONTRACT = "this_week" //周合约
	NEXT_WEEK_CONTRACT = "next_week" //次周合约
	QUARTER_CONTRACT   = "quarter"   //季度合约
)

//exchanges const
const (
	HUOBI       = "huobi"
	BINANCE     = "binance"
	OKEX        = "okex"
	ZB          = "zb"
	ET          = "et"
	COINEX      = "coinex"
	OKCOIN_CN   = "okcoin"
	OKCOIN_COM  = "okcoin"
	OKEX_FUTURE = "okex"
	OKEX_SWAP   = "okex"
	BITSTAMP    = "bitstamp"
	KRAKEN      = "kraken"
	BITFINEX    = "bitfinex"
	POLONIEX    = "poloniex"
	BITHUMB     = "bithumb"
	GATE        = "gate"
	BITTREX     = "bittrex"
	GDAX        = "gdax"
	WEX_NZ      = "wex"
	BIGONE      = "bigone"
	COIN58      = "58coin"
	FCOIN       = "fcoin"
	HITBTC      = "hitbtc"
	BITMEX      = "bitmex"
	CRYPTOPIA   = "cryptopia"
	HBDM        = "hbdm"
)
