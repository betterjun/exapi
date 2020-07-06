package okex

import (
	"bytes"
	"compress/flate"
	"fmt"
	. "github.com/betterjun/exapi"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

type OkexSpotWs struct {
	SpotWsBase
}

func NewSpotWebsocket(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://real.OKEx.com:8443/ws/v3"
	}

	ws := &OkexSpotWs{}
	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.SetHeartBeatHandler(func(conn *Connection) (err error) {
		//Error("OkexSpotWs:HeartBeat")
		//发了心跳，好像起了反作用，会停顿一下
		//conn.SendMessage([]byte(`{"event":"ping"}`))
		return nil
	})
	ws.HeartbeatIntervalTime = time.Second * 30
	ws.SpotWsBase.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
	ws.SpotWsBase.SpotWebsocket = ws

	go ws.SpotWsBase.Loop()
	return ws, err
}

func (ws *OkexSpotWs) GetExchangeName() string {
	return OKEX
}

// 格式化流名称
func (ws *OkexSpotWs) FormatTopicName(topic string, pair CurrencyPair) string {
	symbol := pair.ToSymbol("-")
	switch topic {
	case STREAM_TICKER:
		return fmt.Sprintf("spot/ticker:%v", symbol)
	case STREAM_DEPTH:
		return fmt.Sprintf("spot/depth5:%v", symbol)
	case STREAM_TRADE:
		return fmt.Sprintf("spot/trade:%v", symbol)
	default:
		return ""
	}
}

// 格式化流订阅消息
func (ws *OkexSpotWs) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	stream := ws.FormatTopicName(topic, pair)
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"op": "subscribe", "args": []string{stream}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"op": "subscribe", "args": []string{stream}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"op": "subscribe", "args": []string{stream}})
	default:
		return nil
	}
}

// 格式化流取消订阅消息
func (ws *OkexSpotWs) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	stream := ws.FormatTopicName(topic, pair)
	switch topic {
	case STREAM_TICKER:
		return ws.Pack(map[string]interface{}{"op": "unsubscribe", "args": []string{stream}})
	case STREAM_DEPTH:
		return ws.Pack(map[string]interface{}{"op": "unsubscribe", "args": []string{stream}})
	case STREAM_TRADE:
		return ws.Pack(map[string]interface{}{"op": "unsubscribe", "args": []string{stream}})
	default:
		return nil
	}
}

