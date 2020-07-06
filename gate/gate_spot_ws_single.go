package gate

import (
	"fmt"
	. "github.com/betterjun/exapi"
	"sort"
	"time"
)

type depthCacheMap struct {
	asksMap map[float64]float64
	bidsMap map[float64]float64
}

type GateSpotWsSingle struct {
	SpotWsBase
	// 缓存的全量depth
	spotPairDepthMap map[string]*depthCacheMap
}

func NewGateSpotWsSingle(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://ws.gate.io/v3"
	}

	ws := &GateSpotWsSingle{}
	ws.spotPairDepthMap = make(map[string]*depthCacheMap)

	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.SetHeartBeatHandler(func(conn *Connection) (err error) {
		//Log("GateSpotWsSingle:HeartBeat")
		conn.SendMessage([]byte(fmt.Sprintf(`{"id":%v, "method":"server.ping", "params":[]}`, time.Now().Unix())))
		return nil
	})
	ws.HeartbeatIntervalTime = time.Second * 30
	ws.SpotWsBase.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
	ws.SpotWsBase.SpotWebsocket = ws

	go ws.SpotWsBase.Loop()
	return ws, err
}

func (ws *GateSpotWsSingle) GetExchangeName() string {
	return GATE
}

// 格式化流名称
func (ws *GateSpotWsSingle) FormatTopicName(topic string, pair CurrencyPair) string {
	return ws.formatTopicName(topic, pair.ToSymbol("_"))
}

// 格式化流名称
func (ws *GateSpotWsSingle) formatTopicName(topic, symbol string) string {
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("%s%s", symbol, topic)
	case STREAM_DEPTH:
		return fmt.Sprintf("%s%s", symbol, topic)
	case STREAM_TRADE:
		return fmt.Sprintf("%s%s", symbol, topic)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *GateSpotWsSingle) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	symbol := pair.ToSymbol("_")
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{
			"id":     time.Now().UnixNano(),
			"method": "ticker.subscribe",
			"params": []string{symbol}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{
			"id":     time.Now().UnixNano(),
			"method": "depth.subscribe",
			"params": []interface{}{symbol, 30, "0"}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{
			"id":     time.Now().UnixNano(),
			"method": "trades.subscribe",
			"params": []string{symbol}})
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *GateSpotWsSingle) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	return nil
}

func (ws *GateSpotWsSingle) OnMessage(data []byte) (err error) {
	if data == nil {
		Error("[ws][%s] websocket OnMessage failed:%v", ws.GetURL(), err)
		ws.SetDisconnected(true)
		return nil
	}

	msg := data
	ts := time.Now().UnixNano()

	Log("[ws][%s] websocket received:%v", ws.GetURL(), string(msg))

	resp := make(map[string]interface{})
	err = json.Unmarshal(msg, &resp)
	if err != nil {
		Error("[ws][%s] websocket json.Unmarshal failed:%v", ws.GetURL(), err)
		return err
	}

	method, ok := resp["method"].(string)
	if !ok {
		return
	}

	// 数据包
	switch method {
	case "ticker.update":
		ticker := ws.parseTicker(resp["params"].([]interface{}), ts)
		if ws.OnTicker != nil {
			ws.OnTicker(ticker)
		}
	case "depth.update":
		dep := ws.parseDepth(resp["params"].([]interface{}), ts)
		if ws.OnDepth != nil {
			ws.OnDepth(dep)
		}
	case "trades.update":
		trades := ws.parseTrade(resp["params"].([]interface{}), ts)
		if ws.OnTrade != nil {
			ws.OnTrade(trades)
		}
	default:
		return nil
	}

	return nil
}

