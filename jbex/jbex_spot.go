package jbex

import (
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	jsoniter "github.com/json-iterator/go"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]string{
	KLINE_M1:    "1m",
	KLINE_M5:    "5m",
	KLINE_M15:   "15m",
	KLINE_M30:   "30m",
	KLINE_H1:    "1h",
	KLINE_H4:    "4h",
	KLINE_DAY:   "1d",
	KLINE_WEEK:  "1w",
	KLINE_MONTH: "1M",
}

type JbexSpot struct {
	httpClient *http.Client
	baseUrl    string
	accessKey  string
	secretKey  string
}

/**
 * spot
 */
func NewSpotAPI(client *http.Client, apikey, secretkey string) SpotAPI {
	jbex := new(JbexSpot)
	jbex.httpClient = client
	jbex.baseUrl = "https://api.jbex.com/"
	jbex.accessKey = apikey
	jbex.secretKey = secretkey
	return jbex
}

func (jbex *JbexSpot) GetExchangeName() string {
	return JBEX
}

func (jbex *JbexSpot) SetURL(exurl string) {
	jbex.baseUrl = exurl
}

func (jbex *JbexSpot) GetURL() string {
	return jbex.baseUrl
}

type TradeFee struct {
	Maker float64 `json:"maker,string"` // 挂单手续费
	Taker float64 `json:"taker,string"` // 吃单手续费
}

func (jbex *JbexSpot) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	url := jbex.baseUrl + "openapi/v1/brokerInfo"
	respmap, err := HttpGet(jbex.httpClient, url)
	if err != nil {
		return nil, err
	}

	// 错误时才返回此字段
	msg, ok := respmap["msg"].(string)
	if ok {
		return nil, fmt.Errorf("code:%v, msg:%v", respmap["code"], msg)
	}

	symbolArr, ok := respmap["symbols"].([]interface{})
	if !ok {
		return nil, errors.New("symbols assert error")
	}

	tf := TradeFee{
		Maker: 0.002,
		Taker: 0.002,
	}

	ssm := make(map[string]SymbolSetting)
	for _, v := range symbolArr {
		d := v.(map[string]interface{})
		filters, ok := d["filters"].([]interface{})
		if !ok {
			return nil, errors.New("filters assert error")
		}

		symbol := strings.ToLower(ToString(d["symbol"]))
		// 发现有这种ETC-SWAP-USDT，不知道做什么的
		if strings.Contains(symbol, "-") {
			continue
		}

		ss := SymbolSetting{
			Symbol:   symbol,
			Base:     strings.ToLower(ToString(d["baseAsset"])),
			Quote:    strings.ToLower(ToString(d["quoteAsset"])),
			MakerFee: tf.Maker,
			TakerFee: tf.Taker,
		}

		for _, f := range filters {
			obj := f.(map[string]interface{})
			filterType := ToString(obj["filterType"])
			switch filterType {
			case "LOT_SIZE":
				ss.MinSize = ToFloat64(obj["minQty"])
			case "PRICE_FILTER":
				ss.MinPrice = ToFloat64(obj["minPrice"])
			case "MIN_NOTIONAL":
				ss.MinNotional = ToFloat64(obj["minNotional"])
			}
		}

		ssm[symbol] = ss
	}

	return ssm, nil
}

func (jbex *JbexSpot) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	all, err := jbex.GetAllCurrencyStatus()
	if err != nil {
		return CurrencyStatus{
			Deposit:  false,
			Withdraw: false,
		}, err
	}

	return all[currency.Symbol()], nil
}

func (jbex *JbexSpot) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	ssm, err := jbex.GetAllCurrencyPair()
	if err != nil {
		return nil, err
	}

	currencyMap := GetCurrencyMap(ssm)

	// 默认都开
	all = make(map[string]CurrencyStatus)
	for k, _ := range currencyMap {
		all[strings.ToUpper(k)] = CurrencyStatus{
			Deposit:  true,
			Withdraw: true,
		}
	}

	return all, nil
}

