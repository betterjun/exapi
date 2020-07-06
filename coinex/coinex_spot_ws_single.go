package coinex

import (
	"fmt"
	. "github.com/betterjun/exapi"
	"sort"
	"time"
)

type CoinexSpotWsSingle struct {
	SpotWsBase
}

func NewCoinexSpotWsSingle(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://socket.coinex.com/"
	}

	ws := &CoinexSpotWsSingle{}
	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.SetHeartBeatHandler(func(conn *Connection) (err error) {
		//Log("CoinexSpotWsSingle:HeartBeat")
		conn.SendMessage([]byte(fmt.Sprintf(`{"method":"server.ping","params":[],"id": %v}`, time.Now().Unix())))
		return nil
	})
	ws.HeartbeatIntervalTime = time.Second * 30
	ws.SpotWsBase.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
	ws.SpotWsBase.SpotWebsocket = ws

	go ws.SpotWsBase.Loop()
	return ws, err
}

func (ws *CoinexSpotWsSingle) GetExchangeName() string {
	return COINEX
}

// 格式化流名称
func (ws *CoinexSpotWsSingle) FormatTopicName(topic string, pair CurrencyPair) string {
	symbol := pair.ToSymbol("")
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("%v_ticker", symbol)
	case STREAM_DEPTH:
		return fmt.Sprintf("%v_depth", symbol)
	case STREAM_TRADE:
		return fmt.Sprintf("%v_trade", symbol)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *CoinexSpotWsSingle) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"method": "state.subscribe", "params": []interface{}{pair.ToSymbol("")}, "id": time.Now().Unix()})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"method": "depth.subscribe_full", "params": []interface{}{pair.ToSymbol(""), 20, "0"}, "id": time.Now().Unix()})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"method": "deals.subscribe", "params": []interface{}{pair.ToSymbol("")}, "id": time.Now().Unix()})
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *CoinexSpotWsSingle) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"method": "state.unsubscribe", "params": []interface{}{pair.ToSymbol("")}, "id": time.Now().Unix()})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"method": "depth.unsubscribe_full", "params": []interface{}{pair.ToSymbol("")}, "id": time.Now().Unix()})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"method": "deals.unsubscribe", "params": []interface{}{pair.ToSymbol("")}, "id": time.Now().Unix()})
	default:
		return nil
	}
}

func (ws *CoinexSpotWsSingle) OnMessage(data []byte) (err error) {
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

	dataType, ok := resp["method"].(string)
	if !ok {
		return
	}

	//channel, ok := resp["channel"].(string)
	//if !ok {
	//	return
	//}
	//pair := ws.GetPairByStream(channel)
	// 数据包
	switch dataType {
	case "state.update":
		ticker := ws.parseTicker(resp["params"].([]interface{}))
		if ws.OnTicker != nil {
			ws.OnTicker(ticker)
		}
	case "depth.update":
		dep := ws.parseDepth(resp["params"].([]interface{}))
		if ws.OnDepth != nil {
			ws.OnDepth(dep)
		}
	case "deals.update":
		trades := ws.parseTrade(resp["params"].([]interface{}))
		if ws.OnTrade != nil {
			ws.OnTrade(trades)
		}
	default:
		return nil
	}

	return nil
}

func (ws *CoinexSpotWsSingle) parseTicker(resp []interface{}) *Ticker {
	/*
				[
		       {
		        "ETHUSDT": {
		           "close":"430.33", #close price
		           "deal":"1574489.5181782117",  #value
		           "high":"445.68", #highest price
		           "last":"430.33", #latest price
		           "low":"420.32",  #lowest price
		           "open":"434.11", #open price
		           "period":86400,  #cycle period
		           "volume":"3624.85992531" #volume
		        },
		    }
		  ]
	*/

	if len(resp) == 0 {
		return nil
	}
	dataMap, _ := resp[0].(map[string]interface{})
	for k, v := range dataMap {
		pair := ws.GetPairByStream(k + "_ticker")

		tickerMap, _ := v.(map[string]interface{})
		return &Ticker{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Open:   ToFloat64(tickerMap["open"]),
			Last:   ToFloat64(tickerMap["last"]),
			High:   ToFloat64(tickerMap["high"]),
			Low:    ToFloat64(tickerMap["low"]),
			Vol:    ToFloat64(tickerMap["volume"]),
			//Buy:    ToFloat64(tickerMap["buy"]),
			//Sell:   ToFloat64(tickerMap["sell"]),
			TS: time.Now().UnixNano() / (int64(time.Millisecond)),
		}
	}
	return nil
}

func (ws *CoinexSpotWsSingle) parseDepth(resp []interface{}) (dep *Depth) {
	/*
		[true, {
		            "asks": [["0.198743", "875.27844000"]],
		            "bids": [["0.198394", "61.18503375"]],
		            "last": "0.198620",
		            "time": 1590584132453,
		            "checksum": 2525524644
		        }, "XRPUSDT"]
	*/
	if len(resp) != 3 {
		return nil
	}
	symbol, _ := resp[2].(string)
	pair := ws.GetPairByStream(symbol + "_depth")

	obj, _ := resp[1].(map[string]interface{})

	dep = &Depth{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		TS:     ToInt64(obj["time"]) * 1000,
	}

	bids, _ := obj["bids"].([]interface{})
	for _, v := range bids {
		bid, _ := v.([]interface{})
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(bid[0]), ToFloat64(bid[1])})
	}

	asks, _ := obj["asks"].([]interface{})
	for _, v := range asks {
		ask, _ := v.([]interface{})
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(ask[0]), ToFloat64(ask[1])})
	}
	sort.Sort(dep.AskList)

	return dep
}

func (ws *CoinexSpotWsSingle) parseTrade(resp []interface{}) (trades []Trade) {
	/*
			[
		    "BTCBCH",                        #1.market: See<API invocation description·market>
		    [
		      {
		        "type": "sell",             #order type
		        "time": 1496458040.059284,  #order time
		        "price": "17868.41",        #order price
		        "id": 29433,                #order no.
		        "amount": "0.0281"          #order count
		      }
		    ]
		  ],
	*/
	if len(resp)%2 != 0 {
		return nil
	}

	trades = make([]Trade, 0)
	for i := 0; i < len(resp); i = i + 2 {
		pair := ws.GetPairByStream(resp[i].(string) + "_trade")
		arr, _ := resp[i+1].([]interface{})
		for _, v := range arr {
			tradeMap, _ := v.(map[string]interface{})

			trades = append(trades, Trade{
				Tid:    ToInt64(tradeMap["id"]),
				Side:   adaptTradeSide(ToString(tradeMap["type"])),
				Amount: ToFloat64(tradeMap["amount"]),
				Price:  ToFloat64(tradeMap["price"]),
				TS:     int64(ToFloat64(tradeMap["time"]) * 1000),
				Market: pair,
				Symbol: pair.ToLowerSymbol("/"),
			})
		}
	}

	return trades
}
