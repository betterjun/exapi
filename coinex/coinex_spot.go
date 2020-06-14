package coinex

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CoinEx struct {
	httpClient *http.Client
	accessKey  string
	secretKey  string
	baseurl    string
}

func NewSpotAPI(client *http.Client, apiKey, secretKey string) SpotAPI {
	coinex := &CoinEx{
		httpClient: client,
		accessKey:  apiKey,
		secretKey:  secretKey,
		baseurl:    "https://api.coinex.com/v1/",
	}
	return coinex
}

func (coinex *CoinEx) GetExchangeName() string {
	return COINEX
}

func (coinex *CoinEx) SetURL(exurl string) {
	coinex.baseurl = exurl
}

func (coinex *CoinEx) GetURL() string {
	return coinex.baseurl
}

func checkResult(resp map[string]interface{}) error {
	code, ok := resp["code"].(float64)
	if !ok {
		return fmt.Errorf("code assertion failed")
	}

	if code == 0 {
		return nil
	}
	msg, _ := resp["message"].(string)
	return fmt.Errorf("code=%v, msg=%v", code, msg)
}

//
//type TradeFee struct {
//	Maker float64 `json:"maker,string"` // 挂单手续费
//	Taker float64 `json:"taker,string"` // 吃单手续费
//}
//
//func (coinex *CoinEx) GetTradeFee() (tf *TradeFee, err error) {
//	tf = &TradeFee{
//		Maker: 0.002,
//		Taker: 0.002,
//	}
//	return tf, nil
//}

// 获取支持的交易对
func (coinex *CoinEx) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	params := url.Values{}
	datamap, err := coinex.doRequest("GET", "market/info", &params)
	if err != nil {
		return nil, err
	}

	//err = checkResult(resp)
	//if err != nil {
	//	return nil, err
	//}
	//
	//data, ok := resp["data"].(map[string]interface{})
	//if !ok {
	//	return nil, fmt.Errorf("no data returned")
	//}

	ssm := make(map[string]SymbolSetting)
	for _, o := range datamap {
		obj, _ := o.(map[string]interface{})
		base := ToString(obj["trading_name"])
		quote := ToString(obj["pricing_name"])

		symbol := strings.ToLower(base + quote)
		ssm[symbol] = SymbolSetting{
			Symbol:      symbol,
			Base:        base,
			Quote:       quote,
			MinSize:     math.Pow10(-ToInt(obj["trading_decimal"])),
			MinPrice:    math.Pow10(-ToInt(obj["pricing_decimal"])),
			MinNotional: ToFloat64(obj["min_amount"]),
			MakerFee:    ToFloat64(obj["maker_fee_rate"]),
			TakerFee:    ToFloat64(obj["taker_fee_rate"]),
		}
	}

	return ssm, nil
}

// 获取此币种是否可以充提币
func (coinex *CoinEx) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	all, err := coinex.GetAllCurrencyStatus()
	if err != nil {
		return CurrencyStatus{
			Deposit:  false,
			Withdraw: false,
		}, err
	}

	return all[currency.Symbol()], nil
}

// 获取所有币种是否可以充提币
func (coinex *CoinEx) GetAllCurrencyStatus() (map[string]CurrencyStatus, error) {
	params := url.Values{}
	datamap, err := coinex.doRequest("GET", "common/asset/config", &params)
	if err != nil {
		return nil, err
	}

	all := make(map[string]CurrencyStatus)
	for k, v := range datamap {
		obj, _ := v.(map[string]interface{})
		all[strings.ToUpper(k)] = CurrencyStatus{
			Deposit:  ToBool(obj["can_deposit"]),
			Withdraw: ToBool(obj["can_withdraw"]),
		}
	}

	return all, nil
}

func (coinex *CoinEx) GetTicker(pair CurrencyPair) (*Ticker, error) {
	params := url.Values{}
	params.Set("market", pair.ToSymbol(""))
	datamap, err := coinex.doRequest("GET", "market/ticker", &params)
	if err != nil {
		return nil, err
	}

	tickermap := datamap["ticker"].(map[string]interface{})

	ticker := coinex.parseTicker(tickermap)
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	ticker.TS = int64(ToFloat64(datamap["date"]))

	return ticker, nil
}