func (jbex *JbexSpot) GetTicker(pair CurrencyPair) (*Ticker, error) {
	url := jbex.baseUrl + "openapi/quote/v1/ticker/24hr?symbol=" + pair.ToSymbol("")
	respmap, err := HttpGet(jbex.httpClient, url)
	if err != nil {
		return nil, err
	}

	// 错误时才返回此字段
	msg, ok := respmap["msg"].(string)
	if ok {
		return nil, fmt.Errorf("code:%v, msg:%v", respmap["code"], msg)
	}

	return jbex.parseTicker(respmap)
}

func (jbex *JbexSpot) GetAllTicker() ([]Ticker, error) {
	url := jbex.baseUrl + "openapi/quote/v1/ticker/24hr"
	tickerArr, err := HttpGet3(jbex.httpClient, url, nil)
	if err != nil {
		return nil, err
	}

	tickers := make([]Ticker, 0, len(tickerArr))
	for _, v := range tickerArr {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		ticker, err := jbex.parseTicker(obj)
		if err == nil {
			tickers = append(tickers, *ticker)
		}
	}

	return tickers, nil
}

func (jbex *JbexSpot) parseTicker(tickmap map[string]interface{}) (*Ticker, error) {
	base, quote := getSymbol(ToString(tickmap["symbol"]))
	if len(base) == 0 || len(quote) == 0 {
		return nil, fmt.Errorf("invalid symbol %v", tickmap["symbol"])
	}

	ticker := new(Ticker)
	ticker.Symbol = strings.ToLower(base + "/" + quote)
	ticker.Market = NewCurrencyPairFromString(ticker.Symbol)
	ticker.Open = ToFloat64(tickmap["openPrice"])
	ticker.Last = ToFloat64(tickmap["lastPrice"])
	ticker.High = ToFloat64(tickmap["highPrice"])
	ticker.Low = ToFloat64(tickmap["lowPrice"])
	ticker.Vol = ToFloat64(tickmap["volume"])
	//ticker.Buy = ToFloat64(tickmap["bestBidPrice"])
	//ticker.Sell = ToFloat64(tickmap["bestAskPrice"])
	ticker.TS = ToInt64(tickmap["time"])
	return ticker, nil
}

func getSymbol(market string) (base, quote string) {
	fiatCoin := []string{"USDT", "BTC"}

	for _, v := range fiatCoin {
		if strings.HasSuffix(market, v) {
			p := strings.LastIndex(market, v)
			return market[0:p], v
		}
	}

	return "", ""
}

func (jbex *JbexSpot) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	url := jbex.baseUrl + "openapi/quote/v1/depth?symbol=" + pair.ToSymbol("")
	respmap, err := HttpGet(jbex.httpClient, url)
	if err != nil {
		return nil, err
	}

	// 错误时才返回此字段
	msg, ok := respmap["msg"].(string)
	if ok {
		return nil, fmt.Errorf("code:%v, msg:%v", respmap["code"], msg)
	}

	dep := jbex.parseDepthData(respmap)
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")

	return dep, nil
}

func (jbex *JbexSpot) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	url := jbex.baseUrl + "openapi/quote/v1/trades?symbol=" + pair.ToSymbol("")
	tradeArr, err := HttpGet3(jbex.httpClient, url, nil)
	if err != nil {
		return nil, err
	}

	trades := make([]Trade, 0, len(tradeArr))
	for _, v := range tradeArr {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		var side TradeSide
		isBuyerMaker := ToBool(obj["isBuyerMaker"])
		if isBuyerMaker {
			side = BUY
		} else {
			side = SELL
		}

		ts := ToInt64(obj["time"])
		trades = append(trades, Trade{
			Tid:    ts,
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Amount: ToFloat64(obj["qty"]),
			Price:  ToFloat64(obj["price"]),
			Side:   side,
			TS:     ts})
	}
	return trades, nil
}

