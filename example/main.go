package main

import (
	"github.com/betterjun/exapi"
	"github.com/betterjun/exapi/builder"
	"log"
	"time"
)

// 代理
var proxy string = "socks5://127.0.0.1:1060"

// 测试配置
type exchangeCfgs struct {
	ex, accessKey, secretKey, apiPass string
}

// 错误检测
func isErrorOccurred(err error) bool {
	if err != nil && err != exapi.ErrorUnsupported {
		return true
	}
	return false
}

func main() {
	// 测试http公共接口
	//testSpotHttpPublic()

	// 测试http私有接口
	testSpotHttpPrivate()

	// 测试websocket公共接口
	//testSpotWebsocket()
}

// 测试websocket行情订阅
func testSpotWebsocket() {
	exArr := []exchangeCfgs{
		exchangeCfgs{exapi.HUOBI, "", "", ""},
		exchangeCfgs{exapi.ET, "", "", ""},
		exchangeCfgs{exapi.BITZ, "", "", ""},
		exchangeCfgs{exapi.BINANCE, "", "", ""},
		exchangeCfgs{exapi.COINEX, "", "", ""},
		exchangeCfgs{exapi.GATE, "", "", ""},
		exchangeCfgs{exapi.JBEX, "", "", ""},
		exchangeCfgs{exapi.OKEX, "", "", ""},
		exchangeCfgs{exapi.ZB, "", "", ""},
	}

	for _, v := range exArr {
		spot_ws_test(v.ex, proxy)
	}
}

// 测试http公共接口
func testSpotHttpPublic() {
	exArr := []exchangeCfgs{
		exchangeCfgs{exapi.AOFEX, "", "", ""},
		//exchangeCfgs{exapi.BINANCE, "", "", ""}, // 需要设置apikey
		exchangeCfgs{exapi.BITZ, "", "", ""},
		exchangeCfgs{exapi.COINEX, "", "", ""},
		exchangeCfgs{exapi.ET, "", "", ""},
		exchangeCfgs{exapi.GATE, "", "", ""},
		exchangeCfgs{exapi.HUOBI, "", "", ""},
		exchangeCfgs{exapi.JBEX, "", "", ""},
		//exchangeCfgs{exapi.OKEX, "", "", ""}, // 需要设置apikey
		exchangeCfgs{exapi.ZB, "", "", ""},
	}

	for _, v := range exArr {
		spot_api_public_test(v.ex, proxy)
	}
}