func (coinex *CoinEx) GetAllTicker() ([]Ticker, error) {
	params := url.Values{}
	datamap, err := coinex.doRequest("GET", "market/ticker/all", &params)
	if err != nil {
		return nil, err
	}

	ts := int64(ToFloat64(datamap["date"]))

	tickermap := datamap["ticker"].(map[string]interface{})
	tickers := make([]Ticker, 0, len(tickermap))
	for k, v := range tickermap {
		tm := v.(map[string]interface{})
		ticker := coinex.parseTicker(tm)
		base, quote := getSymbol(k)
		if len(base) == 0 || len(quote) == 0 {
			continue
		}
		pair := NewCurrencyPairFromString(base + "/" + quote)
		ticker.Market = pair
		ticker.Symbol = pair.ToLowerSymbol("/")
		ticker.TS = ts

		tickers = append(tickers, *ticker)
	}

	return tickers, nil
}

func (coinex *CoinEx) parseTicker(tickermap map[string]interface{}) *Ticker {
	ticker := new(Ticker)
	ticker.Open, _ = strconv.ParseFloat(tickermap["open"].(string), 64)
	ticker.Last, _ = strconv.ParseFloat(tickermap["last"].(string), 64)
	ticker.High, _ = strconv.ParseFloat(tickermap["high"].(string), 64)
	ticker.Low, _ = strconv.ParseFloat(tickermap["low"].(string), 64)
	ticker.Vol, _ = strconv.ParseFloat(tickermap["vol"].(string), 64)
	ticker.Buy, _ = strconv.ParseFloat(tickermap["buy"].(string), 64)
	ticker.Sell, _ = strconv.ParseFloat(tickermap["sell"].(string), 64)
	return ticker
}

func getSymbol(market string) (base, quote string) {
	fiatCoin := []string{"BTC", "BCH", "ETH", "CET", "USDT", "USDC", "TUSD", "PAX"}

	for _, v := range fiatCoin {
		if strings.HasSuffix(market, v) {
			p := strings.LastIndex(market, v)
			return market[0:p], v
		}
	}

	return "", ""
}

func (coinex *CoinEx) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	params := url.Values{}
	params.Set("market", pair.ToSymbol(""))
	params.Set("merge", "0.00000001")
	params.Set("limit", fmt.Sprint(size))

	datamap, err := coinex.doRequest("GET", "market/depth", &params)
	if err != nil {
		return nil, err
	}

	dep := Depth{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		TS:     int64(ToFloat64(datamap["time"])),
	}
	dep.AskList = make([]DepthRecord, 0, size)
	dep.BidList = make([]DepthRecord, 0, size)

	asks := datamap["asks"].([]interface{})
	bids := datamap["bids"].([]interface{})

	for _, v := range asks {
		r := v.([]interface{})
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(r[0]), ToFloat64(r[1])})
	}

	for _, v := range bids {
		r := v.([]interface{})
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(r[0]), ToFloat64(r[1])})
	}

	sort.Sort(sort.Reverse(dep.AskList))

	return &dep, nil
}