//倒序
func (jbex *JbexSpot) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", jbex.GetExchangeName(), period)
	}
	url := jbex.baseUrl + "openapi/quote/v1/klines?interval=%s&limit=%d&symbol=%s"
	symbol := pair.ToSymbol("")
	klineArr, err := HttpGet3(jbex.httpClient, fmt.Sprintf(url, periodS, size, symbol), nil)
	if err != nil {
		return nil, err
	}

	klines := make([]Kline, 0, len(klineArr))
	for _, v := range klineArr {
		item, ok := v.([]interface{})
		if !ok {
			continue
		}
		klines = append(klines, Kline{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Open:   ToFloat64(item[1]),
			Close:  ToFloat64(item[4]),
			High:   ToFloat64(item[2]),
			Low:    ToFloat64(item[3]),
			Vol:    ToFloat64(item[5]),
			TS:     ToInt64(item[0]) / 1000})

	}
	return klines, nil
}

func (jbex *JbexSpot) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := jbex.placeOrder(amount, price, pair, "BUY", "LIMIT")
	if err != nil {
		return nil, err
	}
	return &Order{
		Market:  pair,
		Symbol:  pair.ToLowerSymbol("/"),
		OrderID: orderId,
		Amount:  ToFloat64(amount),
		Price:   ToFloat64(price),
		Side:    BUY,
	}, nil
}

func (jbex *JbexSpot) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := jbex.placeOrder(amount, price, pair, "SELL", "LIMIT")
	if err != nil {
		return nil, err
	}
	return &Order{
		Market:  pair,
		Symbol:  pair.ToLowerSymbol("/"),
		OrderID: orderId,
		Amount:  ToFloat64(amount),
		Price:   ToFloat64(price),
		Side:    SELL,
	}, nil
}

func (jbex *JbexSpot) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := jbex.placeOrder(amount, "", pair, "BUY", "MARKET")
	if err != nil {
		return nil, err
	}
	return &Order{
		Market:  pair,
		Symbol:  pair.ToLowerSymbol("/"),
		OrderID: orderId,
		Amount:  ToFloat64(amount),
		Side:    BUY_MARKET,
	}, nil
}

func (jbex *JbexSpot) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := jbex.placeOrder(amount, "", pair, "SELL", "MARKET")
	if err != nil {
		return nil, err
	}
	return &Order{
		Market:  pair,
		Symbol:  pair.ToLowerSymbol("/"),
		OrderID: orderId,
		Amount:  ToFloat64(amount),
		Side:    SELL_MARKET,
	}, nil
}

func (jbex *JbexSpot) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	requrl := jbex.baseUrl + "openapi/v1/order"
	params := map[string]string{}
	params["orderId"] = orderId
	params["timestamp"] = fmt.Sprint(time.Now().Unix() * 1000)
	respmap, err := jbex.httpDelete(requrl, params)
	if err != nil {
		return false, err
	}

	status, ok := respmap["status"].(string)
	if !ok {
		return false, fmt.Errorf("order not cancled, status is empty")
	}
	if status != "CANCELED" {
		return false, fmt.Errorf("order not cancled, status is %v", status)
	}

	return true, nil
}

func (jbex *JbexSpot) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	requrl := jbex.baseUrl + "openapi/v1/order"
	params := map[string]string{}
	params["orderId"] = orderId
	params["timestamp"] = fmt.Sprint(time.Now().Unix() * 1000)
	respmap := make(map[string]interface{})
	err := jbex.httpGet(requrl, params, &respmap)
	if err != nil {
		return nil, err
	}

	return jbex.parseOrder(respmap, pair)
}

func (jbex *JbexSpot) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	requrl := jbex.baseUrl + "openapi/v1/openOrders"
	params := map[string]string{}
	params["timestamp"] = fmt.Sprint(time.Now().Unix() * 1000)
	respArr := make([]map[string]interface{}, 0)
	err := jbex.httpGet(requrl, params, &respArr)
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0, len(respArr))
	for _, v := range respArr {
		ord, err := jbex.parseOrder(v, pair)
		if err != nil {
			continue
		}
		orders = append(orders, *ord)
	}

	return orders, nil
}

