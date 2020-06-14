package gate

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]int{
	KLINE_M1:    60,
	KLINE_M5:    300,
	KLINE_M15:   900,
	KLINE_M30:   1800,
	KLINE_H1:    3600,
	KLINE_H4:    14400,
	KLINE_DAY:   86400,
	KLINE_WEEK:  604800,
	KLINE_MONTH: 2592000,
}

func getSign(secret, params string) string {
	key := []byte(secret)
	mac := hmac.New(sha512.New, key)
	mac.Write([]byte(params))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

type Gate struct {
	httpClient *http.Client
	baseUrl    string
	accessKey  string
	secretKey  string
}

func NewSpotAPI(client *http.Client, apiKey, secretKey string) SpotAPI {
	gate := &Gate{
		httpClient: client,
		baseUrl:    "http://data.gateio.life/api2/1/",
		accessKey:  apiKey,
		secretKey:  secretKey,
	}
	return gate
}

func (gate *Gate) GetExchangeName() string {
	return GATE
}

func (gate *Gate) SetURL(exurl string) {
	gate.baseUrl = exurl
}

func (gate *Gate) GetURL() string {
	return gate.baseUrl
}

func (gate *Gate) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	reqURL := gate.baseUrl + "marketinfo"
	resp, err := HttpGet(gate.httpClient, reqURL)
	if err != nil {
		return nil, err
	}

	result := ToString(resp["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", resp["code"], resp["message"])
	}

	pairs := resp["pairs"].([]interface{})
	ssm := make(map[string]SymbolSetting)
	for _, v := range pairs {
		data := v.(map[string]interface{})
		for k, v1 := range data {
			symbol := strings.ToLower(strings.Replace(k, "_", "", -1))
			currencies := strings.Split(k, "_")

			obj := v1.(map[string]interface{})
			// 去掉不可交易的币对
			trade_disabled := ToInt(obj["trade_disabled"])
			buy_disabled := ToInt(obj["buy_disabled"])
			sell_disabled := ToInt(obj["sell_disabled"])
			if trade_disabled == 1 || buy_disabled == 1 || sell_disabled == 1 {
				continue
			}
			ssm[symbol] = SymbolSetting{
				Symbol: symbol,
				Base:   strings.ToLower(currencies[0]),
				Quote:  strings.ToLower(currencies[1]),
				// 经验证，用下面的数字来计算
				MinSize:     math.Pow10(-ToInt(obj["amount_decimal_places"])),
				MinPrice:    math.Pow10(-ToInt(obj["decimal_places"])),
				MinNotional: ToFloat64(obj["min_amount"]),
				MakerFee:    ToFloat64(obj["fee"]) / 100.0,
				TakerFee:    ToFloat64(obj["fee"]) / 100.0,
			}
		}
	}

	return ssm, nil
}

func (gate *Gate) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	all, err := gate.GetAllCurrencyStatus()
	if err != nil {
		return CurrencyStatus{
			Deposit:  false,
			Withdraw: false,
		}, err
	}

	return all[currency.Symbol()], nil
}

func (gate *Gate) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	reqURL := gate.baseUrl + "coininfo"
	resp, err := HttpGet(gate.httpClient, reqURL)
	if err != nil {
		return nil, err
	}

	result := ToString(resp["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", resp["code"], resp["message"])
	}

	coins := resp["coins"].([]interface{})
	all = make(map[string]CurrencyStatus)
	for _, v := range coins {
		data := v.(map[string]interface{})
		for k, v1 := range data {
			obj := v1.(map[string]interface{})
			currency := strings.ToUpper(k)

			delisted := ToInt(obj["delisted"])
			trade_disabled := ToInt(obj["trade_disabled"])
			if delisted == 1 || trade_disabled == 1 { // 已下架或暂停交易的
				all[currency] = CurrencyStatus{
					Deposit:  false,
					Withdraw: false,
				}
			} else {
				all[currency] = CurrencyStatus{
					Deposit:  ToInt(obj["deposit_disabled"]) == 0,
					Withdraw: ToInt(obj["withdraw_disabled"]) == 0,
				}
			}
		}
	}

	return all, nil
}

