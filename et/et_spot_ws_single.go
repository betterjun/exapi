package et

import (
	"encoding/json"
	"fmt"
	. "github.com/betterjun/exapi"
	"sort"
	"strings"
	"time"
)

type depthCacheMap struct {
	asksMap map[float64]float64
	bidsMap map[float64]float64
}

type EtSpotWsSingle struct {
	SpotWsBase
	// 缓存的全量depth
	spotPairDepthMap map[string]*depthCacheMap
}

func NewEtSpotWsSingle(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "ws://47.108.94.209/ws/"
	}

	ws := &EtSpotWsSingle{}
	ws.spotPairDepthMap = make(map[string]*depthCacheMap)

	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.SetHeartBeatHandler(func(conn *Connection) (err error) {
		//Log("EtSpotWsSingle:HeartBeat")
		conn.SendMessage([]byte(fmt.Sprintf(`{"method":"server.ping","params":[],"id":%v}`, time.Now().Unix())))
		return nil
	})
	ws.HeartbeatIntervalTime = time.Second * 30
	ws.SpotWsBase.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
	ws.SpotWsBase.SpotWebsocket = ws

	go ws.SpotWsBase.Loop()
	return ws, err
}

func (ws *EtSpotWsSingle) GetExchangeName() string {
	return ET
}

// 格式化流名称
func (ws *EtSpotWsSingle) FormatTopicName(topic string, pair CurrencyPair) string {
	symbol := pair.ToLowerSymbol("")
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("%v_ticker", symbol)
	case STREAM_DEPTH:
		return fmt.Sprintf("%v_depth", symbol)
	case STREAM_TRADE:
		return fmt.Sprintf("spot/trade:%v", symbol)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *EtSpotWsSingle) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{
			"method": "today.subscribe",
			"params": []string{pair.ToSymbol("/")},
			"id":     time.Now().Unix()})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{
			"method": "depth.subscribe",
			"params": []interface{}{pair.ToSymbol("/"), 30, "0"},
			"id":     time.Now().Unix()})
	case STREAM_TRADE:
		return nil
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *EtSpotWsSingle) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{
			"method": "today.unsubscribe",
			"params": []string{},
			"id":     time.Now().Unix()})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{
			"method": "depth.unsubscribe",
			"params": []string{},
			"id":     time.Now().Unix()})
	case STREAM_TRADE:
		return nil
	default:
		return nil
	}
}

type ErrorStruct struct {
	Code    int    `json:"code"`
	Message string `json:"message"` // 响应消息有此字段
}

// {"method": "today.update", "params": ["BTC/USDT", {"low": "1.2012564", "open": "1.370376", "last": "1.4508153", "high": "6000", "volume": "2851.10447902", "deal": "5651.1018913680194814"}], "id": null}
type wsSpotResponse struct {
	Method string          `json:"method"` // 方法
	Params json.RawMessage `json:"params"` // update 消息有次字段
	Error  *ErrorStruct    `json:"error"`  // 响应消息有此字段
}

func (ws *EtSpotWsSingle) OnMessage(data []byte) (err error) {
	if data == nil {
		Error("[ws][%s] websocket OnMessage failed:%v", ws.GetURL(), err)
		ws.SetDisconnected(true)
		return nil
	}

	Log("[ws][%s] websocket received:%v", ws.GetURL(), string(data))

	var resp wsSpotResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		Error("[ws][%s] websocket json.Unmarshal failed:%v", ws.GetURL(), err)
		return err
	}

	// 数据包
	switch resp.Method {
	case "today.update":
		ticker := ws.parseTicker(resp.Params)
		if ws.OnTicker != nil {
			ws.OnTicker(ticker)
		}
	case "depth.update":
		dep := ws.parseDepth(resp.Params)
		if ws.OnDepth != nil {
			ws.OnDepth(dep)
		}
	//case "trades":
	//	trades := ws.parseTrade(resp, pair)
	//	if ws.OnTrade != nil {
	//		ws.OnTrade(trades)
	//	}
	default:
		return nil
	}

	return nil
}

