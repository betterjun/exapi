package bitz

import (
	"bytes"
	"compress/gzip"
	"fmt"
	. "github.com/betterjun/exapi"
	"io/ioutil"
	"strings"
	"time"
)

// 推送信息格式
type wsSpotResponse struct {
	Params map[string]interface{} `json:"params"` // 订阅的参数
	Action string                 `json:"action"` // 订阅类型
	Ts     int64                  `json:"time"`   // 消息时间
	Data   json.RawMessage
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
		ws.parseTicker(resp.Data, resp.Ts)
	case "Pushdata.depth":
		// "params":{"_CDID":"100002","dataType":"1","symbol":"xrp_usdt","type":"depth"}
		k, _ := resp.Params["symbol"].(string)
		pair := NewCurrencyPairFromString(strings.Replace(k, "_", "/", -1))
		depth := ws.parseDepth(resp.Data, pair)
		if depth != nil {
			depth.TS = resp.Ts
			if ws.OnDepth != nil {
				ws.OnDepth(depth)
			}
		}
	case "Pushdata.order":
		//"params":{"symbol":"xrp_usdt"}
		k, _ := resp.Params["symbol"].(string)
		pair := NewCurrencyPairFromString(strings.Replace(k, "_", "/", -1))
		trade := ws.parseTrade(resp.Data, pair)
		if ws.OnTrade != nil {
			ws.OnTrade(trade)
		}

	default:
		Log("[ws][%s] websocket 未知的topic数据:%v", ws.GetURL(), resp.Action)
		return nil
	}

	return nil
}

func (ws *BitzSpotWs) parseTicker(msg json.RawMessage, ts int64) (ticker *Ticker) {
	tickerMap := map[string]interface{}{}
	err := json.Unmarshal(msg, &tickerMap)
	if err != nil {
		Log("[ws][%s] websocket parseTicker 错误:%v", ws.GetURL(), err)
		return nil
	}

	if ws.OnTicker == nil {
		return nil
	}

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
		ticker := &Ticker{
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

		ws.OnTicker(ticker)
	}

	return nil
}

func (ws *BitzSpotWs) parseDepth(msg json.RawMessage, pair CurrencyPair) (dep *Depth) {
	/*
		TODO 按下面的修改
		1 深度订阅，第一次响应是全量，后面都是增量？当深度中的数量为0，表示删除之前的深度？
		[
		"0.1586", #价格
		"0", #数量
		"97.7638" #总额
		]
	*/
	var depResp wsSpotDepthResponse
	err := json.Unmarshal(msg, &depResp)
	if err != nil {
		Log("[ws][%s] websocket parseDepth 错误:%v", ws.GetURL(), err)
		return nil
	}

	dep = &Depth{}
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")

	for _, bid := range depResp.Bids {
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(bid[0]), ToFloat64(bid[1])})
	}

	for _, ask := range depResp.Asks {
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(ask[0]), ToFloat64(ask[1])})
	}

	return dep
}

func (ws *BitzSpotWs) parseTrade(msg json.RawMessage, pair CurrencyPair) (trades []Trade) {
	traderMap := make([]interface{}, 0)
	err := json.Unmarshal(msg, &traderMap)
	if err != nil {
		Log("[ws][%s] websocket parseTrade 错误:%v", ws.GetURL(), err)
		return nil
	}

	if ws.OnTrade == nil {
		return nil
	}

	trades = make([]Trade, 0, len(traderMap))
	for _, v := range traderMap {
		obj, _ := v.(map[string]interface{})

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