func (ws *GateSpotWsSingle) parseTicker(resp []interface{}, ts int64) *Ticker {
	symbol := resp[0].(string)
	pair := ws.GetPairByStream(ws.formatTopicName(STREAM_TICKER, symbol))

	tickerMap, _ := resp[1].(map[string]interface{})
	return &Ticker{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Open:   ToFloat64(tickerMap["open"]),
		Last:   ToFloat64(tickerMap["last"]),
		High:   ToFloat64(tickerMap["high"]),
		Low:    ToFloat64(tickerMap["low"]),
		Vol:    ToFloat64(tickerMap["baseVolume"]),
		//Buy:    ToFloat64(obj["buy"]),
		//Sell:   ToFloat64(obj["sell"]),
		TS: ts / int64(time.Millisecond),
	}
}

func (ws *GateSpotWsSingle) parseDepth(resp []interface{}, ts int64) (dep *Depth) {
	fullUpdate := ToBool(resp[0])
	data := resp[1].(map[string]interface{})
	symbol := resp[2].(string)
	pair := ws.GetPairByStream(ws.formatTopicName(STREAM_DEPTH, symbol))

	dep = &Depth{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		TS:     ts / int64(time.Millisecond),
	}

	depMap, ok := ws.spotPairDepthMap[symbol]
	if !ok {
		depMap = &depthCacheMap{
			asksMap: make(map[float64]float64),
			bidsMap: make(map[float64]float64),
		}
		ws.spotPairDepthMap[symbol] = depMap
	}

	bids, _ := data["bids"].([]interface{})
	asks, _ := data["asks"].([]interface{})
	if fullUpdate {
		depMap.asksMap = make(map[float64]float64)
		depMap.bidsMap = make(map[float64]float64)

		for _, v := range bids {
			bid, _ := v.([]interface{})
			price := ToFloat64(bid[0])
			amount := ToFloat64(bid[1])
			dep.BidList = append(dep.BidList, DepthRecord{price, amount})
			depMap.bidsMap[price] = amount
		}

		for _, v := range asks {
			ask, _ := v.([]interface{})
			price := ToFloat64(ask[0])
			amount := ToFloat64(ask[1])
			dep.AskList = append(dep.AskList, DepthRecord{price, amount})
			depMap.asksMap[price] = amount
		}

		return dep
	}

	// 增量更新
	for _, v := range bids {
		bid, _ := v.([]interface{})
		price := ToFloat64(bid[0])
		amount := ToFloat64(bid[1])
		if amount == 0 {
			delete(depMap.bidsMap, price)
		} else {
			depMap.bidsMap[price] = amount
		}
	}
	dep.BidList = getDepthList(depMap.bidsMap, true)

	for _, v := range asks {
		ask, _ := v.([]interface{})
		price := ToFloat64(ask[0])
		amount := ToFloat64(ask[1])
		if amount == 0 {
			delete(depMap.asksMap, price)
		} else {
			depMap.asksMap[price] = amount
		}
	}
	dep.AskList = getDepthList(depMap.asksMap, false)

	return dep
}

func getDepthList(m map[float64]float64, reverse bool) (list DepthRecords) {
	keys := make([]float64, 0, len(m))
	for k, _ := range m {
		keys = append(keys, k)
	}

	sort.Float64s(keys)
	list = make(DepthRecords, 0, len(m))

	if reverse {
		for i := len(keys) - 1; i >= 0; i-- {
			list = append(list, DepthRecord{keys[i], m[keys[i]]})
		}
	} else {
		for _, k := range keys {
			list = append(list, DepthRecord{k, m[k]})
		}
	}

	return list
}

func (ws *GateSpotWsSingle) parseTrade(resp []interface{}, ts int64) (trades []Trade) {
	symbol := resp[0].(string)
	pair := ws.GetPairByStream(ws.formatTopicName(STREAM_TRADE, symbol))

	data := resp[1].([]interface{})
	trades = make([]Trade, 0, len(data))
	for _, v := range data {
		tradeMap, _ := v.(map[string]interface{})

		trades = append(trades, Trade{
			Tid:    ToInt64(tradeMap["id"]),
			Side:   AdaptTradeSide(ToString(tradeMap["type"])),
			Amount: ToFloat64(tradeMap["amount"]),
			Price:  ToFloat64(tradeMap["price"]),
			TS:     int64(ToFloat64(tradeMap["time"]) * 1000),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
	}

	return trades
}
