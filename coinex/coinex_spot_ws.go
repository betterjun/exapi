package coinex

import (
	"fmt"
	. "github.com/betterjun/exapi"
	"time"
)

type CoinexSpotWs struct {
	SpotWsBase

	streamMap map[string]SpotWebsocket
}

func NewSpotWebsocket(wsURL, proxyURL string) (sw SpotWebsocket, err error) {
	if len(wsURL) == 0 {
		wsURL = "wss://socket.coinex.com/"
	}

	ws := &CoinexSpotWs{}
	ws.streamMap = make(map[string]SpotWebsocket)

	ws.WsURL = wsURL
	ws.ProxyURL = proxyURL
	ws.HeartbeatIntervalTime = time.Second * 30
	ws.SpotWsBase.SpotWebsocket = ws
	//ws.SetHeartBeatHandler(func  (*Connection) (err error) {
	//	fmt.Println("HuobiSpotWs:HeartBeat")
	//	return nil
	//})

	//go ws.SpotWsBase.Loop()
	return ws, err
}

func (ws *CoinexSpotWs) GetExchangeName() string {
	return COINEX
}

// 格式化流名称
func (ws *CoinexSpotWs) FormatTopicName(topic string, pair CurrencyPair) string {
	return ws.formatTopicName(topic, pair.ToSymbol("_"))
}

// 格式化流名称
func (ws *CoinexSpotWs) formatTopicName(topic, symbol string) string {
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
func (ws *CoinexSpotWs) FormatTopicSubData(topic string, pair CurrencyPair) []byte {
	return nil
}

// 格式化流取消订阅消息
func (ws *CoinexSpotWs) FormatTopicUnsubData(topic string, pair CurrencyPair) []byte {
	return nil
}

func (ws *CoinexSpotWs) SubTicker(pair CurrencyPair, cb func(*Ticker) error) (err error) {
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

	bs, _ := NewCoinexSpotWsSingle(ws.WsURL, ws.ProxyURL)
	err = bs.SubTicker(pair, cb)
	if err != nil {
		return err
	}
	ws.streamMap[stream] = bs
	return nil
}

func (ws *CoinexSpotWs) SubDepth(pair CurrencyPair, cb func(*Depth) error) (err error) {
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

	bs, _ := NewCoinexSpotWsSingle(ws.WsURL, ws.ProxyURL)
	err = bs.SubDepth(pair, cb)
	if err != nil {
		return err
	}
	ws.streamMap[stream] = bs
	return nil
}

func (ws *CoinexSpotWs) SubTrade(pair CurrencyPair, cb func([]Trade) error) (err error) {
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

	bs, _ := NewCoinexSpotWsSingle(ws.WsURL, ws.ProxyURL)
	err = bs.SubTrade(pair, cb)
	if err != nil {
		return err
	}
	ws.streamMap[stream] = bs
	return nil
}

func (ws *CoinexSpotWs) OnMessage(data []byte) (err error) {
	return nil
}
