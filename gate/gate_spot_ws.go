package gate

import (
	"fmt"
	. "github.com/betterjun/exapi"
	"time"
)

type GateSpotWs struct {
	SpotWsBase

	streamMap map[string]SpotWebsocket
}

func NewSpotWebsocket(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://ws.gate.io/v3"
	}

	ws := &GateSpotWs{}
	ws.streamMap = make(map[string]SpotWebsocket)

	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.HeartbeatIntervalTime = time.Second * 30
	ws.SpotWsBase.SpotWebsocket = ws
	//ws.SetHeartBeatHandler(func  (*Connection) (err error) {
	//	fmt.Println("HuobiSpotWs:HeartBeat")
	//	return nil
	//})

	go ws.SpotWsBase.Loop()
	return ws, err

	return ws, err
}

func (ws *GateSpotWs) GetExchangeName() string {
	return GATE
}

// 格式化流名称
func (ws *GateSpotWs) FormatTopicName(topic string, pair CurrencyPair) string {
	return ws.formatTopicName(topic, pair.ToSymbol("_"))
}

// 格式化流名称
func (ws *GateSpotWs) formatTopicName(topic, symbol string) string {
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
func (ws *GateSpotWs) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
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
func (ws *GateSpotWs) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	return nil
}

func (ws *GateSpotWs) SubTicker(pair CurrencyPair, cb func(*Ticker) error) (err error) {
	stream := fmt.Sprintf("ticker_%v", pair.ToLowerSymbol(""))
	s, ok := ws.streamMap[stream]
	// 取消
	if cb == nil {
		if !ok {
			return nil
		}

		s.Close()
		delete(ws.streamMap, stream)
		return nil
	}

	// 订阅
	if ok {
		return nil
	}

	bs, _ := NewGateSpotWsSingle(ws.WsURL, ws.ProxyURL)
	err = bs.SubTicker(pair, cb)
	if err != nil {
		return err
	}
	ws.streamMap[stream] = bs
	return nil
}

func (ws *GateSpotWs) SubDepth(pair CurrencyPair, cb func(*Depth) error) (err error) {
	stream := fmt.Sprintf("depth_%v", pair.ToLowerSymbol(""))
	s, ok := ws.streamMap[stream]
	// 取消
	if cb == nil {
		if !ok {
			return nil
		}

		s.Close()
		delete(ws.streamMap, stream)
		return nil
	}

	// 订阅
	if ok {
		return nil
	}

	bs, _ := NewGateSpotWsSingle(ws.WsURL, ws.ProxyURL)
	err = bs.SubDepth(pair, cb)
	if err != nil {
		return err
	}
	ws.streamMap[stream] = bs
	return nil
}

func (ws *GateSpotWs) SubTrade(pair CurrencyPair, cb func([]Trade) error) (err error) {
	stream := fmt.Sprintf("trade_%v", pair.ToLowerSymbol(""))
	s, ok := ws.streamMap[stream]
	// 取消
	if cb == nil {
		if !ok {
			return nil
		}

		s.Close()
		delete(ws.streamMap, stream)
		return nil
	}

	// 订阅
	if ok {
		return nil
	}

	bs, _ := NewGateSpotWsSingle(ws.WsURL, ws.ProxyURL)
	err = bs.SubTrade(pair, cb)
	if err != nil {
		return err
	}
	ws.streamMap[stream] = bs
	return nil
}

func (ws *GateSpotWs) OnMessage(data []byte) (err error) {
	return nil
}
