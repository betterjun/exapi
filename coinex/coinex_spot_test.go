package coinex

import (
	"github.com/betterjun/exapi"
	"github.com/stretchr/testify/assert"
	"net/http"
	"strings"
	"testing"
	"time"
)

//
//// 测试1: 测试名称是否符合预期
//func TestSpotAPICommon(t *testing.T) {
//	ws, err := NewSpotWebsocket("", "")
//	assert.Equal(t, nil, err)
//	assert.Equal(t, exapi.COINEX, ws.GetExchangeName())
//}

func NewTransport() (client *http.Client) {
	client = &http.Client{}
	transport := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 4 * time.Second,
	}
	client.Transport = transport

	return client
}

// 测试1: 测试名称是否符合预期
func TestSpotAPICommon(t *testing.T) {
	api := NewSpotAPI(NewTransport(), "", "")
	assert.NotEqual(t, nil, api)
	assert.Equal(t, exapi.COINEX, api.GetExchangeName())
	realurl := api.GetURL()

	exurl := "http://test.com"
	api.SetURL(exurl)
	assert.Equal(t, exurl, api.GetURL())

	api.SetURL(realurl)
	assert.Equal(t, realurl, api.GetURL())

	ssm, err := api.GetAllCurrencyPair()
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, ssm)

	currencyName := "BTC"
	currency := exapi.NewCurrency(currencyName)
	cs, err := api.GetCurrencyStatus(currency)
	assert.Equal(t, nil, err)

	csm, err := api.GetAllCurrencyStatus()
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, csm)

	cs2 := csm[currencyName]
	assert.Equal(t, cs, cs2)

	pairName := "btc/usdt"
	pair := exapi.NewCurrencyPairFromString(pairName)
	ticker, err := api.GetTicker(pair)
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, ticker)
	assert.Equal(t, pair, ticker.Market)
	assert.Equal(t, pairName, ticker.Symbol)

	tickers, err := api.GetAllTicker()
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, tickers)
	for _, v := range tickers {
		symbol := strings.Replace(v.Symbol, "/", "", -1)
		_, ok := ssm[symbol]
		assert.Equal(t, true, ok, "ticker "+symbol+" not found")
	}
	//log.Println(ssm)

	depth, err := api.GetDepth(pair, 20, 0)
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, depth)
	assert.Equal(t, pair, depth.Market)
	assert.Equal(t, pairName, depth.Symbol)

	trades, err := api.GetTrades(pair, 20)
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, trades)
	for _, v := range trades {
		assert.Equal(t, pair, v.Market)
		assert.Equal(t, pairName, v.Symbol)
	}

	klines, err := api.GetKlineRecords(pair, exapi.KLINE_M1, 20, 0)
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, klines)
	for _, v := range klines {
		assert.Equal(t, pair, v.Market)
		assert.Equal(t, pairName, v.Symbol)
	}
}
