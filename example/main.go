package main

import (
	"github.com/betterjun/exapi"
	"github.com/betterjun/exapi/builder"
	"log"
	"time"
)

func main() {
	type exchangeCfgs struct {
		ex, accessKey, secretKey, apiPass string
	}
	exArr := []exchangeCfgs{
		// TODO put your keys here
		exchangeCfgs{exapi.HUOBI, "", "", ""},
	}

	for _, v := range exArr {
		spot_api_test(v.ex, v.accessKey, v.secretKey, v.apiPass, "socks5://127.0.0.1:1060")
		spot_ws_test(v.ex, "socks5://127.0.0.1:1060")
	}
}

func spot_api_test(ex, accessKey, secretKey, apiPass, proxy string) {
	apiBuilder := builder.NewAPIBuilder()
	apiBuilder.HttpProxy(proxy).APIKey(accessKey).APISecretkey(secretKey)
	if ex == exapi.OKEX {
		apiBuilder.ApiPassphrase(apiPass)
	}
	api := apiBuilder.BuildSpot(ex)

	log.Println(ex, "ExchangeName", api.GetExchangeName())

	ssArr, err := api.GetAllCurrencyPair()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(ex, "ssArr", ssArr)

	log.Println(ex, "currency list", exapi.GetCurrencyMap(ssArr))

	cs, err := api.GetCurrencyStatus(exapi.NewCurrency("btc"))
	log.Println(ex, cs, err)
	csmap, err := api.GetAllCurrencyStatus()
	log.Println(ex, csmap, err)

	currencyPair := exapi.NewCurrencyPairFromString("xrp/usdt")

	ticker, err := api.GetTicker(currencyPair)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "ticker", *ticker)

	tickers, err := api.GetAllTicker()
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "tickers", tickers)

	depth, err := api.GetDepth(currencyPair, 50, 0)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "depth", *depth)

	trades, err := api.GetTrades(currencyPair, 10)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "trades", trades)

	klines, err := api.GetKlineRecords(currencyPair, exapi.KLINE_M1, 100, 0)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "klines", klines)

	accounts, err := api.GetAccount()
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "accounts", accounts)

	orderBuy, err := api.LimitBuy(currencyPair, "6000", "0.001")
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "orderBuy", orderBuy)

	orderSell, err := api.LimitSell(currencyPair, "16001", "0.001")
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "orderSell", orderSell)

	orderGet, err := api.GetOrder(orderSell.OrderID, currencyPair)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "orderGet", orderGet)

	orderGet, err = api.GetOrder(orderBuy.OrderID, currencyPair)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "orderGet", orderGet)

	pendingOrders, err := api.GetPendingOrders(currencyPair)
	if err != nil {
		log.Println(ex, err)
		return
	}
	for i, v := range pendingOrders {
		log.Println(ex, "pendingOrders", i, v)
		//ok, err := api.Cancel(v.OrderID, currencyPair)
		//if err != nil {
		//	log.Println(ex, err)
		//	return
		//}
		//log.Println(ex, "ok", ok)
		//
		//orderGet, err = api.GetOrder(v.OrderID, currencyPair)
		//if err != nil {
		//	log.Println(ex, err)
		//	return
		//}
	}

	finishedOrders, err := api.GetFinishedOrders(currencyPair)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "finishedOrders", finishedOrders)

	marketOrderBuy, err := api.MarketBuy(currencyPair, "10")
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "marketOrderBuy", marketOrderBuy)

	orderGet, err = api.GetOrder(marketOrderBuy.OrderID, currencyPair)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "orderGet", orderGet)

	orderDeals, err := api.GetOrderDeal(marketOrderBuy.OrderID, currencyPair)
	for i, v := range orderDeals {
		log.Println(ex, "marketOrderBuy deals", i, v)
	}

	marketOrderSell, err := api.MarketSell(currencyPair, "0.001")
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "marketOrderSell", marketOrderSell)

	orderGet, err = api.GetOrder(marketOrderSell.OrderID, currencyPair)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "orderGet", orderGet)

	orderDeals, err = api.GetOrderDeal(marketOrderSell.OrderID, currencyPair)
	for i, v := range orderDeals {
		log.Println(ex, "marketOrderSell deals", i, v)
	}

	userTrades, err := api.GetUserTrades(currencyPair)
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "userTrades", userTrades)

	accounts, err = api.GetAccount()
	if err != nil {
		log.Println(ex, err)
		return
	}
	log.Println(ex, "accounts", accounts)

	return
}

func spot_ws_test(ex, proxy string) {
	apiBuilder := builder.NewAPIBuilder()
	spotws, err := apiBuilder.BuildSpotWebsocket(ex, proxy)
	if err != nil {
		log.Fatalf("创建%v websocket失败:%v", ex, err)
	}

	markets := []string{
		"xrp/usdt",
		//"btc/usdt",
		//"eth/usdt",
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

	time.Sleep(time.Second * 60)

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
