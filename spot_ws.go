package exapi

// 流常量
type StreamType string

const (
	STREAM_TICKER = "__TICKER"
	STREAM_DEPTH  = "__DEPTH"
	STREAM_TRADE  = "__TRADE"
)

type SpotWebsocket interface {
	// 设置交易所地址，仅在初始化时设置，不支持后续动态更改
	SetURL(exURL string)
	// 获取交易所地址
	GetURL() string
	// 设置代理地址
	SetProxyURL(proxyURL string)
	// 获取代理地址
	GetProxyURL() string

	// 订阅或取消行情，cb传nil为取消，目前cb只有最后一次设置的生效
	SubTicker(pair CurrencyPair, cb func(*Ticker) error) (err error)
	SubDepth(pair CurrencyPair, cb func(*Depth) error) (err error)
	SubTrade(pair CurrencyPair, cb func([]Trade) error) (err error)
	//SubKline(pair CurrencyPair, period KlinePeriod, cb func([]Kline) error) (err error)
	// 全部重新订阅
	Resubscribe() (err error)
	// 全部取消订阅
	Unsubscribe() (err error)
	// 设置初始化函数，当连接建立时调用
	SetConnectHandler(h func(*Connection) error)
	// 设置心跳函数
	SetHeartBeatHandler(h func(*Connection) error)
	// 关闭连接
	Close()

	// 以下为需要单独定制的接口
	// 获取交易所名称
	GetExchangeName() string
	// 格式化主题名称
	FormatTopicName(topic string, pair CurrencyPair) string
	// 格式化主题订阅消息
	FormatTopicSubData(topic string, pair CurrencyPair) []byte
	// 格式化主题取消订阅消息
	FormatTopicUnsubData(topic string, pair CurrencyPair) []byte
	// 消息解析函数
	OnMessage([]byte) error
}
