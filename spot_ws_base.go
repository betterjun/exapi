package exapi

import (
	"encoding/json"
	"fmt"
	"time"
)

type SpotWsBase struct {
	SpotWebsocket
	// 交易所地址
	WsURL string
	// 代理地址
	ProxyURL string
	// 心跳时长
	HeartbeatIntervalTime time.Duration
	// 是否已关闭，不需要重连
	isClosed bool
	// 是否已断开，需要重连
	isDisconnected bool
	// 底层websocket连接
	Conn *Connection
	// 消息主题的map
	TopicMap TopicMap

	// 连接建立处理函数
	OnConnect func(*Connection) error

	// 心跳处理函数
	OnHeartBeat func(*Connection) error

	// callbacks
	OnTicker func(*Ticker) error
	OnDepth  func(*Depth) error
	OnTrade  func([]Trade) error
}

func (ws *SpotWsBase) SetURL(exURL string) {
	ws.WsURL = exURL
}

func (ws *SpotWsBase) GetURL() string {
	return ws.WsURL
}

func (ws *SpotWsBase) SetProxyURL(proxyURL string) {
	ws.ProxyURL = proxyURL
}

func (ws *SpotWsBase) GetProxyURL() string {
	return ws.ProxyURL
}

func (ws *SpotWsBase) SubTicker(pair CurrencyPair, cb func(*Ticker) error) (err error) {
	topic := ws.FormatTopicName(STREAM_TICKER, pair)
	_, ok := ws.TopicMap.Load(topic)
	// 取消
	if cb == nil {
		if !ok {
			return nil
		}

		ws.sendmessage(ws.FormatTopicUnsubData(STREAM_TICKER, pair))
		ws.TopicMap.Delete(topic)
		return nil
	}

	// 订阅
	if ok {
		return nil
	}

	ws.sendmessage(ws.FormatTopicSubData(STREAM_TICKER, pair))
	ws.OnTicker = cb
	ws.TopicMap.Store(topic, pair)
	return nil
}

func (ws *SpotWsBase) SubDepth(pair CurrencyPair, cb func(*Depth) error) (err error) {
	topic := ws.FormatTopicName(STREAM_DEPTH, pair)
	_, ok := ws.TopicMap.Load(topic)
	// 取消
	if cb == nil {
		if !ok {
			return nil
		}

		ws.sendmessage(ws.FormatTopicUnsubData(STREAM_DEPTH, pair))
		ws.TopicMap.Delete(topic)
		return nil
	}

	// 订阅
	if ok {
		return nil
	}

	ws.sendmessage(ws.FormatTopicSubData(STREAM_DEPTH, pair))
	ws.OnDepth = cb
	ws.TopicMap.Store(topic, pair)
	return nil
}

func (ws *SpotWsBase) SubTrade(pair CurrencyPair, cb func([]Trade) error) (err error) {
	topic := ws.FormatTopicName(STREAM_TRADE, pair)
	_, ok := ws.TopicMap.Load(topic)
	// 取消
	if cb == nil {
		if !ok {
			return nil
		}

		ws.sendmessage(ws.FormatTopicUnsubData(STREAM_TRADE, pair))
		ws.TopicMap.Delete(topic)
		return nil
	}

	// 订阅
	if ok {
		return nil
	}

	ws.sendmessage(ws.FormatTopicSubData(STREAM_TRADE, pair))
	ws.OnTrade = cb
	ws.TopicMap.Store(topic, pair)
	return nil
}

// 全部重新订阅
func (ws *SpotWsBase) Resubscribe() (err error) {
	ws.TopicMap.Range(func(k string, v CurrencyPair) bool {
		var data []byte
		if ws.FormatTopicName(STREAM_TICKER, v) == k {
			data = ws.FormatTopicSubData(STREAM_TICKER, v)
		} else if ws.FormatTopicName(STREAM_DEPTH, v) == k {
			data = ws.FormatTopicSubData(STREAM_DEPTH, v)
		} else if ws.FormatTopicName(STREAM_TRADE, v) == k {
			data = ws.FormatTopicSubData(STREAM_TRADE, v)
		}
		if len(data) > 0 {
			ws.sendmessage(data)
		}

		// todo : 注意控制频率，很多websocket都有频率限制的

		return true
	})
	return nil
}