func (gate *Gate) GetTicker(pair CurrencyPair) (*Ticker, error) {
	symbol := pair.ToLowerSymbol("_")
	resp, err := HttpGet(gate.httpClient, gate.baseUrl+fmt.Sprintf("ticker/%s", symbol))
	if err != nil {
		return nil, err
	}

	result := ToString(resp["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", resp["code"], resp["message"])
	}

	ticker := new(Ticker)
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	//ticker.Open
	ticker.Last, _ = strconv.ParseFloat(resp["last"].(string), 64)
	ticker.High, _ = strconv.ParseFloat(resp["high24hr"].(string), 64)
	ticker.Low, _ = strconv.ParseFloat(resp["low24hr"].(string), 64)
	ticker.Vol, _ = strconv.ParseFloat(resp["baseVolume"].(string), 64)
	ticker.Buy, _ = strconv.ParseFloat(resp["highestBid"].(string), 64)
	ticker.Sell, _ = strconv.ParseFloat(resp["lowestAsk"].(string), 64)
	ticker.TS = time.Now().UnixNano() / int64(time.Millisecond)

	percentChange, _ := strconv.ParseFloat(resp["percentChange"].(string), 64)
	ticker.Open = ticker.Last / (1 + percentChange)

	return ticker, nil
}

func (gate *Gate) GetAllTicker() ([]Ticker, error) {
	resp, err := HttpGet(gate.httpClient, gate.baseUrl+"tickers")
	if err != nil {
		return nil, err
	}

	tickers := make([]Ticker, 0)
	for k, v := range resp {
		tickerMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		ticker := Ticker{}
		ticker.Symbol = strings.Replace(k, "_", "/", -1)
		ticker.Market = NewCurrencyPairFromString(ticker.Symbol)
		//ticker.Open
		ticker.Last, _ = strconv.ParseFloat(tickerMap["last"].(string), 64)
		ticker.High, _ = strconv.ParseFloat(tickerMap["high24hr"].(string), 64)
		ticker.Low, _ = strconv.ParseFloat(tickerMap["low24hr"].(string), 64)
		ticker.Vol, _ = strconv.ParseFloat(tickerMap["baseVolume"].(string), 64)
		ticker.Buy, _ = strconv.ParseFloat(tickerMap["highestBid"].(string), 64)
		ticker.Sell, _ = strconv.ParseFloat(tickerMap["lowestAsk"].(string), 64)
		ticker.TS = time.Now().UnixNano() / int64(time.Millisecond)

		percentChange, _ := strconv.ParseFloat(tickerMap["percentChange"].(string), 64)
		ticker.Open = ticker.Last / (1 + percentChange)
		tickers = append(tickers, ticker)
	}

	return tickers, nil
}

func (gate *Gate) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	symbol := pair.ToLowerSymbol("_")
	resp, err := HttpGet(gate.httpClient, gate.baseUrl+fmt.Sprintf("orderBook/%s", symbol))
	if err != nil {
		return nil, err
	}

	result := ToString(resp["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", resp["code"], resp["message"])
	}

	asks, isok1 := resp["asks"].([]interface{})
	bids, isok2 := resp["bids"].([]interface{})

	if isok2 != true || isok1 != true {
		return nil, errors.New("no depth data!")
	}

	depth := new(Depth)
	depth.Market = pair
	depth.Symbol = pair.ToLowerSymbol("/")
	depth.TS = time.Now().UnixNano() / int64(time.Millisecond)

	for _, e := range bids {
		var r DepthRecord
		ee := e.([]interface{})
		r.Price = ToFloat64(ee[0])
		r.Amount = ToFloat64(ee[1])

		depth.BidList = append(depth.BidList, r)
	}

	for _, e := range asks {
		var r DepthRecord
		ee := e.([]interface{})
		r.Price = ToFloat64(ee[0])
		r.Amount = ToFloat64(ee[1])

		depth.AskList = append(depth.AskList, r)
	}
	sort.Sort(depth.AskList)

	return depth, nil
}

func (gate *Gate) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	symbol := pair.ToLowerSymbol("_")
	resp, err := HttpGet(gate.httpClient, gate.baseUrl+fmt.Sprintf("tradeHistory/%s", symbol))
	if err != nil {
		return nil, err
	}

	result := ToString(resp["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", resp["code"], resp["message"])
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		return nil, errors.New("no trade data!")
	}

	trades := make([]Trade, 0)
	for _, v := range data {
		obj, _ := v.(map[string]interface{})
		t := Trade{
			Tid:    ToInt64(obj["tradeID"]),
			Side:   BUY,
			Amount: ToFloat64(obj["amount"]),
			Price:  ToFloat64(obj["rate"]),
			TS:     ToInt64(obj["timestamp"]) * 1000,
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		}
		if ToString(obj["type"]) == "sell" {
			t.Side = SELL
		}
		trades = append(trades, t)
	}

	return trades, nil
}

