package bitz

import (
	"bytes"
	"compress/gzip"
	"fmt"
	. "github.com/betterjun/exapi"
	"io/ioutil"
	"sort"
	"strings"
	"time"
)

// 推送信息格式
type wsSpotResponse struct {
	Params interface{} `json:"params"` // 订阅的参数
	Action string      `json:"action"` // 订阅类型
	Ts     int64       `json:"time"`   // 消息时间
	Data   interface{} `json:"data"`   // 返回数据
}

type wsSpotDepthResponse struct {
	Bids [][]interface{}
	Asks [][]interface{}
}

type depthCacheMap struct {
	asksMap map[float64]float64
	bidsMap map[float64]float64
}

type BitzSpotWs struct {
	SpotWsBase
	// 缓存的全量depth
	spotPairDepthMap map[string]*depthCacheMap
}

func NewSpotWebsocket(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://wsapi.bitz.so/"
	}

	ws := &BitzSpotWs{}
	ws.spotPairDepthMap = make(map[string]*depthCacheMap)

	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.SpotWsBase.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
	ws.SpotWsBase.SpotWebsocket = ws
	ws.SetHeartBeatHandler(func(conn *Connection) (err error) {
		Log("BitzSpotWs:HeartBeat")
		conn.SendMessage([]byte(fmt.Sprintf(`{"event": "ping"}`)))
		return nil
	})
	ws.HeartbeatIntervalTime = time.Second * 5

	go ws.SpotWsBase.Loop()
	return ws, err
}

func (ws *BitzSpotWs) GetExchangeName() string {
	return BITZ
}

// 格式化流名称
func (ws *BitzSpotWs) FormatTopicName(topic string, pair CurrencyPair) string {
	symbol := pair.ToLowerSymbol("_")

	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("market.%v", symbol)
	case STREAM_DEPTH:
		return fmt.Sprintf("depth.%v", symbol)
	case STREAM_TRADE:
		return fmt.Sprintf("order.%v", symbol)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *BitzSpotWs) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	symbol := pair.ToLowerSymbol("_")

	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"action": "Topic.sub", "msg_id": time.Now().UnixNano() / int64(time.Millisecond),
			"data": map[string]interface{}{"symbol": symbol, "type": "market", "_CDID": "100002", "dataType": "1"}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"action": "Topic.sub", "msg_id": time.Now().UnixNano() / int64(time.Millisecond),
			"data": map[string]interface{}{"symbol": symbol, "type": "depth", "_CDID": "100002", "dataType": "1"}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"action": "Topic.sub", "msg_id": time.Now().UnixNano() / int64(time.Millisecond),
			"data": map[string]interface{}{"symbol": symbol, "type": "order", "_CDID": "100002", "dataType": "1"}})
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *BitzSpotWs) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	symbol := pair.ToLowerSymbol("_")

	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"action": "Topic.unsub", "msg_id": time.Now().UnixNano() / int64(time.Millisecond),
			"data": map[string]interface{}{"symbol": symbol, "type": "market", "_CDID": "100002", "dataType": "1"}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"action": "Topic.unsub", "msg_id": time.Now().UnixNano() / int64(time.Millisecond),
			"data": map[string]interface{}{"symbol": symbol, "type": "depth", "_CDID": "100002", "dataType": "1"}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"action": "Topic.unsub", "msg_id": time.Now().UnixNano() / int64(time.Millisecond),
			"data": map[string]interface{}{"symbol": symbol, "type": "order", "_CDID": "100002", "dataType": "1"}})
	default:
		return nil
	}
}

