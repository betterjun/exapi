package zb

import (
	"encoding/json"
	"fmt"
	. "github.com/betterjun/exapi"
	"sort"
	"time"
)

type ZbSpotWs struct {
	SpotWsBase
}

func NewSpotWebsocket(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://api.zb.cn/websocket"
	}

	ws := &ZbSpotWs{}
	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.HeartbeatIntervalTime = time.Second * 30
	ws.SpotWsBase.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
	ws.SpotWsBase.SpotWebsocket = ws
	//ws.SetHeartBeatHandler(func  (*Connection) (err error) {
	//	fmt.Println("OkexSpotWs:HeartBeat")
	//	return nil
	//})

	go ws.SpotWsBase.Loop()
	return ws, err
}

func (ws *ZbSpotWs) GetExchangeName() string {
	return ZB
}

// 格式化流名称
func (ws *ZbSpotWs) FormatTopicName(topic string, pair CurrencyPair) string {
	symbol := pair.ToLowerSymbol("")
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("%v_ticker", symbol)
	case STREAM_DEPTH:
		return fmt.Sprintf("%v_depth", symbol)
	case STREAM_TRADE:
		return fmt.Sprintf("%v_trades", symbol)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *ZbSpotWs) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	stream := ws.FormatTopicName(topic, pair)
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"event": "addChannel", "channel": stream})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"event": "addChannel", "channel": stream})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"event": "addChannel", "channel": stream})
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *ZbSpotWs) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	// 中币技术顾问说没有取消订阅的命令
	return nil
}

func (ws *ZbSpotWs) OnMessage(data []byte) (err error) {
	if data == nil {
		Error("[ws][%s] websocket OnMessage failed:%v", ws.GetURL(), err)
		ws.SetDisconnected(true)
		return nil
	}

	msg := data

	Log("[ws][%s] websocket received:%v", ws.GetURL(), string(msg))

	resp := make(map[string]interface{})
	err = json.Unmarshal(msg, &resp)
	if err != nil {
		Error("[ws][%s] websocket json.Unmarshal failed:%v", ws.GetURL(), err)
		return err
	}

	dataType, ok := resp["dataType"].(string)
	if !ok {
		return
	}

	channel, ok := resp["channel"].(string)
	if !ok {
		return
	}
	pair := ws.GetPairByStream(channel)
	// 数据包
	switch dataType {
	case "ticker":
		ticker := ws.parseTicker(resp, pair)
		if ws.OnTicker != nil {
			ws.OnTicker(ticker)
		}
	case "depth":
		dep := ws.parseDepth(resp, pair)
		if ws.OnDepth != nil {
			ws.OnDepth(dep)
		}
	case "trades":
		trades := ws.parseTrade(resp, pair)
		if ws.OnTrade != nil {
			ws.OnTrade(trades)
		}
	default:
		return nil
	}

	return nil
}

func (ws *ZbSpotWs) parseTicker(resp map[string]interface{}, pair CurrencyPair) *Ticker {
	/*
			{
		    "date": "1582820345617",
		    "ticker": {
		        "vol": "86154.0366",
		        "buy": "8918.0",
		        "sell": "8919.99"
		    },
		    "dataType": "ticker",
		    "channel": "btcusdt_ticker"
		}
	*/

	tickerMap, _ := resp["ticker"].(map[string]interface{})
	return &Ticker{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Last:   ToFloat64(tickerMap["last"]),
		High:   ToFloat64(tickerMap["high"]),
		Low:    ToFloat64(tickerMap["low"]),
		Vol:    ToFloat64(tickerMap["vol"]),
		Buy:    ToFloat64(tickerMap["buy"]),
		Sell:   ToFloat64(tickerMap["sell"]),
		TS:     ToInt64(resp["date"]),
	}
}