//非个人，整个交易所的交易记录
func (coinex *CoinEx) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	params := url.Values{}
	params.Set("market", pair.ToSymbol(""))
	//params.Set("limit", fmt.Sprint(size))
	resp, err := coinex.doRequestInner("GET", "market/deals", &params)
	if err != nil {
		return nil, err
	}

	retmap := make(map[string]interface{}, 1)
	err = json.Unmarshal(resp, &retmap)
	if err != nil {
		return nil, err
	}

	if ToInt(retmap["code"]) != 0 {
		return nil, errors.New(retmap["message"].(string))
	}

	dataArr := retmap["data"].([]interface{})
	trades := make([]Trade, 0, len(dataArr))
	symbol := pair.ToLowerSymbol("/")
	for _, v := range dataArr {
		obj, _ := v.(map[string]interface{})

		trade := Trade{
			Tid:    ToInt64(obj["id"]),
			Side:   adaptTradeSide(ToString(obj["type"])),
			Amount: ToFloat64(obj["amount"]),
			Price:  ToFloat64(obj["price"]),
			TS:     ToInt64(obj["date_ms"]),
			Market: pair,
			Symbol: symbol,
		}

		if trade.Price == 0 {
			trade.Price = ToFloat64(obj["avg_price"])
			trade.Amount = ToFloat64(obj["deal_amount"])
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]string{
	KLINE_M1:   "1min",
	KLINE_M5:   "5min",
	KLINE_M15:  "15min",
	KLINE_M30:  "30min",
	KLINE_H1:   "1hour",
	KLINE_H4:   "4hour",
	KLINE_DAY:  "1day",
	KLINE_WEEK: "1week",
	//KLINE_MONTH: "1mon",
}

func (coinex *CoinEx) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", coinex.GetExchangeName(), period)
	}

	params := url.Values{}
	params.Set("market", pair.ToSymbol(""))
	params.Set("limit", fmt.Sprint(size))
	params.Set("type", periodS)
	resp, err := coinex.doRequestInner("GET", "market/kline", &params)

	if err != nil {
		return nil, err
	}

	retmap := make(map[string]interface{}, 1)
	err = json.Unmarshal(resp, &retmap)
	if err != nil {
		return nil, err
	}

	if ToInt(retmap["code"]) != 0 {
		return nil, errors.New(retmap["message"].(string))
	}

	dataArr := retmap["data"].([]interface{})
	klines := make([]Kline, 0, len(dataArr))
	symbol := pair.ToLowerSymbol("/")
	for _, v := range dataArr {
		obj, _ := v.([]interface{})
		klines = append(klines, Kline{
			Market: pair,
			Symbol: symbol,
			TS:     ToInt64(obj[0]) / 1000,
			Open:   ToFloat64(obj[1]),
			Close:  ToFloat64(obj[2]),
			High:   ToFloat64(obj[3]),
			Low:    ToFloat64(obj[4]),
			Vol:    ToFloat64(obj[5]),
		})
	}

	return klines, nil
}

func (coinex *CoinEx) placeLimitOrder(side, amount, price string, pair CurrencyPair) (*Order, error) {
	params := url.Values{}
	params.Set("market", pair.ToSymbol(""))
	params.Set("type", side)
	params.Set("amount", amount)
	params.Set("price", price)

	retmap, err := coinex.doRequest("POST", "order/limit", &params)
	if err != nil {
		return nil, err
	}

	order := adaptOrder(retmap, pair)

	return order, nil
}

func (coinex *CoinEx) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	return coinex.placeLimitOrder("buy", amount, price, pair)
}

func (coinex *CoinEx) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	return coinex.placeLimitOrder("sell", amount, price, pair)
}

func (coinex *CoinEx) placeMarketOrder(side, amount string, pair CurrencyPair) (*Order, error) {
	params := url.Values{}
	params.Set("market", pair.ToSymbol(""))
	params.Set("type", side)
	params.Set("amount", amount)

	retmap, err := coinex.doRequest("POST", "order/market", &params)
	if err != nil {
		return nil, err
	}

	order := adaptOrder(retmap, pair)

	return order, nil
}

func (coinex *CoinEx) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	return coinex.placeMarketOrder("buy", amount, pair)
}

func (coinex *CoinEx) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	return coinex.placeMarketOrder("sell", amount, pair)
}

func (coinex *CoinEx) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	params := url.Values{}
	params.Set("id", orderId)
	params.Set("market", pair.ToSymbol(""))
	_, err := coinex.doRequest("DELETE", "order/pending", &params)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (coinex *CoinEx) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	params := url.Values{}
	params.Set("id", orderId)
	params.Set("market", pair.ToSymbol(""))
	datamap, err := coinex.doRequest("GET", "order/status", &params)
	if err != nil {
		if strings.Contains(err.Error(), "Order not found") {
			return nil, nil
		}
		return nil, err
	}
	order := adaptOrder(datamap, pair)
	return order, nil
}