// 消息解析函数
func (ws *BitzSpotWs) OnMessage(data []byte) (err error) {
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
	defer r.Close()

	msg, err := ioutil.ReadAll(r)
	if err != nil {
		Error("[ws][%s] websocket gzip.ReadAll failed:%v", ws.GetURL(), err)
		return err
	}

	Log("[ws][%s] websocket received:%v", ws.GetURL(), string(msg))

	var resp wsSpotResponse
	err = json.Unmarshal(msg, &resp)
	if err != nil {
		Error("[ws][%s] websocket json.Unmarshal failed:%v", ws.GetURL(), err)
		return err
	}

	switch resp.Action {
	case "Pushdata.market":
		if ws.OnTicker != nil {
			ticker := ws.parseTicker(resp.Data, resp.Ts)
			if ticker != nil {
				ws.OnTicker(ticker)
			}
		}
	case "Pushdata.depth":
		// "params":{"_CDID":"100002","dataType":"1","symbol":"xrp_usdt","type":"depth"}
		if ws.OnDepth != nil {
			params, ok := resp.Params.(map[string]interface{})
			if !ok {
				Error("[ws][%s] websocket depth params assert failed:%v", ws.GetURL())
				return nil
			}
			k, _ := params["symbol"].(string)
			pushType, _ := params["type"].(string) // 以此字段是否存在来判定是全量还是增量推送,存在这个字段，则是全量
			pair := NewCurrencyPairFromString(strings.Replace(k, "_", "/", -1))
			depth := ws.parseDepth(resp.Data, pair, len(pushType) != 0)
			if depth != nil {
				depth.TS = resp.Ts
				ws.OnDepth(depth)
			}
		}
	case "Pushdata.order":
		//"params":{"symbol":"xrp_usdt"}
		if ws.OnTrade != nil {
			params, ok := resp.Params.(map[string]interface{})
			if !ok {
				Error("[ws][%s] websocket trade params assert failed:%v", ws.GetURL())
				return nil
			}
			k, _ := params["symbol"].(string)
			pair := NewCurrencyPairFromString(strings.Replace(k, "_", "/", -1))
			trade := ws.parseTrade(resp.Data, pair)
			ws.OnTrade(trade)
		}

	default:
		Log("[ws][%s] websocket 未知的topic数据:%v", ws.GetURL(), resp.Action)
	}

	return nil
}

func (ws *BitzSpotWs) parseTicker(msg interface{}, ts int64) (ticker *Ticker) {
	tickerMap := msg.(map[string]interface{})
	for k, v := range tickerMap {
		pair := NewCurrencyPairFromString(strings.Replace(k, "_", "/", -1))
		pairExist := ws.GetPairByStream(ws.FormatTopicName(STREAM_TICKER, pair))
		if pair != pairExist {
			continue
		}
		obj, _ := v.(map[string]interface{})

		/*
			"s": "btc_usdt", #交易对名称
			            "q": "748289393.19", #24小时交易额
			            "v": "68457.02", #24小时交易量
			            "tp": "6.58",  #今日涨跌幅
			            "p24": "11.74", #24小时涨跌幅
			            "o": "10138.95", #开盘价
			            "h": "11500.00",  #24小时最高价
			            "l": "9728.61",  #24小时最低价
			            "n": "11330.00", #当前价格
			            "nP": 4,  #数量展示小数点位
			            "pP": 2, #价格展示小数点位
		*/
		return &Ticker{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Open:   ToFloat64(obj["o"]),
			Last:   ToFloat64(obj["n"]),
			High:   ToFloat64(obj["h"]),
			Low:    ToFloat64(obj["l"]),
			Vol:    ToFloat64(obj["v"]),
			//Buy   :, // 没有最优买卖价
			//Sell  :,
			TS: ts,
		}
	}

	return nil
}

func (ws *BitzSpotWs) parseDepth(msg interface{}, pair CurrencyPair, fullUpdate bool) (dep *Depth) {
	/*
		TODO 按下面的修改
		1 深度订阅，第一次响应是全量，后面都是增量？当深度中的数量为0，表示删除之前的深度？
		[
		"0.1586", #价格
		"0", #数量
		"97.7638" #总额
		]
	*/

	data, ok := msg.(map[string]interface{})
	if !ok {
		return nil
	}

	dep = &Depth{}
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")

	depMap, ok := ws.spotPairDepthMap[dep.Symbol]
	if !ok {
		depMap = &depthCacheMap{
			asksMap: make(map[float64]float64),
			bidsMap: make(map[float64]float64),
		}
		ws.spotPairDepthMap[dep.Symbol] = depMap
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

func (ws *BitzSpotWs) parseTrade(msg interface{}, pair CurrencyPair) (trades []Trade) {
	tradeMap, _ := msg.([]interface{})
	trades = make([]Trade, 0, len(tradeMap))
	for _, v := range tradeMap {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		/*
			"id": 1216814315, #id
			            "t": "21:04:10", #时间
			            "T": 1562159050, #时间戳
			            "p": "0.1599", #价格
			            "n": "2185.0000", #数量
			            "s": "sell" #方向 sell: 卖 buy:买
		*/
		trades = append(trades, Trade{
			Tid:    ToInt64(obj["id"]),
			Price:  ToFloat64(obj["p"]),
			Amount: ToFloat64(obj["n"]),
			Side:   AdaptTradeSide(ToString(obj["s"])),
			TS:     ToInt64(obj["T"]) * 1000,
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
	}
	return trades
}