// 全部取消订阅
func (ws *SpotWsBase) Unsubscribe() (err error) {
	ws.TopicMap.Range(func(k string, v CurrencyPair) bool {
		var data []byte
		if ws.FormatTopicName(STREAM_TICKER, v) == k {
			data = ws.FormatTopicUnsubData(STREAM_TICKER, v)
		} else if ws.FormatTopicName(STREAM_DEPTH, v) == k {
			data = ws.FormatTopicUnsubData(STREAM_DEPTH, v)
		} else if ws.FormatTopicName(STREAM_TRADE, v) == k {
			data = ws.FormatTopicUnsubData(STREAM_TRADE, v)
		}
		if len(data) > 0 {
			ws.sendmessage(data)
		}

		// todo : 注意控制频率，很多websocket都有频率限制的

		return true
	})
	return nil
}

// 设置初始化函数，当连接建立时调用
func (ws *SpotWsBase) SetConnectHandler(h func(*Connection) error) {
	ws.OnConnect = h
}

// 设置心跳处理函数
func (ws *SpotWsBase) SetHeartBeatHandler(h func(*Connection) error) {
	ws.OnHeartBeat = h
}

// 关闭连接
func (ws *SpotWsBase) Close() {
	if !ws.isClosed && ws.Conn != nil {
		ws.Conn.Close()
		ws.isClosed = true
		ws.isDisconnected = true
	}
}

func (ws *SpotWsBase) Loop() {
	if ws.HeartbeatIntervalTime <= 0 {
		ws.HeartbeatIntervalTime = time.Second * 30
	}
	heartTimer := time.NewTimer(ws.HeartbeatIntervalTime)
	lastConnectTime := time.Now().Unix()

	var err error
	for !ws.isClosed {
		select {
		case <-heartTimer.C:
			if ws.OnHeartBeat != nil && ws.Conn != nil {
				err = ws.OnHeartBeat(ws.Conn)
				if err != nil {
					Error("[ws][%s] websocket OnHeartBeat failed:%v\n", ws.GetURL(), err)
					ws.isDisconnected = true
				}
			}

			heartTimer.Reset(ws.HeartbeatIntervalTime)

		default:
			if !ws.isDisconnected {
				time.Sleep(time.Millisecond) // 避免cpu持续轮询，造成繁忙假象
				continue
			}

			// 间隔两秒再重连，避免服务器连接太多报错
			if time.Now().Unix()-lastConnectTime <= 2 {
				continue
			}
			lastConnectTime = time.Now().Unix()
			ws.Conn, err = NewConnectionWithURL(ws.GetURL(), ws.GetProxyURL(), ws.OnMessage)
			if err != nil {
				Error("[ws][%s] websocket reconnect failed:%v\n", ws.GetURL(), err)
				continue
			}
			ws.isDisconnected = false

			// 重连后，第一次操作函数
			if ws.OnConnect != nil && ws.Conn != nil {
				err = ws.OnConnect(ws.Conn)
				if err != nil {
					Error("[ws][%s] websocket OnConnect failed:%v\n", ws.GetURL(), err)
					ws.isDisconnected = true
					continue
				}
			}

			// 重新发送订阅消息
			ws.Resubscribe()
		}
	}

	if ws.Conn != nil {
		ws.Conn.Close()
	}
}

func (ws *SpotWsBase) GetPairByStream(topic string) (pair CurrencyPair) {
	pair, _ = ws.TopicMap.Load(topic)
	return pair
}

func (ws *SpotWsBase) SetDisconnected(disconnected bool) {
	ws.isDisconnected = disconnected
}

func (ws *SpotWsBase) Pack(topic interface{}) []byte {
	data, err := json.Marshal(topic)
	if err != nil {
		Error("[ws][%s] websocket Pack failed:%v\n", ws.GetURL(), err)
		return nil
	}

	return data
}

// 发送消息
func (ws *SpotWsBase) sendmessage(data []byte) (err error) {
	if !ws.isClosed && ws.Conn != nil {
		return ws.Conn.SendMessage(data)
	}

	return fmt.Errorf("websocket is closed or disconnected")
}