func (coinex *CoinEx) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprint(1))
	params.Set("limit", fmt.Sprint(100))
	params.Set("market", pair.ToSymbol(""))

	retmap, err := coinex.doRequest("GET", "order/pending", &params)
	if err != nil {
		return nil, err
	}

	datamap, isok := retmap["data"].([]interface{})
	if !isok {
		log.Println(datamap)
		return nil, errors.New("response format error")
	}

	var orders []Order
	for _, v := range datamap {
		vv := v.(map[string]interface{})
		orders = append(orders, *adaptOrder(vv, pair))
	}

	return orders, nil
}

func (coinex *CoinEx) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprint(1))
	params.Set("limit", fmt.Sprint(100))
	params.Set("market", pair.ToSymbol(""))

	retmap, err := coinex.doRequest("GET", "order/finished", &params)
	if err != nil {
		return nil, err
	}

	datamap, isok := retmap["data"].([]interface{})
	if !isok {
		log.Println(datamap)
		return nil, errors.New("response format error")
	}

	var orders []Order
	for _, v := range datamap {
		vv := v.(map[string]interface{})
		orders = append(orders, *adaptOrder(vv, pair))
	}

	return orders, nil
}

func (coinex *CoinEx) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	params := url.Values{}
	params.Set("id", orderId)
	params.Set("page", fmt.Sprint(1))
	params.Set("limit", fmt.Sprint(100))

	retmap, err := coinex.doRequest("GET", "order/deals", &params)
	if err != nil {
		return nil, err
	}

	datamap, isok := retmap["data"].([]interface{})
	if !isok {
		log.Println(datamap)
		return nil, errors.New("response format error")
	}

	var deals []OrderDeal
	for _, v := range datamap {
		vv := v.(map[string]interface{})
		deals = append(deals, adaptOrderDeal(vv, orderId, pair))
	}

	return deals, nil
}

func adaptOrderDeal(obj map[string]interface{}, orderId string, pair CurrencyPair) OrderDeal {
	return OrderDeal{
		OrderID: orderId,
		// OrderID:          orderId,fmt.Sprint(ToInt64(obj["order_id"])),
		DealID:           fmt.Sprint(ToInt64(obj["id"])),
		TS:               ToInt64(obj["create_time"]) * 1000,
		Price:            ToFloat64(obj["price"]),
		FilledAmount:     ToFloat64(obj["amount"]),
		FilledCashAmount: ToFloat64(obj["deal_money"]),
		Side:             adaptTradeSide(ToString(obj["type"])),
		Market:           pair,
		Symbol:           pair.ToLowerSymbol("/"),
	}
}

