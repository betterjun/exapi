package builder

import (
	"context"
	. "github.com/betterjun/exapi"
	"github.com/betterjun/exapi/aofex"
	"github.com/betterjun/exapi/binance"
	"github.com/betterjun/exapi/bitz"
	"github.com/betterjun/exapi/coinex"
	"github.com/betterjun/exapi/et"
	"github.com/betterjun/exapi/gate"
	"github.com/betterjun/exapi/huobi"
	"github.com/betterjun/exapi/jbex"
	"github.com/betterjun/exapi/okex"
	"github.com/betterjun/exapi/zb"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

type APIBuilder struct {
	client        *http.Client
	httpTimeout   time.Duration
	apiKey        string
	secretkey     string
	clientId      string
	apiPassphrase string
}

func NewAPIBuilder() (builder *APIBuilder) {
	_client := &http.Client{
		Timeout: 30 * time.Second,
	}
	transport := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 4 * time.Second,
	}
	_client.Transport = transport
	return &APIBuilder{client: _client}
}

func NewCustomAPIBuilder(client *http.Client) (builder *APIBuilder) {
	return &APIBuilder{client: client}
}

func (builder *APIBuilder) APIKey(key string) (_builder *APIBuilder) {
	builder.apiKey = key
	return builder
}

func (builder *APIBuilder) APISecretkey(key string) (_builder *APIBuilder) {
	builder.secretkey = key
	return builder
}

func (builder *APIBuilder) HttpProxy(proxyUrl string) (_builder *APIBuilder) {
	if proxyUrl == "" {
		return builder
	}
	proxy, err := url.Parse(proxyUrl)
	if err != nil {
		return builder
	}
	transport := builder.client.Transport.(*http.Transport)
	transport.Proxy = http.ProxyURL(proxy)
	return builder
}

func (builder *APIBuilder) ClientID(id string) (_builder *APIBuilder) {
	builder.clientId = id
	return builder
}

func (builder *APIBuilder) HttpTimeout(timeout time.Duration) (_builder *APIBuilder) {
	builder.httpTimeout = timeout
	builder.client.Timeout = timeout
	transport := builder.client.Transport.(*http.Transport)
	if transport != nil {
		transport.ResponseHeaderTimeout = timeout
		transport.TLSHandshakeTimeout = timeout
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, timeout)
		}
	}
	return builder
}

func (builder *APIBuilder) ApiPassphrase(apiPassphrase string) (_builder *APIBuilder) {
	builder.apiPassphrase = apiPassphrase
	return builder
}

// 使用默认交易所连接地址构建
func (builder *APIBuilder) BuildSpot(exName string) (api SpotAPI) {
	return builder.BuildSpotWithURL(exName, "")
}

// 使用自定义交易所连接地址构建
func (builder *APIBuilder) BuildSpotWithURL(exName, wsURL string) (api SpotAPI) {
	switch exName {
	case HUOBI:
		api = huobi.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
	case BINANCE:
		api = binance.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
	case OKEX:
		api = okex.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey, builder.apiPassphrase)
	case ZB:
		api = zb.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
	case GATE:
		api = gate.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
	case ET:
		api = et.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
	case COINEX:
		api = coinex.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
	case BITZ:
		api = bitz.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey, builder.apiPassphrase)
	case AOFEX:
		api = aofex.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
	case JBEX:
		api = jbex.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
	default:
		panic("exchange name error [" + exName + "].")
	}
	if len(wsURL) > 0 {
		api.SetURL(wsURL)
	}
	return api
}

// 使用默认交易所连接地址构建
func (builder *APIBuilder) BuildSpotWebsocket(exName, proxyURL string) (ws SpotWebsocket, err error) {
	return builder.BuildSpotWebsocketWithURL(exName, "", proxyURL)
}

// 使用自定义交易所连接地址构建
func (builder *APIBuilder) BuildSpotWebsocketWithURL(exName, wsURL, proxyURL string) (ws SpotWebsocket, err error) {
	switch exName {
	case HUOBI:
		ws, err = huobi.NewSpotWebsocket(wsURL, proxyURL)
	case BINANCE:
		ws, err = binance.NewSpotWebsocket(wsURL, proxyURL)
	case OKEX:
		ws, err = okex.NewSpotWebsocket(wsURL, proxyURL)
	case ZB:
		ws, err = zb.NewSpotWebsocket(wsURL, proxyURL)
	case GATE:
		ws, err = gate.NewSpotWebsocket(wsURL, proxyURL)
	case ET:
		ws, err = et.NewSpotWebsocket(wsURL, proxyURL)
	case COINEX:
		ws, err = coinex.NewSpotWebsocket(wsURL, proxyURL)
	case BITZ:
		ws, err = bitz.NewSpotWebsocket(wsURL, proxyURL)
	case JBEX:
		ws, err = jbex.NewSpotWebsocket(wsURL, proxyURL)
	default:
		log.Printf("exchange [" + exName + "] not supported.\n")
	}

	return ws, err
}

//
//func (builder *APIBuilder) BuildFuture(exName string) (api FutureRestAPI) {
//	switch exName {
//	case OKEX_FUTURE:
//		return okcoin.NewSpotAPI(builder.client, builder.apiKey, builder.secretkey)
//	case HBDM:
//		return huobi.NewHbdm(&APIConfig{HttpClient: builder.client, ApiKey: builder.apiKey, ApiSecretKey: builder.secretkey})
//	case OKEX_SWAP:
//		return okex.NewOKExSwap(&APIConfig{HttpClient: builder.client, Endpoint: "https://www.okex.com", ApiKey: builder.apiKey, ApiSecretKey: builder.secretkey, ApiPassphrase: builder.apiPassphrase})
//	default:
//		panic(fmt.Sprintf("%s not support", exName))
//	}
//}