func (ws *ZbSpotWs) parseDepth(resp map[string]interface{}, pair CurrencyPair) (dep *Depth) {
	/*
		{
		    "asks": [[8952.07, 0.0004], [8951.08, 0.0003], [8950.09, 0.0010], [8950.0, 0.8500], [8949.1, 0.0006], [8948.11, 0.0008], [8947.12, 0.0007], [8946.13, 0.0009], [8945.14, 0.0008], [8944.15, 0.0006], [8943.16, 0.0009], [8942.17, 0.0002], [8941.18, 0.0004], [8940.19, 0.0002], [8939.2, 0.0006], [8938.21, 0.0004], [8937.22, 0.0010], [8936.23, 0.0008], [8935.24, 0.0005], [8935.19, 0.5070], [8934.66, 10.8000], [8934.25, 0.0003], [8933.26, 0.0010], [8932.27, 0.0005], [8931.28, 0.0004], [8930.38, 7.6087], [8930.29, 0.0010], [8929.3, 0.0008], [8928.31, 0.0010], [8928.08, 4.5050], [8927.32, 0.0005], [8926.33, 0.0005], [8926.29, 0.1520], [8925.34, 0.1801], [8925.17, 3.2254], [8925.0, 0.7991], [8924.35, 0.0005], [8924.24, 0.0107], [8923.96, 0.1330], [8923.69, 0.0879], [8923.36, 0.0008], [8923.05, 1.2621], [8922.37, 0.0007], [8921.02, 0.9610], [8920.39, 0.0004], [8920.0, 0.3299], [8919.16, 0.2710], [8918.41, 0.0008], [8917.42, 0.0006], [8916.43, 0.0009]],
		    "dataType": "depth",
		    "bids": [[8914.45, 0.0010], [8913.62, 0.0290], [8913.46, 0.0007], [8912.66, 0.0190], [8912.65, 0.0004], [8912.47, 0.0008], [8911.48, 0.0004], [8910.49, 0.0003], [8910.0, 0.6391], [8909.5, 0.0006], [8908.51, 0.0002], [8908.37, 0.2500], [8907.52, 0.0003], [8906.87, 0.1330], [8906.53, 0.0009], [8906.52, 6.8815], [8906.41, 0.0843], [8906.25, 4.4878], [8905.86, 0.0803], [8905.66, 0.4039], [8905.54, 0.0009], [8904.55, 0.0002], [8904.4, 1.6865], [8903.77, 2.9759], [8903.56, 0.0007], [8902.57, 0.0007], [8902.07, 4.3927], [8901.58, 0.0008], [8900.95, 3.0563], [8900.59, 0.0007], [8899.6, 0.0007], [8898.92, 3.4377], [8898.91, 2.9796], [8898.61, 0.0002], [8897.62, 0.0007], [8896.63, 0.0009], [8895.64, 0.0005], [8894.65, 0.0008], [8893.66, 0.0004], [8893.5, 10.4138], [8892.67, 0.0009], [8891.68, 0.0010], [8890.69, 0.0002], [8889.7, 0.0010], [8888.71, 0.0007], [8888.15, 7.2796], [8887.72, 0.0004], [8887.71, 0.5059], [8886.73, 0.0004], [8885.74, 0.0002]],
		    "channel": "btcusdt_depth",
		    "timestamp": 1582820379
		}
	*/
	dep = &Depth{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		TS:     ToInt64(resp["timestamp"]) * 1000,
	}

	bids, _ := resp["bids"].([]interface{})
	for _, v := range bids {
		bid, _ := v.([]interface{})
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(bid[0]), ToFloat64(bid[1])})
	}

	asks, _ := resp["asks"].([]interface{})
	for _, v := range asks {
		ask, _ := v.([]interface{})
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(ask[0]), ToFloat64(ask[1])})
	}
	sort.Sort(dep.AskList)

	return dep
}

func (ws *ZbSpotWs) parseTrade(resp map[string]interface{}, pair CurrencyPair) (trades []Trade) {
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
	data, _ := resp["data"].([]interface{})
	trades = make([]Trade, 0)
	for _, v := range data {
		tradeMap, _ := v.(map[string]interface{})

		trades = append(trades, Trade{
			Tid:    ToInt64(tradeMap["tid"]),
			Side:   AdaptTradeSide(ToString(tradeMap["type"])),
			Amount: ToFloat64(tradeMap["amount"]),
			Price:  ToFloat64(tradeMap["price"]),
			TS:     ToInt64(tradeMap["date"]) * 1000,
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
	}

	return trades
}