func gzipDecode(in []byte) ([]byte, error) {
	reader := flate.NewReader(bytes.NewReader(in))
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

/*
订阅响应
{"event": "<value>","channel":"<value>"}

数据
{"table":"channel","data":"[{"<value1>","<value2>"}]"}
*/
type generalResponse struct {
	Event   string        `json:"event"`
	Channel string        `json:"channel"`
	Table   string        `json:"table"`
	Data    []interface{} `json:"data"`
}

func (ws *OkexSpotWs) OnMessage(data []byte) (err error) {
	if data == nil {
		Error("[ws][%s] websocket OnMessage failed:%v", ws.GetURL(), err)
		ws.SetDisconnected(true)
		return nil
	}

	msg, err := gzipDecode(data)
	if err != nil {
		Error("[ws][%s] websocket unzip failed:%v", ws.GetURL(), err)
		return err
	}

	Log("[ws][%s] websocket received:%v", ws.GetURL(), string(msg))

	var resp generalResponse
	err = json.Unmarshal(msg, &resp)
	if err != nil {
		Error("[ws][%s] websocket json.Unmarshal failed:%v", ws.GetURL(), err)
		return err
	}

	// 响应包
	if len(resp.Event) > 0 {
		return
	}

	// 数据包
	switch resp.Table {
	case "spot/ticker":
		ticker := ws.parseTicker(resp.Data)
		if ws.OnTicker != nil {
			ws.OnTicker(ticker)
		}
	case "spot/depth5":
		dep := ws.parseDepth(resp.Data)
		if ws.OnDepth != nil {
			ws.OnDepth(dep)
		}
	case "spot/trade":
		trades := ws.parseTrade(resp.Data)
		if ws.OnTrade != nil {
			ws.OnTrade(trades)
		}
	}

	return nil
}

func (ws *OkexSpotWs) parseTicker(data []interface{}) *Ticker {
	/*
		{
		            "instrument_id":"ETH-USDT",
		            "last_qty":"0.082483",
		            "best_bid":"146.24",
		            "best_bid_size":"0.006822",
		            "best_ask":"146.25",
		            "best_ask_size":"80.541709",
		            "high_24h":"147.48",
		            "low_24h":"143.88",
		            "base_volume_24h":"117387.58",
		            "quote_volume_24h":"17159427.21",
		            "timestamp":"2019-12-11T02:31:40.436Z"
		        }
	*/
	if len(data) == 0 {
		return nil
	}
	tickerMap, _ := data[0].(map[string]interface{})

	pair := toSymbol(tickerMap["instrument_id"])
	return &Ticker{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Open:   ToFloat64(tickerMap["open_24h"]),
		Last:   ToFloat64(tickerMap["last"]),
		High:   ToFloat64(tickerMap["high_24h"]),
		Low:    ToFloat64(tickerMap["low_24h"]),
		Vol:    ToFloat64(tickerMap["base_volume_24h"]),
		Buy:    ToFloat64(tickerMap["best_bid"]),
		Sell:   ToFloat64(tickerMap["best_ask"]),
		TS:     toTimestamp(tickerMap["timestamp"]),
	}
}

func (ws *OkexSpotWs) parseDepth(data []interface{}) (dep *Depth) {
	depthMap, _ := data[0].(map[string]interface{})

	dep = &Depth{}
	dep.Market = toSymbol(depthMap["instrument_id"])
	dep.Symbol = dep.Market.ToLowerSymbol("/")
	dep.TS = toTimestamp(depthMap["timestamp"])

	bids, _ := depthMap["bids"].([]interface{})
	for _, v := range bids {
		bid, _ := v.([]interface{})
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(bid[0]), ToFloat64(bid[1])})
	}

	asks, _ := depthMap["asks"].([]interface{})
	for _, v := range asks {
		ask, _ := v.([]interface{})
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(ask[0]), ToFloat64(ask[1])})
	}

	return dep
}

func (ws *OkexSpotWs) parseTrade(data []interface{}) (trades []Trade) {
	trades = make([]Trade, 0)
	for _, v := range data {
		tradeMap, _ := v.(map[string]interface{})

		pair := toSymbol(tradeMap["instrument_id"])
		trades = append(trades, Trade{
			Tid:    ToInt64(tradeMap["trade_id"]),
			Side:   AdaptTradeSide(ToString(tradeMap["side"])),
			Amount: ToFloat64(tradeMap["size"]),
			Price:  ToFloat64(tradeMap["price"]),
			TS:     toTimestamp(tradeMap["timestamp"]),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
	}

	return trades
}

func (ws *OkexSpotWs) parseKline(data []interface{}) (klines []Kline) {
	klines = make([]Kline, 0)
	for _, v := range data {
		klineMap, _ := v.(map[string]interface{})
		candle, _ := klineMap["candle"].([]interface{})

		pair := toSymbol(klineMap["instrument_id"])
		klines = append(klines, Kline{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			TS:     toTimestamp(candle[0]),
			Open:   ToFloat64(candle[1]),
			Close:  ToFloat64(candle[4]),
			High:   ToFloat64(candle[2]),
			Low:    ToFloat64(candle[3]),
			Vol:    ToFloat64(candle[5]),
		})
	}

	return klines
}

// "instrument_id":"ETH-USDT",
func toSymbol(si interface{}) (market CurrencyPair) {
	symbolStr, _ := si.(string)
	sA := strings.Split(symbolStr, "-")
	if len(sA) != 2 {
		log.Printf("解析货币对[%v]错误", si)
		return
	}
	return NewCurrencyPairFromString(strings.Join(sA, "/"))
}

// "timestamp":"2019-04-16T11:03:03.712Z"
func toTimestamp(si interface{}) int64 {
	tsStr, _ := si.(string)
	date, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		log.Printf("解析时间戳[%v]错误%v", si, err)
		return 0
	}

	return int64(date.UnixNano() / int64(time.Millisecond))
}
