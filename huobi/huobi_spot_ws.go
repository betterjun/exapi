package huobi

import (
	"bytes"
	"compress/gzip"
	"fmt"
	. "github.com/betterjun/exapi"
	"io/ioutil"
	"strings"
)

/*
订阅命令的响应
{
 "id": "id1",
 "status": "ok",
 "subbed": "market.btcusdt.trade.detail",
 "ts": 1489474081631
}

数据推送有下面三个字段
"ch": "market.btcusdt.trade.detail",
"ts": 1489474082831,
"tick"
*/
type wsSpotResponse struct {
	Ch   string                 `json:"ch"`
	Ts   int64                  `json:"ts"`
	Tick map[string]interface{} `json:"tick"`
}

type wsSpotTradeResponse struct {
	Id   int64
	Ts   int64
	Data []struct {
		TradeId   int64 //tradeId
		Amount    float64
		Price     float64
		Direction string
		Ts        int64
	}
}

/*
	id	integer	unix时间，同时作为消息ID
	ts	integer	unix系统时间
	amount	float	24小时成交量
	count	integer	24小时成交笔数
	open	float	24小时开盘价
	close	float	最新价
	low	float	24小时最低价
	high	float	24小时最高价
	vol	float	24小时成交额
*/
type wsSpotTickerResponse struct {
	Id     int64   `json:"id"`
	Ts     int64   `json:"ts"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Amount float64 `json:"amount"`
	Vol    float64 `json:"vol"`
	Count  int64   `json:"count"`
}

/*
"id": 1489464480,
"amount": 0.0,
"count": 0,
"open": 7962.62,
"close": 7962.62,
"low": 7962.62,
"high": 7962.62,
"vol": 0.0
*/
type wsSpotKlineResponse struct {
	Id     int64   `json:"id"`
	Amount float64 `json:"amount"`
	Count  int64   `json:"count"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	Low    float64 `json:"low"`
	High   float64 `json:"high"`
	Vol    float64 `json:"vol"`
}

type wsSpotDepthResponse struct {
	Bids [][]float64
	Asks [][]float64
}

type HuobiSpotWs struct {
	SpotWsBase
}

func NewSpotWebsocket(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://api.huobi.pro/ws"
	}

	ws := &HuobiSpotWs{}
	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.SpotWsBase.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
	ws.SpotWsBase.SpotWebsocket = ws
	//ws.SetHeartBeatHandler(func  (*Connection) (err error) {
	//	fmt.Println("HuobiSpotWs:HeartBeat")
	//	return nil
	//})

	go ws.SpotWsBase.Loop()
	return ws, err
}

func (ws *HuobiSpotWs) GetExchangeName() string {
	return HUOBI
}

// 格式化流名称
func (ws *HuobiSpotWs) FormatTopicName(topic string, pair CurrencyPair) string {
	symbol := pair.ToLowerSymbol("")
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("market.%s.detail", symbol)
	case STREAM_DEPTH:
		//return fmt.Sprintf("market.%s.depth.step0", symbol)
		return fmt.Sprintf("market.%s.mbp.refresh.%v", symbol, 20)
	case STREAM_TRADE:
		return fmt.Sprintf("market.%s.trade.detail", symbol)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *HuobiSpotWs) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	symbol := pair.ToLowerSymbol("")
	stream := ws.FormatTopicName(topic, pair)
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"id": "id_ticker_" + symbol, "sub": stream})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"id": "id_depth_" + symbol, "sub": stream})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"id": "id_trade_" + symbol, "sub": stream})
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *HuobiSpotWs) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	symbol := pair.ToLowerSymbol("")
	stream := ws.FormatTopicName(topic, pair)
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"id": "id_ticker_" + symbol, "unsub": stream})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"id": "id_depth_" + symbol, "unsub": stream})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"id": "id_trade_" + symbol, "unsub": stream})
	default:
		return nil
	}
}