func (ws *EtSpotWsSingle) parseTicker(params json.RawMessage) *Ticker {
	/*
		["BTC/USDT", {"low": "1.2012564", "open": "1.370376", "last": "1.4508153", "high": "6000", "volume": "2851.10447902", "deal": "5651.1018913680194814"}]
	*/

	resp := make([]interface{}, 0)
	err := json.Unmarshal(params, &resp)
	if err != nil {
		return nil
	}
	if len(resp) != 2 {
		return nil
	}

	tickerMap, ok := resp[1].(map[string]interface{})
	if !ok {
		return nil
	}

	ticker := &Ticker{
		Symbol: strings.ToLower(resp[0].(string)),
		Open:   ToFloat64(tickerMap["open"]),
		Last:   ToFloat64(tickerMap["last"]),
		High:   ToFloat64(tickerMap["high"]),
		Low:    ToFloat64(tickerMap["low"]),
		Vol:    ToFloat64(tickerMap["volume"]),
		TS:     time.Now().UnixNano() / 1000000,
	}
	ticker.Market = NewCurrencyPairFromString(ticker.Symbol)
	return ticker
}

func (ws *EtSpotWsSingle) parseDepth(params json.RawMessage) (dep *Depth) {
	/*
		[true, {"asks": [["6000", "0.8"], ["6728.73411484", "0.8"], ["6729.15266678", "0.1"], ["6729.29131043", "0.2"], ["6742.86835564", "0.6"], ["6744.65965459", "1"], ["6747.12015259", "0.9"], ["6747.24260373", "1"], ["6750", "1"], ["6750.60089853", "0.3"], ["6754.2141362", "0.6"], ["6761.4133801", "0.8"], ["6763.03921398", "1"], ["6763.51525677", "0.3"], ["6763.6981488", "0.5"], ["6767.77242811", "1"], ["6769.78400857", "0.6"], ["6772.6000665", "1"], ["6777.63268271", "0.2"], ["6784.78420221", "0.4"], ["6788.34525729", "0.5"], ["6789.40397918", "0.7"], ["6789.46591191", "0.2"], ["6789.54741867", "0.9"], ["6791.37906366", "1"], ["6795.90389949", "0.8"], ["6914.78976997", "2.18597848"], ["6926.13971015", "0.50939117"], ["6928.4263771", "2.18073747"], ["6942.54251264", "0.1"]], "bids": [["1.2012564", "1.62995585"], ["1.201241", "1.1"], ["1.2011689", "2.13849904"], ["1.2009897", "1.13394833"], ["1.2007686", "2"], ["1.2005962", "1.79975235"], ["1.200555", "2.2"], ["1.200459", "2.2"], ["1.200286", "1.8"], ["1.200228", "1.68019653"], ["1.200093", "1.5"], ["1", "12.7"]]}, "BTC/USDT"]
	*/
	resp := make([]interface{}, 0)
	err := json.Unmarshal(params, &resp)
	if err != nil {
		return nil
	}
	if len(resp) != 3 {
		return nil
	}

	fullUpdate, ok := resp[0].(bool)
	if !ok {
		return nil
	}
	symbol, ok := resp[2].(string)
	if !ok {
		return nil
	}

	data, ok := resp[1].(map[string]interface{})
	if !ok {
		return nil
	}

	dep = &Depth{
		Market: NewCurrencyPairFromString(symbol),
		Symbol: strings.ToLower(symbol),
		TS:     time.Now().UnixNano() / int64(time.Millisecond),
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

func (ws *EtSpotWsSingle) parseTrade(params json.RawMessage) (trades []Trade) {
	/*
		{
		    "data": [{
		            "date": 1582820402,
		            "amount": "0.0552",
		            "price": "8917.1",
		            "trade_type": "bid",
		            "type": "buy",
		            "tid": 785221584
		        }
		    ],
		    "dataType": "trades",
		    "channel": "btcusdt_trades"
		}
	*/
	//data, _ := resp["data"].([]interface{})
	//trades = make([]Trade, 0)
	//for _, v := range data {
	//	tradeMap, _ := v.(map[string]interface{})
	//
	//	trades = append(trades, Trade{
	//		Tid:    ToInt64(tradeMap["tid"]),
	//		Side:   AdaptTradeSide(ToString(tradeMap["type"])),
	//		Amount: ToFloat64(tradeMap["amount"]),
	//		Price:  ToFloat64(tradeMap["price"]),
	//		TS:     ToInt64(tradeMap["date"]) * 1000,
	//		Market: pair,
	//		Symbol: pair.ToLowerSymbol("/"),
	//	})
	//}

	return trades
}
