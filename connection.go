package exapi

import (
	"errors"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

// 封装websocket连接，可双工读写
type Connection struct {
	// 互斥锁
	sync.Mutex
	// 存放websocket连接
	wsConn *websocket.Conn
	// 是否已关闭
	isClosed bool
	// 消息处理函数，当有数据时调用，当断开连接时，data传nil
	OnMessage func([]byte) error
}

func connect(wsurl, proxyurl string) (wsConn *websocket.Conn, err error) {
	dialer := websocket.DefaultDialer

	if proxyurl != "" {
		proxy, err := url.Parse(proxyurl)
		if err == nil {
			dialer.Proxy = http.ProxyURL(proxy)
		} else {
			return nil, err
		}
	}

	wsConn, resp, err := dialer.Dial(wsurl, nil)
	if err != nil {
		if resp != nil {
			dumpData, _ := httputil.DumpResponse(resp, true)
			log.Printf("[ws][%s] websocket dump connect:%s\n", wsurl, string(dumpData))
		}
		return nil, err
	}

	return wsConn, nil
}

// 初始化长连接
func NewConnectionWithURL(wsurl, proxyurl string, onMessage func([]byte) error) (conn *Connection, err error) {
	wsConn, err := connect(wsurl, proxyurl)
	if err != nil {
		return nil, err
	}

	return NewConnection(wsConn, onMessage)
}

// 初始化长连接
func NewConnection(wsConn *websocket.Conn, onMessage func([]byte) error) (conn *Connection, err error) {
	conn = &Connection{
		wsConn:    wsConn,
		isClosed:  false,
		OnMessage: onMessage,
	}

	// 启动读协程
	go conn.readLoop()

	return
}

// 发送数据
func (conn *Connection) SendMessage(data []byte) (err error) {
	if conn.isClosed {
		return errors.New("Connection is closed")
	}

	// 上锁，发送数据加锁
	conn.Lock()
	defer conn.Unlock()

	if err = conn.wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
		conn.close()
	}
	return err
}

// 接收数据
func (conn *Connection) ReceiveMessage() (data []byte, err error) {
	if conn.isClosed {
		return nil, errors.New("Connection is closed")
	}

	_, data, err = conn.wsConn.ReadMessage()
	if err != nil {
		Error("[ws] websocket ReadMessage failed:%v", err)
		conn.Close()
	}

	return data, err
}

// 关闭连接
func (conn *Connection) Close() {
	conn.Lock()
	defer conn.Unlock()

	conn.close()
}

// 关闭
func (conn *Connection) close() {
	if !conn.isClosed {
		conn.wsConn.Close()
		conn.isClosed = true

		if conn.OnMessage != nil {
			conn.OnMessage(nil) // 发送断开消息
		}
	}
}

// 循环读取数据
func (conn *Connection) readLoop() {
	for {
		var data []byte
		var err error

		if _, data, err = conn.wsConn.ReadMessage(); err != nil {
			Error("[ws] websocket ReadMessage failed:%v", err)
			conn.Close()
			break
		}

		if conn.OnMessage != nil {
			conn.OnMessage(data)
		}
	}
}