func spot_api_public_test(ex, proxy string) {
	apiBuilder := builder.NewAPIBuilder()
	apiBuilder.HttpProxy(proxy)
	api := apiBuilder.BuildSpot(ex)

	if ex != api.GetExchangeName() {
		log.Fatalf("%v, GetExchangeName Failed, expect %q, got %q\n", ex, ex, api.GetExchangeName())
	}

	ssArr, err := api.GetAllCurrencyPair()
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetAllCurrencyPair Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetAllCurrencyPair", ssArr)

	log.Println(ex, "GetCurrencyMap", exapi.GetCurrencyMap(ssArr))

	cs, err := api.GetCurrencyStatus(exapi.NewCurrency("btc"))
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetCurrencyStatus Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetCurrencyStatus", cs)

	csmap, err := api.GetAllCurrencyStatus()
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetAllCurrencyStatus Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetAllCurrencyStatus", csmap)

	currencyPair := exapi.NewCurrencyPairFromString("btc/usdt")
	ticker, err := api.GetTicker(currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetTicker Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetTicker", *ticker)

	tickers, err := api.GetAllTicker()
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetAllTicker Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetAllTicker", tickers)

	depth, err := api.GetDepth(currencyPair, 50, 0)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetDepth Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetDepth", *depth)

	trades, err := api.GetTrades(currencyPair, 10)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetTrades Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetTrades", trades)

	klines, err := api.GetKlineRecords(currencyPair, exapi.KLINE_M1, 100, 0)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetKlineRecords Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetKlineRecords", klines)
}

// 测试http私有接口
func testSpotHttpPrivate() {
	exArr := []exchangeCfgs{
		// TODO 设置各个交易所的apikey，进行测试

		exchangeCfgs{exapi.AOFEX, "", "", ""},
	}

	for _, v := range exArr {
		spot_api_private_test(v.ex, v.accessKey, v.secretKey, v.apiPass, proxy)
	}
}

func spot_api_private_test(ex, accessKey, secretKey, apiPass, proxy string) {
	apiBuilder := builder.NewAPIBuilder()
	apiBuilder.HttpProxy(proxy).APIKey(accessKey).APISecretkey(secretKey).ApiPassphrase(apiPass)
	api := apiBuilder.BuildSpot(ex)

	cp := exapi.NewCurrencyPairFromString("btc/usdt")
	//cp := exapi.NewCurrencyPairFromString("neo/aq")

	spot_api_private_trade_test(ex, api, cp)
	spot_api_private_query_test(ex, api, cp)
}

func spot_api_private_query_test(ex string, api exapi.SpotAPI, currencyPair exapi.CurrencyPair) {
	orders, err := api.GetPendingOrders(currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetPendingOrders Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetPendingOrders", orders)
	spot_api_private_query_order_test(ex, api, currencyPair, orders)

	orders, err = api.GetFinishedOrders(currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetFinishedOrders Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetFinishedOrders", orders)
	spot_api_private_query_order_test(ex, api, currencyPair, orders)

	trades, err := api.GetUserTrades(currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetUserTrades Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetUserTrades", trades)

	accounts, err := api.GetAccount()
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetAccount Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetAccount", accounts)
}

func spot_api_private_query_order_test(ex string, api exapi.SpotAPI, currencyPair exapi.CurrencyPair, orders []exapi.Order) {
	for i, v := range orders {
		log.Println(ex, "orders", i, v)

		orderGet, err := api.GetOrder(v.OrderID, currencyPair)
		if isErrorOccurred(err) {
			log.Fatalf("%v, GetOrder Failed, expect %q, got %q\n", ex, nil, err)
		}
		log.Println(ex, "orderGet", orderGet)

		orderDeals, err := api.GetOrderDeal(v.OrderID, currencyPair)
		if isErrorOccurred(err) {
			log.Fatalf("%v, GetOrderDeal Failed, expect %q, got %q\n", ex, nil, err)
		}
		log.Println(ex, "orderDeals", orderDeals)

		for j, d := range orderDeals {
			log.Println(ex, "GetOrderDeal", j, d)
		}
	}
}

func spot_api_private_trade_test(ex string, api exapi.SpotAPI, currencyPair exapi.CurrencyPair) {
	spot_api_trade_limit_buy(ex, api, currencyPair)
	//spot_api_trade_limit_sell(ex, api, currencyPair)
	spot_api_trade_market_buy(ex, api, currencyPair)
	spot_api_trade_market_sell(ex, api, currencyPair)
}

func spot_api_trade_limit_buy(ex string, api exapi.SpotAPI, currencyPair exapi.CurrencyPair) {
	orderBuy, err := api.LimitBuy(currencyPair, "1000", "0.01")
	if isErrorOccurred(err) {
		log.Fatalf("%v, LimitBuy Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "LimitBuy", orderBuy)

	orderGet, err := api.GetOrder(orderBuy.OrderID, currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetOrder Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetOrder", orderGet)

	ok, err := api.Cancel(orderBuy.OrderID, currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, Cancel Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "Cancel", ok)

	orderGet, err = api.GetOrder(orderBuy.OrderID, currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetOrder Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetOrder", orderGet)
}

func spot_api_trade_limit_sell(ex string, api exapi.SpotAPI, currencyPair exapi.CurrencyPair) {
	orderSell, err := api.LimitSell(currencyPair, "70000", "0.001")
	if isErrorOccurred(err) {
		log.Fatalf("%v, LimitSell Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "LimitSell", orderSell)

	orderGet, err := api.GetOrder(orderSell.OrderID, currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetOrder Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetOrder", orderGet)

	ok, err := api.Cancel(orderSell.OrderID, currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, Cancel Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "Cancel", ok)

	orderGet, err = api.GetOrder(orderSell.OrderID, currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetOrder Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetOrder", orderGet)
}

func spot_api_trade_market_buy(ex string, api exapi.SpotAPI, currencyPair exapi.CurrencyPair) {
	marketOrderBuy, err := api.MarketBuy(currencyPair, "10")
	if isErrorOccurred(err) {
		log.Fatalf("%v, MarketBuy Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "MarketBuy", marketOrderBuy)

	if marketOrderBuy == nil {
		return
	}

	orderGet, err := api.GetOrder(marketOrderBuy.OrderID, currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetOrder Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetOrder", orderGet)
}

func spot_api_trade_market_sell(ex string, api exapi.SpotAPI, currencyPair exapi.CurrencyPair) {
	marketOrderSell, err := api.MarketSell(currencyPair, "2")
	if isErrorOccurred(err) {
		log.Fatalf("%v, MarketSell Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "MarketSell", marketOrderSell)

	if marketOrderSell == nil {
		return
	}

	orderGet, err := api.GetOrder(marketOrderSell.OrderID, currencyPair)
	if isErrorOccurred(err) {
		log.Fatalf("%v, GetOrder Failed, expect %q, got %q\n", ex, nil, err)
	}
	log.Println(ex, "GetOrder", orderGet)
}

func spot_ws_test(ex, proxy string) {
	apiBuilder := builder.NewAPIBuilder()
	spotws, err := apiBuilder.BuildSpotWebsocket(ex, proxy)
	if err != nil {
		log.Fatalf("创建%v websocket失败:%v", ex, err)
	}

	markets := []string{
		//"fm/usdt",
		//"ht/usdt",
		"btc/usdt",
		"eth/usdt",
		//"eos/usdt",
		//"trx/usdt",
		//"xrp/usdt",
		//"etc/usdt",
		//"bsv/usdt",
	}

	for _, v := range markets {
		spotws.SubTicker(exapi.NewCurrencyPairFromString(v), onTicker)
		spotws.SubDepth(exapi.NewCurrencyPairFromString(v), onDepth)
		spotws.SubTrade(exapi.NewCurrencyPairFromString(v), onTrade)
	}

	time.Sleep(time.Second * 6000)

	for _, v := range markets {
		spotws.SubTicker(exapi.NewCurrencyPairFromString(v), nil)
		spotws.SubDepth(exapi.NewCurrencyPairFromString(v), nil)
		spotws.SubTrade(exapi.NewCurrencyPairFromString(v), nil)
	}

	time.Sleep(time.Second * 1)
	spotws.Close()
}

func onTicker(ticker *exapi.Ticker) error {
	log.Println("onTicker", ticker)

	return nil
}

func onDepth(depth *exapi.Depth) error {
	log.Println("onDepth", depth)

	depth.TS = depth.TS / 1000
	st := time.Now().Unix()
	log.Printf("systemTime:%v, depthTime:%v, diff:%vs\n", st, depth.TS, st-depth.TS)
	return nil
}

func onTrade(trades []exapi.Trade) error {
	log.Println("onTrade", trades)

	return nil
}