func (gate *Gate) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", gate.GetExchangeName(), period)
	}
	symbol := pair.ToLowerSymbol("_")
	resp, err := HttpGet(gate.httpClient, gate.baseUrl+fmt.Sprintf("candlestick2/%s?group_sec=%v&range_hour=8760", symbol, periodS))
	if err != nil {
		return nil, err
	}

	result := ToString(resp["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", resp["code"], resp["message"])
	}

	dataArr, ok := resp["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("data assert failed")
	}

	klines := make([]Kline, 0)
	for _, v := range dataArr {
		obj, _ := v.([]interface{})
		t := Kline{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			TS:     ToInt64(obj[0]) / 1000,
			Open:   ToFloat64(obj[5]),
			Close:  ToFloat64(obj[2]),
			High:   ToFloat64(obj[3]),
			Low:    ToFloat64(obj[4]),
			Vol:    ToFloat64(obj[1]),
		}
		klines = append(klines, t)
	}

	return klines, nil
}

func (gate *Gate) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	return gate.placeOrder(amount, price, "buy", pair)
}

func (gate *Gate) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	return gate.placeOrder(amount, price, "sell", pair)
}

func (gate *Gate) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	// TODO 目前没有找到相关接口
	return nil, fmt.Errorf("unsupport the market order")
}

func (gate *Gate) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	// TODO 目前没有找到相关接口
	return nil, fmt.Errorf("unsupport the market order")
}

func (gate *Gate) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("orderNumber", orderId)
	params.Set("currencyPair", symbol)

	postData := params.Encode()
	sign := getSign(gate.secretKey, postData)
	resp, err := HttpPostForm5(gate.httpClient, gate.baseUrl+"private/cancelOrder", postData, map[string]string{"key": gate.accessKey, "sign": sign})
	if err != nil {
		return false, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return false, err
	}

	result := ToBool(respmap["result"])
	if !result {
		return false, fmt.Errorf("error occurred, code:%v, message:%v", respmap["code"], respmap["message"])
	}

	return true, nil
}

func (gate *Gate) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("orderNumber", orderId)
	params.Set("currencyPair", symbol)

	postData := params.Encode()
	sign := getSign(gate.secretKey, postData)
	resp, err := HttpPostForm5(gate.httpClient, gate.baseUrl+"private/getOrder", postData, map[string]string{"key": gate.accessKey, "sign": sign})
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	result := ToString(respmap["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", respmap["code"], respmap["message"])
	}

	order := new(Order)
	order.OrderID = orderId
	order.Market = pair
	order.Symbol = pair.ToLowerSymbol("/")
	orderMap := respmap["order"].(map[string]interface{})
	parseOrder(order, orderMap)

	return order, nil
}

func (gate *Gate) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("currencyPair", symbol)

	postData := params.Encode()
	sign := getSign(gate.secretKey, postData)
	resp, err := HttpPostForm5(gate.httpClient, gate.baseUrl+"private/openOrders", postData, map[string]string{"key": gate.accessKey, "sign": sign})
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	result := ToString(respmap["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", respmap["code"], respmap["message"])
	}

	orderArr := respmap["orders"].([]interface{})
	var orders []Order
	for _, v := range orderArr {
		ordermap := v.(map[string]interface{})
		order := Order{}
		order.Market = pair
		order.Symbol = pair.ToLowerSymbol("/")
		parseOrder(&order, ordermap)
		orders = append(orders, order)
	}

	return orders, nil
}

func (gate *Gate) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	panic("not supported yet")
}

func (gate *Gate) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("currencyPair", symbol)
	if len(orderId) > 0 {
		params.Set("orderNumber", orderId)
	}

	postData := params.Encode()
	sign := getSign(gate.secretKey, postData)
	resp, err := HttpPostForm5(gate.httpClient, gate.baseUrl+"private/tradeHistory", postData, map[string]string{"key": gate.accessKey, "sign": sign})
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	result := ToString(respmap["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", respmap["code"], respmap["message"])
	}

	tradeArr := respmap["trades"].([]interface{})
	deals := make([]OrderDeal, 0, len(tradeArr))
	for _, v := range tradeArr {
		obj := v.(map[string]interface{})
		deal := OrderDeal{
			OrderID:      ToString(obj["orderid"]),
			DealID:       ToString(obj["id"]),
			TS:           ToInt64(obj["time_unix"]) * 1000,
			Price:        ToFloat64(obj["rate"]),
			FilledAmount: ToFloat64(obj["amount"]),
			Market:       pair,
			Symbol:       pair.ToLowerSymbol("/"),
		}

		deal.FilledCashAmount = deal.Price * deal.FilledAmount
		t := ToString(obj["type"])
		switch t {
		case "buy":
			deal.Side = BUY
		case "sell":
			deal.Side = SELL
		}
		deals = append(deals, deal)
	}

	return deals, nil
}