func (jbex *JbexSpot) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	requrl := jbex.baseUrl + "openapi/v1/historyOrders"
	params := map[string]string{}
	params["timestamp"] = fmt.Sprint(time.Now().Unix() * 1000)
	respArr := make([]map[string]interface{}, 0)
	err := jbex.httpGet(requrl, params, &respArr)
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0, len(respArr))
	for _, v := range respArr {
		ord, err := jbex.parseOrder(v, pair)
		if err != nil {
			continue
		}
		orders = append(orders, *ord)
	}

	return orders, nil
}

func (jbex *JbexSpot) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	return nil, errors.New("exchange not supported yet")
}

func (jbex *JbexSpot) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	return nil, errors.New("exchange not supported yet")
}

func (jbex *JbexSpot) GetAccount() (*Account, error) {
	requrl := jbex.baseUrl + "openapi/v1/account"
	params := map[string]string{}
	params["timestamp"] = fmt.Sprint(time.Now().Unix() * 1000)
	respmap := make(map[string]interface{})
	err := jbex.httpGet(requrl, params, &respmap)
	if err != nil {
		return nil, err
	}

	list := respmap["balances"].([]interface{})
	acc := new(Account)
	acc.SubAccounts = make(map[Currency]SubAccount, 6)
	acc.Exchange = jbex.GetExchangeName()

	for _, v := range list {
		balancemap := v.(map[string]interface{})
		currencySymbol := balancemap["asset"].(string)
		currency := NewCurrency(currencySymbol)

		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(balancemap["free"]),
			FrozenAmount: ToFloat64(balancemap["locked"]),
		}
	}

	return acc, nil
}

func (jbex *JbexSpot) parseOrder(ordmap map[string]interface{}, pair CurrencyPair) (*Order, error) {
	ord := &Order{
		OrderID:    fmt.Sprint(ToInt(ordmap["orderId"])),
		Amount:     ToFloat64(ordmap["origQty"]),
		Price:      ToFloat64(ordmap["price"]),
		DealAmount: ToFloat64(ordmap["executedQty"]),
		TS:         ToInt64(ordmap["time"]),
	}

	state := ordmap["status"].(string)
	switch state {
	case "NEW":
		ord.Status = ORDER_UNFINISH
	case "PARTIALLY_FILLED":
		ord.Status = ORDER_PART_FINISH
	case "FILLED":
		ord.Status = ORDER_FINISH
	case "CANCELED", "PENDING_CANCEL", "REJECTED":
		ord.Status = ORDER_CANCEL
	default:
		ord.Status = ORDER_UNFINISH
	}

	if ord.DealAmount > 0.0 {
		ord.AvgPrice = ToFloat64(ordmap["cummulativeQuoteQty"]) / ord.DealAmount
	}
	side := ordmap["side"].(string)
	typeS := ordmap["type"].(string)
	switch typeS {
	case "LIMIT":
		if side == "BUY" {
			ord.Side = BUY
		} else {
			ord.Side = SELL
		}
	case "MARKET":
		if side == "BUY" {
			ord.Side = BUY_MARKET
		} else {
			ord.Side = SELL_MARKET
		}

		// 市价，都取成交均价和成交量
		ord.Price = ord.AvgPrice
		ord.Amount = ord.DealAmount
	}

	ord.Market = pair
	ord.Symbol = pair.ToLowerSymbol("/")
	return ord, nil
}

