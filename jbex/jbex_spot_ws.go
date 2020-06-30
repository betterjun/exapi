package jbex

import (
	"encoding/json"
	"fmt"
	. "github.com/betterjun/exapi"
	"strings"
	"time"
)

type JBEXWs struct {
	SpotWsBase
}

func NewSpotWebsocket(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://wsapi.jbex.com/openapi/quote/ws/v1"
		// /openapi/quote/ws/v1
	}

	ws := &JBEXWs{}
	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	//ws.SetConnectHandler(func(conn *Connection) (err error) {
	//	//Log("JBEXWs:SetConnectHandler")
	//	// 初始化函数
	//	conn.SendMessage(ws.Pack(map[string]interface{}{"id": time.Now().Unix(), "method": "SET_PROPERTY", "params": []interface{}{"combined", true}}))
	//	return nil
	//})

	ws.SetHeartBeatHandler(func(conn *Connection) (err error) {
		//Log("JBEXWs:HeartBeat")
		conn.SendMessage([]byte(fmt.Sprintf(`{"ping": %v}`, time.Now().Unix())))
		return nil
	})
	ws.HeartbeatIntervalTime = time.Second * 30
	ws.SpotWsBase.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
	if ws.SpotWsBase.Conn != nil && ws.OnConnect != nil {
		ws.OnConnect(ws.SpotWsBase.Conn)
	}
	ws.SpotWsBase.SpotWebsocket = ws

	go ws.SpotWsBase.Loop()
	return ws, err

}

func (ws *JBEXWs) GetExchangeName() string {
	return JBEX
}

// 格式化流名称
func (ws *JBEXWs) FormatTopicName(topic string, pair CurrencyPair) string {
	symbol := pair.ToSymbol("")
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("%s@ticker", symbol)
	case STREAM_DEPTH:
		return fmt.Sprintf("%s@depth", symbol)
	case STREAM_TRADE:
		return fmt.Sprintf("%s@trade", symbol)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *JBEXWs) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	symbol := pair.ToSymbol("")
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"symbol": symbol, "topic": "realtimes", "event": "sub", "params": map[string]interface{}{"binary": false}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"symbol": symbol, "topic": "depth", "event": "sub", "params": map[string]interface{}{"binary": false}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"symbol": symbol, "topic": "trade", "event": "sub", "params": map[string]interface{}{"binary": false}})
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *JBEXWs) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	symbol := pair.ToSymbol("")
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"symbol": symbol, "topic": "realtimes", "event": "cancel", "params": map[string]interface{}{"binary": false}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"symbol": symbol, "topic": "depth", "event": "cancel", "params": map[string]interface{}{"binary": false}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"symbol": symbol, "topic": "trade", "event": "cancel", "params": map[string]interface{}{"binary": false}})
	default:
		return nil
	}
}

type jbexResponse struct {
	Symbol   string                   `json:"symbol"`
	Topic    string                   `json:"topic"`
	Data     []map[string]interface{} `json:"data"`
	SendTime int64                    `json:"sendTime"`
}

func (ws *JBEXWs) OnMessage(data []byte) (err error) {
	if data == nil {
		Error("[ws][%s] websocket OnMessage failed:%v", ws.GetURL(), err)
		ws.SetDisconnected(true)
		return nil
	}

	Log("[ws][%s] websocket received:%v", ws.GetURL(), string(data))
	// 心跳响应
	if strings.Contains(string(data), "pong") {
		return nil
	}

	result := jbexResponse{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		Error("[ws][%s] websocket json.Unmarshal failed:%v", ws.GetURL(), err)
		return err
	}
	if len(result.Data) == 0 {
		return
	}

	switch result.Topic {
	case "realtimes":
		if ws.OnTicker != nil {
			pair := ws.GetPairByStream(ws.formatTopicName(STREAM_TICKER, result.Symbol))
			tick := ws.parseTickerData(result.Data[0], pair)
			ws.OnTicker(tick)
		}
	case "depth":
		if ws.OnDepth != nil {
			pair := ws.GetPairByStream(ws.formatTopicName(STREAM_DEPTH, result.Symbol))
			depth := ws.parseDepthData(result.Data[0], pair)
			ws.OnDepth(depth)
		}
	case "trade":
		if ws.OnTrade != nil {
			pair := ws.GetPairByStream(ws.formatTopicName(STREAM_TRADE, result.Symbol))
			ws.OnTrade(ws.parseTrades(result.Data, pair))
		}
	}

	return nil
}

// 格式化流名称，在解析数据时要用
func (ws *JBEXWs) formatTopicName(topic string, symbol string) string {
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("%s@ticker", symbol)
	case STREAM_DEPTH:
		return fmt.Sprintf("%s@depth", symbol)
	case STREAM_TRADE:
		return fmt.Sprintf("%s@trade", symbol)
	default:
		return ""
	}
}

func (ws *JBEXWs) parseTickerData(tickmap map[string]interface{}, pair CurrencyPair) *Ticker {
	t := new(Ticker)
	t.Market = pair
	t.Symbol = pair.ToLowerSymbol("/")
	t.Open = ToFloat64(tickmap["o"])
	t.Last = ToFloat64(tickmap["c"])
	t.High = ToFloat64(tickmap["h"])
	t.Low = ToFloat64(tickmap["l"])
	t.Vol = ToFloat64(tickmap["v"])
	t.TS = ToInt64(tickmap["t"])
	return t
}

func (ws *JBEXWs) parseDepthData(depthmap map[string]interface{}, pair CurrencyPair) *Depth {
	depth := new(Depth)
	depth.Market = pair
	depth.Symbol = pair.ToLowerSymbol("/")
	depth.TS = ToInt64(depthmap["t"])

	bids, _ := depthmap["b"].([]interface{})
	asks, _ := depthmap["a"].([]interface{})

	//AskList      DepthRecords `json:"asks"`          // 卖方订单列表，价格从低到高排序
	//BidList      DepthRecords `json:"bids"`          // 买方订单列表，价格从高到底排序
	depth.BidList = make(DepthRecords, 0, len(bids))
	for _, a := range bids {
		v := a.([]interface{})
		depth.BidList = append(depth.BidList, DepthRecord{ToFloat64(v[0]), ToFloat64(v[1])})
	}

	depth.AskList = make(DepthRecords, 0, len(asks))
	for _, a := range asks {
		v := a.([]interface{})
		depth.AskList = append(depth.AskList, DepthRecord{ToFloat64(v[0]), ToFloat64(v[1])})
	}
	return depth
}

func (ws *JBEXWs) parseTrades(records []map[string]interface{}, pair CurrencyPair) []Trade {
	side := BUY
	trades := make([]Trade, 0, len(records))
	for _, v := range records {
		m := ToBool(v["m"])
		if m {
			side = BUY
		} else {
			side = SELL
		}
		trades = append(trades, Trade{
			Tid:    ToInt64(v["v"]),
			Side:   side,
			Amount: ToFloat64(v["q"]),
			Price:  ToFloat64(v["p"]),
			TS:     ToInt64(v["t"]),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
	}
	return trades
}