func (gate *Gate) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("currencyPair", symbol)

	postData := params.Encode()
	sign := getSign(gate.secretKey, postData)
	resp, err := HttpPostForm5(gate.httpClient, gate.baseUrl+"private/tradeHistory", postData, map[string]string{"key": gate.accessKey, "sign": sign})
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	result := ToString(respmap["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", respmap["code"], respmap["message"])
	}

	tradeArr := respmap["trades"].([]interface{})
	trades := make([]Trade, 0, len(tradeArr))
	for _, v := range tradeArr {
		obj := v.(map[string]interface{})
		trade := Trade{
			Tid:    ToInt64(obj["tradeID"]),
			Amount: ToFloat64(obj["amount"]),
			Price:  ToFloat64(obj["rate"]),
			TS:     ToInt64(obj["time_unix"]) * 1000,
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		}

		t := ToString(obj["type"])
		switch t {
		case "buy":
			trade.Side = BUY
		case "sell":
			trade.Side = SELL
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

func (gate *Gate) GetAccount() (*Account, error) {
	params := url.Values{}
	postData := params.Encode()
	sign := getSign(gate.secretKey, postData)
	resp, err := HttpPostForm5(gate.httpClient, gate.baseUrl+"private/balances", postData, map[string]string{"key": gate.accessKey, "sign": sign})
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	result := ToString(respmap["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", respmap["code"], respmap["message"])
	}

	acc := new(Account)
	acc.Exchange = gate.GetExchangeName()
	acc.NetAsset = 0
	acc.Asset = 0
	acc.SubAccounts = make(map[Currency]SubAccount)

	availablemap := respmap["available"].(map[string]interface{})
	lockedmap := respmap["locked"].(map[string]interface{})
	for k, v := range availablemap {
		subAcc := SubAccount{}
		subAcc.Currency = NewCurrency(k)
		subAcc.Amount = ToFloat64(v)
		subAcc.FrozenAmount = ToFloat64(lockedmap[k])
		acc.SubAccounts[subAcc.Currency] = subAcc
	}

	return acc, nil
}

func parseOrder(order *Order, ordermap map[string]interface{}) {
	if len(order.OrderID) == 0 {
		order.OrderID = fmt.Sprint(int64(ToFloat64(ordermap["orderNumber"])))
	}
	order.Price = ToFloat64(ordermap["initialRate"])
	order.Amount = ToFloat64(ordermap["initialAmount"])
	order.DealAmount = ToFloat64(ordermap["filledAmount"])
	order.AvgPrice = ToFloat64(ordermap["filledRate"])
	//	order.Fee = ordermap["fees"].(float64)
	order.TS = ToInt64(ordermap["timestamp"]) * 1000

	//     status: 订单状态 open已挂单 cancelled已取消 closed已完成
	switch ordermap["status"].(string) {
	case "open":
		order.Status = ORDER_UNFINISH
	case "cancelled":
		order.Status = ORDER_CANCEL
	case "closed":
		order.Status = ORDER_FINISH
	}

	switch ordermap["type"].(string) {
	case "sell":
		order.Side = SELL
	case "buy":
		order.Side = BUY
	}
}

func (gate *Gate) placeOrder(amount, price, tradeType string, pair CurrencyPair) (*Order, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("currencyPair", symbol)
	params.Set("rate", price)
	params.Set("amount", amount)

	postData := params.Encode()
	sign := getSign(gate.secretKey, postData)
	resp, err := HttpPostForm5(gate.httpClient, gate.baseUrl+"private/"+tradeType, postData, map[string]string{"key": gate.accessKey, "sign": sign})
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	result := ToString(respmap["result"])
	if result != "true" {
		return nil, fmt.Errorf("error occurred, code:%v, message:%v", respmap["code"], respmap["message"])
	}

	order := new(Order)
	order.OrderID = fmt.Sprintf("%d", int64(respmap["orderNumber"].(float64)))
	order.Price, _ = strconv.ParseFloat(price, 64)
	order.Amount, _ = strconv.ParseFloat(amount, 64)
	order.DealAmount = ToFloat64(respmap["filledAmount"])
	order.TS = int64(ToFloat64(respmap["ctime"]) * 1000)
	order.Status = ORDER_UNFINISH
	order.Market = pair
	order.Symbol = pair.ToLowerSymbol("/")

	switch tradeType {
	case "sell":
		order.Side = SELL
	case "buy":
		order.Side = BUY
	}

	return order, nil
}