func (jbex *JbexSpot) parseDepthData(tick map[string]interface{}) *Depth {
	bids, _ := tick["bids"].([]interface{})
	asks, _ := tick["asks"].([]interface{})

	depth := new(Depth)
	depth.TS = ToInt64(tick["time"])

	for _, r := range asks {
		var dr DepthRecord
		rr := r.([]interface{})
		dr.Price = ToFloat64(rr[0])
		dr.Amount = ToFloat64(rr[1])
		depth.AskList = append(depth.AskList, dr)
	}

	for _, r := range bids {
		var dr DepthRecord
		rr := r.([]interface{})
		dr.Price = ToFloat64(rr[0])
		dr.Amount = ToFloat64(rr[1])
		depth.BidList = append(depth.BidList, dr)
	}

	//sort.Sort(sort.Reverse(depth.AskList))
	sort.Reverse(depth.AskList)

	return depth
}

func (jbex *JbexSpot) placeOrder(amount, price string, pair CurrencyPair, side, orderType string) (string, error) {
	requrl := jbex.baseUrl + "openapi/v1/order"
	params := map[string]string{}
	params["symbol"] = pair.ToSymbol("")
	params["side"] = side
	params["type"] = orderType
	params["quantity"] = amount

	if strings.Contains(orderType, "LIMIT") {
		// timeInForce, quantity, price
		params["timeInForce"] = "GTC"
		params["price"] = price
	}

	params["timestamp"] = fmt.Sprint(time.Now().Unix() * 1000)
	respmap, err := jbex.httpPost(requrl, params)
	if err != nil {
		return "", err
	}

	return fmt.Sprint(ToInt64(respmap["orderId"])), nil
}

func (jbex *JbexSpot) buildHeaders() map[string]string {
	headers := make(map[string]string)
	headers["X-BH-APIKEY"] = jbex.accessKey
	return headers
}

func (jbex *JbexSpot) sign(params map[string]string) string {
	data := map2UrlQuery(params)
	signature, _ := GetParamHmacSHA256Sign(jbex.secretKey, data)
	return data + "&signature=" + signature
}

func (jbex *JbexSpot) httpPost(requrl string, params map[string]string) (map[string]interface{}, error) {
	data := jbex.sign(params)

	respData, err := HttpPostForm5(jbex.httpClient, requrl, data, jbex.buildHeaders())
	if err != nil {
		return nil, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(respData, &respmap)
	if err != nil {
		log.Printf("json.Unmarshal failed : %v, resp %s\n", err, string(respData))
		return nil, err
	}

	// 错误时才返回此字段
	msg, ok := respmap["msg"].(string)
	if ok {
		return respmap, fmt.Errorf("code:%v, msg:%v", respmap["code"], msg)
	}

	return respmap, nil
}

func (jbex *JbexSpot) httpDelete(requrl string, params map[string]string) (map[string]interface{}, error) {
	data := jbex.sign(params)

	respData, err := HttpDeleteForm2(jbex.httpClient, requrl, data, jbex.buildHeaders())
	if err != nil {
		return nil, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(respData, &respmap)
	if err != nil {
		log.Printf("json.Unmarshal failed : %v, resp %s\n", err, string(respData))
		return nil, err
	}

	// 错误时才返回此字段
	msg, ok := respmap["msg"].(string)
	if ok {
		return respmap, fmt.Errorf("code:%v, msg:%v", respmap["code"], msg)
	}

	return respmap, nil
}

func (jbex *JbexSpot) httpGet(requrl string, params map[string]string, result interface{}) error {
	var strRequestUrl string
	if nil == params {
		strRequestUrl = requrl
	} else {
		strRequestUrl = requrl + "?" + jbex.sign(params)
	}

	err := HttpGet4(jbex.httpClient, strRequestUrl, jbex.buildHeaders(), result)
	if err != nil {
		return err
	}

	return nil
}

// 将map格式的请求参数转换为字符串格式的
// mapParams: map格式的参数键值对
// return: 查询字符串
func map2UrlQuery(mapParams map[string]string) string {
	var strParams string
	values := url.Values{}

	for key, value := range mapParams {
		values.Add(key, value)
	}
	strParams = values.Encode()

	return strParams
}