// 消息解析函数
func (ws *HuobiSpotWs) OnMessage(data []byte) (err error) {
	if data == nil {
		Error("[ws][%s] websocket OnMessage failed:%v", ws.GetURL(), err)
		ws.SetDisconnected(true)
		return nil
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		Error("[ws][%s] websocket gzip.NewReader failed:%v", ws.GetURL(), err)
		return err
	}

	msg, err := ioutil.ReadAll(r)
	if err != nil {
		Error("[ws][%s] websocket gzip.ReadAll failed:%v", ws.GetURL(), err)
		return err
	}

	Log("[ws][%s] websocket received:%v", ws.GetURL(), string(msg))

	//心跳
	if strings.Contains(string(msg), "ping") {
		var ping struct {
			Ping int64
		}
		json.Unmarshal(msg, &ping)

		pong := struct {
			Pong int64 `json:"pong"`
		}{ping.Ping}

		ws.Conn.SendMessage(ws.Pack(pong))
		return nil
	}

	var resp wsSpotResponse
	err = json.Unmarshal(msg, &resp)
	if err != nil {
		Error("[ws][%s] websocket json.Unmarshal failed:%v", ws.GetURL(), err)
		return err
	}

	// 响应包
	if resp.Ch == "" {
		return nil
	}

	// 以下是数据包
	fields := strings.Split(resp.Ch, ".")
	if len(fields) < 3 {
		Log("[ws][%s] websocket 未知的topic数据:%v", ws.GetURL(), resp.Ch)
		return nil
	}

	pair := ws.GetPairByStream(resp.Ch)
	switch fields[2] {
	case "detail":
		ticker := ws.parseTicker(resp.Tick, pair)
		ticker.TS = resp.Ts
		if ws.OnTicker != nil {
			ws.OnTicker(ticker)
		}
	case "mbp":
		depth := ws.parseDepth(resp.Tick, pair)
		depth.TS = resp.Ts
		if ws.OnDepth != nil {
			ws.OnDepth(depth)
		}
	case "trade":
		trade := ws.parseTrade(resp.Tick, pair)
		if ws.OnTrade != nil {
			ws.OnTrade(trade)
		}

	default:
		Log("[ws][%s] websocket 未知的topic数据:%v", ws.GetURL(), resp.Ch)
		return nil
	}

	return nil
}

func (ws *HuobiSpotWs) parseTicker(datamap map[string]interface{}, pair CurrencyPair) (ticker *Ticker) {
	return &Ticker{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Open:   ToFloat64(datamap["open"]),
		Last:   ToFloat64(datamap["close"]),
		High:   ToFloat64(datamap["high"]),
		Low:    ToFloat64(datamap["low"]),
		Vol:    ToFloat64(datamap["vol"]),
		//Buy   :, // 火币没有最优买卖价
		//Sell  :,
		TS: ToInt64(datamap["ts"]),
	}
}

func (ws *HuobiSpotWs) parseDepth(datamap map[string]interface{}, pair CurrencyPair) (dep *Depth) {
	dep = &Depth{}
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")

	bids, _ := datamap["bids"].([]interface{})
	for _, bid := range bids {
		v, _ := bid.([]interface{})
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(v[0]), ToFloat64(v[1])})
	}

	asks, _ := datamap["asks"].([]interface{})
	for _, ask := range asks {
		v, _ := ask.([]interface{})
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(v[0]), ToFloat64(v[1])})
	}

	return dep
}

func (ws *HuobiSpotWs) parseTrade(datamap map[string]interface{}, pair CurrencyPair) (trades []Trade) {
	tradesArr, _ := datamap["data"].([]interface{})
	trades = make([]Trade, 0, len(tradesArr))
	for _, v := range tradesArr {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		trades = append(trades, Trade{
			Tid:    ToInt64(obj["tradeId"]),
			Price:  ToFloat64(obj["price"]),
			Amount: ToFloat64(obj["amount"]),
			Side:   AdaptTradeSide(ToString(obj["direction"])),
			TS:     ToInt64(obj["ts"]),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
	}
	return trades
}