func (coinex *CoinEx) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprint(1))
	params.Set("limit", fmt.Sprint(100))
	params.Set("market", pair.ToSymbol(""))

	retmap, err := coinex.doRequest("GET", "order/finished", &params)
	if err != nil {
		return nil, err
	}

	datamap, isok := retmap["data"].([]interface{})
	if !isok {
		log.Println(datamap)
		return nil, errors.New("response format error")
	}

	trades := make([]Trade, 0, len(datamap))
	symbol := pair.ToLowerSymbol("/")
	for _, v := range datamap {
		obj, _ := v.(map[string]interface{})
		trade := Trade{
			Tid:    ToInt64(obj["id"]),
			Side:   adaptTradeSide(ToString(obj["type"])),
			Amount: ToFloat64(obj["amount"]),
			Price:  ToFloat64(obj["price"]),
			TS:     ToInt64(obj["create_time"]) * 1000,
			Market: pair,
			Symbol: symbol,
		}
		if trade.Price == 0 {
			trade.Price = ToFloat64(obj["avg_price"])
			trade.Amount = ToFloat64(obj["deal_amount"])
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

func (coinex *CoinEx) GetAccount() (*Account, error) {
	datamap, err := coinex.doRequest("GET", "balance/info", &url.Values{})
	if err != nil {
		return nil, err
	}
	acc := new(Account)
	acc.SubAccounts = make(map[Currency]SubAccount, 2)
	acc.Exchange = coinex.GetExchangeName()
	for c, v := range datamap {
		vv := v.(map[string]interface{})
		currency := NewCurrency(c)
		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(vv["available"]),
			FrozenAmount: ToFloat64(vv["frozen"])}
	}
	return acc, nil
}

func (coinex *CoinEx) doRequestInner(method, uri string, params *url.Values) (buf []byte, err error) {
	reqUrl := coinex.baseurl + uri

	headermap := map[string]string{
		"Content-Type": "application/json; charset=utf-8"}

	if !strings.HasPrefix(uri, "market") {
		params.Set("access_id", coinex.accessKey)
		params.Set("tonce", fmt.Sprint(time.Now().UnixNano()/int64(time.Millisecond)))
		//	println(params.Encode() + "&secret_key=" + coinex.secretKey)
		sign, _ := GetParamMD5Sign("", params.Encode()+"&secret_key="+coinex.secretKey)
		headermap["authorization"] = strings.ToUpper(sign)
	}

	if ("GET" == method || "DELETE" == method) && len(params.Encode()) > 0 {
		reqUrl += "?" + params.Encode()
	}

	var paramStr string = ""
	if "POST" == method {
		//to json
		paramStr = params.Encode()
		var parammap map[string]string = make(map[string]string, 2)
		for _, v := range strings.Split(paramStr, "&") {
			vv := strings.Split(v, "=")
			parammap[vv[0]] = vv[1]
		}
		jsonData, _ := json.Marshal(parammap)
		paramStr = string(jsonData)
	}

	return NewHttpRequest(coinex.httpClient, method, reqUrl, paramStr, headermap)
}

func (coinex *CoinEx) doRequest(method, uri string, params *url.Values) (map[string]interface{}, error) {
	resp, err := coinex.doRequestInner(method, uri, params)

	if err != nil {
		return nil, err
	}

	retmap := make(map[string]interface{}, 1)
	err = json.Unmarshal(resp, &retmap)
	if err != nil {
		return nil, err
	}

	if ToInt(retmap["code"]) != 0 {
		return nil, errors.New(retmap["message"].(string))
	}

	//	log.Println(retmap)
	datamap := retmap["data"].(map[string]interface{})

	return datamap, nil
}

func adaptOrder(ordermap map[string]interface{}, pair CurrencyPair) *Order {
	ord := &Order{
		OrderID:    fmt.Sprint(ToInt(ordermap["id"])),
		Price:      ToFloat64(ordermap["price"]),
		Amount:     ToFloat64(ordermap["amount"]),
		AvgPrice:   ToFloat64(ordermap["avg_price"]),
		DealAmount: ToFloat64(ordermap["deal_amount"]),
		Fee:        ToFloat64(ordermap["deal_fee"]),
		TS:         ToInt64(ordermap["create_time"]) * 1000,
		Status:     adaptTradeStatus(ordermap["status"].(string)),
		Market:     pair,
		Symbol:     pair.ToLowerSymbol("/"),
		Side:       adaptTradeSide(ordermap["type"].(string)),
	}

	orderType := ToString(ordermap["order_type"])
	if orderType == "market" {
		if ord.Side == BUY {
			ord.Side = BUY_MARKET
			ord.Amount = ord.DealAmount
		} else {
			ord.Side = SELL_MARKET
		}
	}

	return ord
}

func adaptTradeStatus(status string) TradeStatus {
	var tradeStatus TradeStatus = ORDER_UNFINISH
	switch status {
	case "not_deal":
		tradeStatus = ORDER_UNFINISH
	case "done":
		tradeStatus = ORDER_FINISH
	case "partly":
		tradeStatus = ORDER_PART_FINISH
	case "cancel":
		tradeStatus = ORDER_CANCEL
	}
	return tradeStatus
}

func adaptTradeSide(strType string) TradeSide {
	if strType == "buy" {
		return BUY
		//} else if strType == "sell" {
	} else {
		return SELL
	}
}
