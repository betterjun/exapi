package binance

import (
	"fmt"
	. "github.com/betterjun/exapi"
	"strings"
	"time"
)

type BinanceWs struct {
	SpotWsBase
}

func NewSpotWebsocket(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://stream.binance.com:9443/ws"
	}

	ws := &BinanceWs{}
	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.SetConnectHandler(func(conn *Connection) (err error) {
		//Log("BinanceWs:SetConnectHandler")
		// 初始化函数
		conn.SendMessage(ws.Pack(map[string]interface{}{"id": time.Now().Unix(), "method": "SET_PROPERTY", "params": []interface{}{"combined", true}}))
		return nil
	})
	ws.SetHeartBeatHandler(func(conn *Connection) (err error) {
		//Log("BinanceWs:HeartBeat")
		conn.SendMessage([]byte(fmt.Sprintf(`{"method": "LIST_SUBSCRIPTIONS","id":%v}`, time.Now().Unix())))
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

func (ws *BinanceWs) GetExchangeName() string {
	return BINANCE
}

// 格式化流名称
func (ws *BinanceWs) FormatTopicName(topic string, pair CurrencyPair) string {
	symbol := pair.ToLowerSymbol("")
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("%s@ticker", symbol)
	case STREAM_DEPTH:
		return fmt.Sprintf("%s@depth20", symbol)
	case STREAM_TRADE:
		return fmt.Sprintf("%s@trade", symbol)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *BinanceWs) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	stream := ws.FormatTopicName(topic, pair)
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"id": time.Now().Unix(), "method": "SUBSCRIBE", "params": []interface{}{stream}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"id": time.Now().Unix(), "method": "SUBSCRIBE", "params": []interface{}{stream}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"id": time.Now().Unix(), "method": "SUBSCRIBE", "params": []interface{}{stream}})
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *BinanceWs) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	stream := ws.FormatTopicName(topic, pair)
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"id": time.Now().Unix(), "method": "UNSUBSCRIBE", "params": []interface{}{stream}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"id": time.Now().Unix(), "method": "UNSUBSCRIBE", "params": []interface{}{stream}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"id": time.Now().Unix(), "method": "UNSUBSCRIBE", "params": []interface{}{stream}})
	default:
		return nil
	}
}

func (ws *BinanceWs) OnMessage(data []byte) (err error) {
	if data == nil {
		Error("[ws][%s] websocket OnMessage failed:%v", ws.GetURL(), err)
		ws.SetDisconnected(true)
		return nil
	}

	Log("[ws][%s] websocket received:%v", ws.GetURL(), string(data))
	//心跳
	if strings.Contains(string(data), "ping") {
		var ping struct {
			Ping int64
		}
		json.Unmarshal(data, &ping)

		pong := struct {
			Pong int64 `json:"pong"`
		}{ping.Ping}

		ws.Conn.SendMessage(ws.Pack(pong))
		return nil
	}

	result := make(map[string]interface{})
	err = json.Unmarshal(data, &result)
	if err != nil {
		Error("[ws][%s] websocket json.Unmarshal failed:%v", ws.GetURL(), err)
		return err
	}

	stream, ok := result["stream"].(string)
	if !ok {
		return nil
	}

	datamap, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil
	}

	pair := ws.GetPairByStream(stream)
	if strings.Contains(stream, "@ticker") {
		tick := ws.parseTickerData(datamap)
		tick.Market = pair
		tick.Symbol = pair.ToLowerSymbol("/")
		ws.OnTicker(tick)
		return nil
	} else if strings.Contains(stream, "@trade") {
		side := BUY
		if datamap["m"].(bool) == false {
			side = SELL
		}
		trades := make([]Trade, 0)
		trades = append(trades, Trade{
			Tid:    ToInt64(datamap["t"]),
			Side:   side,
			Amount: ToFloat64(datamap["q"]),
			Price:  ToFloat64(datamap["p"]),
			TS:     ToInt64(datamap["T"]),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
		ws.OnTrade(trades)
		return nil
	} else if strings.Contains(stream, "@depth") {
		bids := datamap["bids"].([]interface{})
		asks := datamap["asks"].([]interface{})
		depth := ws.parseDepthData(bids, asks)
		depth.Market = pair
		depth.Symbol = pair.ToLowerSymbol("/")
		ws.OnDepth(depth)
		return nil
	}

	return nil
}

func (ws *BinanceWs) parseTickerData(tickmap map[string]interface{}) *Ticker {
	t := new(Ticker)
	t.Open = ToFloat64(tickmap["o"])
	t.Last = ToFloat64(tickmap["c"])
	t.High = ToFloat64(tickmap["h"])
	t.Low = ToFloat64(tickmap["l"])
	t.Vol = ToFloat64(tickmap["v"])
	t.Buy = ToFloat64(tickmap["b"])
	t.Sell = ToFloat64(tickmap["a"])
	t.TS = ToInt64(tickmap["E"])
	return t
}

func (ws *BinanceWs) parseDepthData(bids, asks []interface{}) *Depth {
	depth := new(Depth)
	depth.TS = int64(time.Now().UnixNano() / int64(time.Millisecond))
	for _, a := range bids {
		v := a.([]interface{})
		depth.BidList = append(depth.BidList, DepthRecord{ToFloat64(v[0]), ToFloat64(v[1])})
	}

	for _, a := range asks {
		v := a.([]interface{})
		depth.AskList = append(depth.AskList, DepthRecord{ToFloat64(v[0]), ToFloat64(v[1])})
	}
	return depth
}

func (ws *BinanceWs) parseKlineData(k map[string]interface{}) *Kline {
	kline := &Kline{
		TS:    ToInt64(k["t"]) / 1000,
		Open:  ToFloat64(k["o"]),
		Close: ToFloat64(k["c"]),
		High:  ToFloat64(k["h"]),
		Low:   ToFloat64(k["l"]),
		//Vol:   ToFloat64(k["v"]),// 这根K线期间成交量
		Vol: ToFloat64(k["q"]), // 这根K线期间成交额
	}
	return kline
}
