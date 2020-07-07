package binance

import (
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	jsoniter "github.com/json-iterator/go"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	TICKER_URI             = "ticker/24hr?symbol=%s"
	ALL_TICKER_URI         = "ticker/24hr"
	DEPTH_URI              = "depth?symbol=%s&limit=%d"
	ACCOUNT_URI            = "account?"
	ORDER_URI              = "order?"
	UNFINISHED_ORDERS_INFO = "openOrders?"
	KLINE_URI              = "klines"
	SERVER_TIME_URL        = "time"
	ALL_ORDER_URI          = "allOrders?"
	MY_TRADES              = "myTrades?"
)

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

type Filter struct {
	FilterType string `json:"filterType"`
	MaxPrice   string `json:"maxPrice"`
	MinPrice   string `json:"minPrice"`
	TickSize   string `json:"tickSize"`
}

type RateLimit struct {
	Interval      string `json:"interval"`
	IntervalNum   int    `json:"intervalNum"`
	Limit         int    `json:"limit"`
	RateLimitType string `json:"rateLimitType"`
}

type TradeSymbol struct {
	BaseAsset              string   `json:"baseAsset"`
	BaseAssetPrecision     int      `json:"baseAssetPrecision"`
	Filters                []Filter `json:"filters"`
	IcebergAllowed         bool     `json:"icebergAllowed"`
	IsMarginTradingAllowed bool     `json:"isMarginTradingAllowed"`
	IsSpotTradingAllowed   bool     `json:"isSpotTradingAllowed"`
	OcoAllowed             bool     `json:"ocoAllowed"`
	OrderTypes             []string `json:"orderTypes"`
	QuoteAsset             string   `json:"quoteAsset"`
	QuotePrecision         int      `json:"quotePrecision"`
	Status                 string   `json:"status"`
	Symbol                 string   `json:"symbol"`
}

type ExchangeInfo struct {
	Timezone        string        `json:"timezone"`
	ServerTime      int           `json:"serverTime"`
	ExchangeFilters []interface{} `json:"exchangeFilters,omitempty"`
	RateLimits      []RateLimit   `json:"rateLimits"`
	Symbols         []TradeSymbol `json:"symbols"`
}

type Binance struct {
	accessKey    string
	secretKey    string
	baseUrl      string
	apiV3        string
	httpClient   *http.Client
	timeoffset   int64 //nanosecond
	tradeSymbols []TradeSymbol
}

func (bn *Binance) buildParamsSigned(postForm *url.Values) error {
	bn.setTimeOffset()
	postForm.Set("recvWindow", "60000")
	tonce := strconv.FormatInt(time.Now().UnixNano()+bn.timeoffset, 10)[0:13]
	postForm.Set("timestamp", tonce)
	payload := postForm.Encode()
	sign, _ := GetParamHmacSHA256Sign(bn.secretKey, payload)
	postForm.Set("signature", sign)
	return nil
}

func NewSpotAPI(client *http.Client, api_key, secret_key string) SpotAPI {
	bn := &Binance{
		baseUrl:    "https://api.binance.com",
		apiV3:      "https://api.binance.com" + "/api/v3/",
		accessKey:  api_key,
		secretKey:  secret_key,
		httpClient: client}
	return bn
}

func (bn *Binance) GetExchangeName() string {
	return BINANCE
}

func (bn *Binance) SetURL(exurl string) {
	bn.baseUrl = exurl
	bn.apiV3 = exurl + "/api/v3/"
}

func (bn *Binance) GetURL() string {
	return bn.baseUrl
}

/*
"maker": "0.001",
"taker": "0.0015",
"timestamp": "2019-12-05T09:06:20.260Z"
*/
type TradeFee struct {
	Maker float64 `json:"maker,string"` // 挂单手续费
	Taker float64 `json:"taker,string"` // 吃单手续费
}

func (bn *Binance) GetTradeFee(symbols string) (tf *TradeFee, err error) {
	tf = &TradeFee{
		Maker: 0.001,
		Taker: 0.001,
	}

	return tf, nil
}

func (bn *Binance) GetTradeFeeMap() (tfmap map[string]TradeFee, err error) {
	params := url.Values{}
	//params.Set("symbol", "ADABNB")
	bn.buildParamsSigned(&params)
	path := bn.baseUrl + "/wapi/v3/tradeFee.html?" + params.Encode()

	// todo: 币安的接口有bug，此接口一直报签名错误
	respmap, err := HttpGet2(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return nil, err
	}

	dataArr, ok := respmap["tradeFee"].([]interface{})
	if !ok {
		return nil, errors.New("tradeFee assert error")
	}

	ssm := make(map[string]TradeFee)
	for _, v := range dataArr {
		d := v.(map[string]interface{})
		symbol := strings.ToLower(ToString(d["symbol"]))
		ssm[symbol] = TradeFee{
			Maker: ToFloat64(d["maker"]),
			Taker: ToFloat64(d["taker"]),
		}
	}

	return ssm, nil
}

func (bn *Binance) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	exchangeUri := bn.apiV3 + "exchangeInfo"
	respmap, err := HttpGet(bn.httpClient, exchangeUri)
	if err != nil {
		return nil, err
	}

	dataArr, ok := respmap["symbols"].([]interface{})
	if !ok {
		return nil, errors.New("symbols assert error")
	}

	//tfmap, err := bn.GetTradeFeeMap()
	//if err != nil {
	//	return nil, err
	//}

	// TODO FIXME: 火币的费率查询，目前最多支持10个币种对。这里用了一个tricky，查一个币种费率，所有币种都用一个费率。
	tf, err := bn.GetTradeFee("btcusdt")
	if err != nil {
		return nil, err
	}

	ssm := make(map[string]SymbolSetting)
	for _, v := range dataArr {
		d := v.(map[string]interface{})
		filters, ok := d["filters"].([]interface{})
		if !ok {
			return nil, errors.New("filters assert error")
		}

		ss := SymbolSetting{
			Base:     strings.ToUpper(ToString(d["baseAsset"])),
			Quote:    strings.ToUpper(ToString(d["quoteAsset"])),
			MakerFee: tf.Maker,
			TakerFee: tf.Taker,
		}
		ss.Symbol = ss.Base + "/" + ss.Quote

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

		ssm[ss.Symbol] = ss
	}

	return ssm, nil
}

func (bn *Binance) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	params := url.Values{}
	bn.buildParamsSigned(&params)
	path := bn.baseUrl + "/sapi/v1/capital/config/getall?" + params.Encode()

	dataArr, err := HttpGet3(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return CurrencyStatus{}, err
	}

	for _, v := range dataArr {
		d := v.(map[string]interface{})
		if strings.ToUpper(ToString(d["coin"])) == currency.Symbol() {
			return CurrencyStatus{
				Deposit:  ToBool(d["depositAllEnable"]),
				Withdraw: ToBool(d["withdrawAllEnable"]),
			}, nil
		}
	}

	return CurrencyStatus{}, ErrorAssetNotFound
}

func (bn *Binance) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	params := url.Values{}
	bn.buildParamsSigned(&params)
	path := bn.baseUrl + "/sapi/v1/capital/config/getall?" + params.Encode()

	dataArr, err := HttpGet3(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return nil, err
	}

	all = make(map[string]CurrencyStatus)
	for _, v := range dataArr {
		d := v.(map[string]interface{})
		all[strings.ToUpper(ToString(d["coin"]))] = CurrencyStatus{
			Deposit:  ToBool(d["depositAllEnable"]),
			Withdraw: ToBool(d["withdrawAllEnable"]),
		}
	}

	return all, nil
}

func (bn *Binance) GetTicker(pair CurrencyPair) (*Ticker, error) {
	currency2 := bn.adaptCurrencyPair(pair)
	tickerUri := bn.apiV3 + fmt.Sprintf(TICKER_URI, currency2.ToSymbol(""))
	tickerMap, err := HttpGet(bn.httpClient, tickerUri)

	if err != nil {
		return nil, err
	}

	var ticker Ticker
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	ticker.Open = ToFloat64(tickerMap["openPrice"])
	ticker.Last = ToFloat64(tickerMap["lastPrice"])
	ticker.High = ToFloat64(tickerMap["highPrice"])
	ticker.Low = ToFloat64(tickerMap["lowPrice"])
	ticker.Vol = ToFloat64(tickerMap["volume"])
	ticker.Buy = ToFloat64(tickerMap["bidPrice"])
	ticker.Sell = ToFloat64(tickerMap["askPrice"])
	ticker.TS = ToInt64(tickerMap["closeTime"])
	return &ticker, nil
}

func (bn *Binance) GetAllTicker() ([]Ticker, error) {
	tickerUri := bn.apiV3 + ALL_TICKER_URI
	data, err := HttpGet3(bn.httpClient, tickerUri, nil)

	if err != nil {
		return nil, err
	}

	tickers := make([]Ticker, 0, len(data))
	for _, v := range data {
		tickerMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		base, quote := getSymbol(ToString(tickerMap["symbol"]))
		if len(base) == 0 || len(quote) == 0 {
			continue
		}
		ticker := Ticker{}
		ticker.Symbol = base + "/" + quote
		ticker.Market = NewCurrencyPairFromString(ticker.Symbol)
		ticker.Open = ToFloat64(tickerMap["openPrice"])
		ticker.Last = ToFloat64(tickerMap["lastPrice"])
		ticker.High = ToFloat64(tickerMap["highPrice"])
		ticker.Low = ToFloat64(tickerMap["lowPrice"])
		ticker.Vol = ToFloat64(tickerMap["volume"])
		ticker.Buy = ToFloat64(tickerMap["bidPrice"])
		ticker.Sell = ToFloat64(tickerMap["askPrice"])
		ticker.TS = ToInt64(tickerMap["closeTime"])
		tickers = append(tickers, ticker)
	}

	return tickers, nil
}

func getSymbol(market string) (base, quote string) {
	fiatCoin := []string{"BNB", "BTC", "ETH", "TRX", "XRP", "USDT", "BUSD", "PAX", "TUSD", "USDC", "EUR", "NGN", "RUB", "TRY"}

	for _, v := range fiatCoin {
		if strings.HasSuffix(market, v) {
			p := strings.LastIndex(market, v)
			return market[0:p], v
		}
	}

	return "", ""
}

func (bn *Binance) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	if size > 1000 {
		size = 1000
	} else if size < 5 {
		size = 5
	}
	currencyPair2 := bn.adaptCurrencyPair(pair)

	apiUrl := fmt.Sprintf(bn.apiV3+DEPTH_URI, currencyPair2.ToSymbol(""), size)
	resp, err := HttpGet(bn.httpClient, apiUrl)
	if err != nil {
		return nil, err
	}

	if _, isok := resp["code"]; isok {
		return nil, errors.New(resp["msg"].(string))
	}

	bids, _ := resp["bids"].([]interface{})
	asks, _ := resp["asks"].([]interface{})

	depth := new(Depth)
	depth.Market = pair
	depth.Symbol = pair.ToLowerSymbol("/")
	for _, bid := range bids {
		_bid := bid.([]interface{})
		amount := ToFloat64(_bid[1])
		price := ToFloat64(_bid[0])
		dr := DepthRecord{Amount: amount, Price: price}
		depth.BidList = append(depth.BidList, dr)
	}

	for _, ask := range asks {
		_ask := ask.([]interface{})
		amount := ToFloat64(_ask[1])
		price := ToFloat64(_ask[0])
		dr := DepthRecord{Amount: amount, Price: price}
		depth.AskList = append(depth.AskList, dr)
	}

	return depth, nil
}

//非个人，整个交易所的交易记录
//注意：since is fromId
func (bn *Binance) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	param := url.Values{}
	param.Set("symbol", bn.adaptCurrencyPair(pair).ToSymbol(""))
	param.Set("limit", "500")
	//if since > 0 {
	//	param.Set("fromId", strconv.Itoa(int(since)))
	//}
	apiUrl := bn.apiV3 + "historicalTrades?" + param.Encode()
	//log.Println(apiUrl)
	resp, err := HttpGet3(bn.httpClient, apiUrl, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return nil, err
	}

	var trades []Trade
	for _, v := range resp {
		m := v.(map[string]interface{})
		ty := SELL
		if m["isBuyerMaker"].(bool) {
			ty = BUY
		}
		trades = append(trades, Trade{
			Tid:    ToInt64(m["id"]),
			Side:   ty,
			Amount: ToFloat64(m["qty"]),
			Price:  ToFloat64(m["price"]),
			TS:     ToInt64(m["time"]),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
	}

	return trades, nil
}

func (bn *Binance) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", bn.GetExchangeName(), period)
	}

	currency2 := bn.adaptCurrencyPair(pair)
	params := url.Values{}
	params.Set("symbol", currency2.ToSymbol(""))

	params.Set("interval", periodS)
	if since > 0 {
		params.Set("startTime", strconv.Itoa(since))
	}
	//params.Set("endTime", strconv.Itoa(int(time.Now().UnixNano()/1000000)))
	if size > 1000 { //最大1000
		size = 1000
	}
	params.Set("limit", fmt.Sprintf("%d", size))

	klineUrl := bn.apiV3 + KLINE_URI + "?" + params.Encode()
	klines, err := HttpGet3(bn.httpClient, klineUrl, nil)
	if err != nil {
		return nil, err
	}
	var klineRecords []Kline

	for _, _record := range klines {
		r := Kline{Market: pair, Symbol: pair.ToLowerSymbol("/")}
		record := _record.([]interface{})
		for i, e := range record {
			switch i {
			case 0:
				r.TS = int64(e.(float64)) / 1000 //to unix timestramp
			case 1:
				r.Open = ToFloat64(e)
			case 2:
				r.High = ToFloat64(e)
			case 3:
				r.Low = ToFloat64(e)
			case 4:
				r.Close = ToFloat64(e)
			case 5:
				r.Vol = ToFloat64(e)
			}
		}
		klineRecords = append(klineRecords, r)
	}

	return klineRecords, nil

}

func (bn *Binance) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	return bn.placeOrder(amount, price, pair, "LIMIT", "BUY")
}

func (bn *Binance) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	return bn.placeOrder(amount, price, pair, "LIMIT", "SELL")
}

func (bn *Binance) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	return bn.placeOrder(amount, "", pair, "MARKET", "BUY")
}

func (bn *Binance) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	return bn.placeOrder(amount, "", pair, "MARKET", "SELL")
}

func (bn *Binance) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	pair = bn.adaptCurrencyPair(pair)
	path := bn.apiV3 + ORDER_URI
	params := url.Values{}
	params.Set("symbol", pair.ToSymbol(""))
	params.Set("orderId", orderId)

	bn.buildParamsSigned(&params)

	resp, err := HttpDeleteForm(bn.httpClient, path, params, map[string]string{"X-MBX-APIKEY": bn.accessKey})

	if err != nil {
		return false, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		log.Println(string(resp))
		return false, err
	}

	orderIdCanceled := ToInt64(respmap["orderId"])
	if orderIdCanceled <= 0 {
		return false, errors.New(string(resp))
	}

	return true, nil
}

func (bn *Binance) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	params := url.Values{}
	pair = bn.adaptCurrencyPair(pair)
	params.Set("symbol", pair.ToSymbol(""))
	if orderId != "" {
		params.Set("orderId", orderId)
	}
	params.Set("orderId", orderId)

	bn.buildParamsSigned(&params)
	path := bn.apiV3 + ORDER_URI + params.Encode()

	respmap, err := HttpGet2(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return nil, err
	}

	return bn.parseOrder(respmap, pair)
}

func (bn *Binance) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	params := url.Values{}
	pair = bn.adaptCurrencyPair(pair)
	params.Set("symbol", pair.ToSymbol(""))

	bn.buildParamsSigned(&params)
	path := bn.apiV3 + UNFINISHED_ORDERS_INFO + params.Encode()

	respmap, err := HttpGet3(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0)
	for _, v := range respmap {
		ord := v.(map[string]interface{})
		order, err := bn.parseOrder(ord, pair)
		if err != nil {
			fmt.Println("err:", err)
			continue
		}
		orders = append(orders, *order)
	}
	return orders, nil
}

func (bn *Binance) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	params := url.Values{}
	pair = bn.adaptCurrencyPair(pair)
	params.Set("symbol", pair.ToSymbol(""))
	//params.Set("limit", 1000)

	bn.buildParamsSigned(&params)
	path := bn.apiV3 + ALL_ORDER_URI + params.Encode()

	respmap, err := HttpGet3(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0)
	for _, v := range respmap {
		ord := v.(map[string]interface{})
		order, err := bn.parseOrder(ord, pair)
		if err != nil {
			fmt.Println("err:", err)
			continue
		}
		orders = append(orders, *order)
	}
	return orders, nil
}

func (bn *Binance) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	return nil, ErrorUnsupported
}

func (bn *Binance) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	params := url.Values{}
	pair = bn.adaptCurrencyPair(pair)
	params.Set("symbol", pair.ToSymbol(""))
	//params.Set("limit", 1000)

	bn.buildParamsSigned(&params)
	path := bn.apiV3 + MY_TRADES + params.Encode()

	resp, err := HttpGet3(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return nil, err
	}

	var trades []Trade
	for _, v := range resp {
		m := v.(map[string]interface{})
		ty := SELL
		if m["isBuyer"].(bool) {
			ty = BUY
		}
		trades = append(trades, Trade{
			Tid:    ToInt64(m["id"]),
			Side:   ty,
			Amount: ToFloat64(m["qty"]),
			Price:  ToFloat64(m["price"]),
			TS:     ToInt64(m["time"]),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		})
	}

	return trades, nil
}

func (bn *Binance) GetAccount() (*Account, error) {
	params := url.Values{}
	bn.buildParamsSigned(&params)
	path := bn.apiV3 + ACCOUNT_URI + params.Encode()
	respmap, err := HttpGet2(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	if _, isok := respmap["code"]; isok == true {
		return nil, errors.New(respmap["msg"].(string))
	}
	acc := Account{}
	acc.Exchange = bn.GetExchangeName()
	acc.SubAccounts = make(map[Currency]SubAccount)

	balances := respmap["balances"].([]interface{})
	for _, v := range balances {
		vv := v.(map[string]interface{})
		currency := NewCurrency(vv["asset"].(string))
		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(vv["free"]),
			FrozenAmount: ToFloat64(vv["locked"]),
		}
	}

	return &acc, nil
}

func (bn *Binance) parseOrder(respmap map[string]interface{}, pair CurrencyPair) (*Order, error) {
	status := respmap["status"].(string)
	side := respmap["side"].(string)
	orderType := respmap["type"].(string)

	ord := Order{}
	ord.OrderID = fmt.Sprint(int64(respmap["orderId"].(float64)))
	ord.Price = ToFloat64(respmap["price"].(string))
	ord.Amount = ToFloat64(respmap["origQty"].(string))
	ord.DealAmount = ToFloat64(respmap["executedQty"])
	cummulativeQuoteQty := ToFloat64(respmap["cummulativeQuoteQty"])
	if cummulativeQuoteQty > 0 && ord.DealAmount > 0 {
		ord.AvgPrice = cummulativeQuoteQty / ord.DealAmount
	}
	// todo: no fee from binance, set it from setting?
	// ord.Fee
	ord.TS = ToInt64(respmap["time"])
	switch status {
	case "NEW":
		ord.Status = ORDER_UNFINISH
	case "PARTIALLY_FILLED":
		ord.Status = ORDER_PART_FINISH
	case "FILLED":
		ord.Status = ORDER_FINISH
	case "CANCELED":
		ord.Status = ORDER_CANCEL
	case "PENDING_CANCEL":
		ord.Status = ORDER_CANCEL_ING
	case "REJECTED":
		ord.Status = ORDER_REJECT
	case "EXPIRED":
		ord.Status = ORDER_FAIL
	}
	ord.Market = pair
	ord.Symbol = pair.ToLowerSymbol("/")
	if side == "SELL" {
		if orderType == "MARKET" {
			ord.Side = SELL_MARKET
		} else {
			ord.Side = SELL
		}
	} else {
		if orderType == "MARKET" {
			ord.Side = BUY_MARKET
		} else {
			ord.Side = BUY
		}
	}

	return &ord, nil
}

func (ba *Binance) adaptCurrencyPair(pair CurrencyPair) CurrencyPair {
	//if pair.CurrencyA.Eq(BCH) || pair.CurrencyA.Eq(BCC) {
	//	return NewCurrencyPair(NewCurrency("BCHABC", ""), pair.CurrencyB).AdaptUsdToUsdt()
	//}
	//
	//if pair.CurrencyA.Symbol == "BSV" {
	//	return NewCurrencyPair(NewCurrency("BCHSV", ""), pair.CurrencyB).AdaptUsdToUsdt()
	//}
	//
	//return pair.AdaptUsdToUsdt()

	return pair
}

func (bn *Binance) getTradeSymbols() ([]TradeSymbol, error) {
	resp, err := HttpGet5(bn.httpClient, bn.apiV3+"exchangeInfo", nil)
	if err != nil {
		return nil, err
	}
	info := new(ExchangeInfo)
	err = json.Unmarshal(resp, info)
	if err != nil {
		return nil, err
	}

	return info.Symbols, nil
}

func (bn *Binance) GetTradeSymbols(pair CurrencyPair) (*TradeSymbol, error) {
	if len(bn.tradeSymbols) == 0 {
		var err error
		bn.tradeSymbols, err = bn.getTradeSymbols()
		if err != nil {
			return nil, err
		}
	}
	for k, v := range bn.tradeSymbols {
		if v.Symbol == pair.ToSymbol("") {
			return &bn.tradeSymbols[k], nil
		}
	}
	return nil, errors.New("symbol not found")
}

func (bn *Binance) setTimeOffset() error {
	if bn.timeoffset == 0 {
		respmap, err := HttpGet(bn.httpClient, bn.apiV3+SERVER_TIME_URL)
		if err != nil {
			return err
		}

		stime := int64(ToInt(respmap["serverTime"]))
		st := time.Unix(stime/1000, 1000000*(stime%1000))
		lt := time.Now()
		offset := st.Sub(lt).Nanoseconds()
		bn.timeoffset = int64(offset)
	}

	return nil
}

func (bn *Binance) placeOrder(amount, price string, pair CurrencyPair, orderType, orderSide string) (*Order, error) {
	pair = bn.adaptCurrencyPair(pair)
	path := bn.apiV3 + ORDER_URI
	params := url.Values{}
	params.Set("symbol", pair.ToSymbol(""))
	params.Set("side", orderSide)
	params.Set("type", orderType)
	params.Set("newOrderRespType", "RESULT")

	switch orderType {
	case "LIMIT":
		params.Set("timeInForce", "GTC")
		params.Set("price", price)
		params.Set("quantity", amount)
	case "MARKET":
		if orderSide == "SELL" {
			params.Set("quoteOrderQty", amount)
		} else {
			params.Set("quantity", amount)
		}
	}

	bn.buildParamsSigned(&params)

	resp, err := HttpPostForm2(bn.httpClient, path, params, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		log.Println(string(resp))
		return nil, err
	}

	orderId := ToInt64(respmap["orderId"])
	if orderId <= 0 {
		return nil, errors.New(string(resp))
	}

	side := BUY
	if orderSide == "SELL" {
		if orderType == "MARKET" {
			side = SELL_MARKET
		} else {
			side = SELL
		}
	} else {
		if orderType == "MARKET" {
			side = BUY_MARKET
		} else {
			side = BUY
		}
	}

	dealAmount := ToFloat64(respmap["executedQty"])
	cummulativeQuoteQty := ToFloat64(respmap["cummulativeQuoteQty"])
	avgPrice := 0.0
	if cummulativeQuoteQty > 0 && dealAmount > 0 {
		avgPrice = cummulativeQuoteQty / dealAmount
	}

	orderStatus, _ := respmap["status"].(string)
	/*
		NEW 新建订单
		PARTIALLY_FILLED 部分成交
		FILLED 全部成交
		CANCELED 已撤销
		PENDING_CANCEL 撤销中（目前并未使用）
		REJECTED 订单被拒绝
		EXPIRED 订单过期（根据timeInForce参数规则）
	*/
	status := ORDER_UNFINISH
	switch orderStatus {
	case "NEW":
		status = ORDER_UNFINISH
	case "PARTIALLY_FILLED":
		status = ORDER_PART_FINISH
	case "FILLED":
		status = ORDER_FINISH
	case "CANCELED":
		status = ORDER_CANCEL
	case "PENDING_CANCEL":
		status = ORDER_CANCEL_ING
	case "REJECTED":
		status = ORDER_REJECT
	case "EXPIRED":
		status = ORDER_FAIL
	}

	return &Order{
		OrderID:    fmt.Sprint(orderId),
		Price:      ToFloat64(price),
		Amount:     ToFloat64(amount),
		AvgPrice:   avgPrice,
		DealAmount: dealAmount,
		TS:         ToInt64(respmap["transactTime"]),
		Status:     status,
		Market:     pair,
		Symbol:     pair.ToLowerSymbol("/"),
		Side:       side,
	}, nil
}

func adaptCurrencyPair(pair CurrencyPair) CurrencyPair {
	if pair.Stock.Equal(BCC) {
		return NewCurrencyPair(NewCurrency("BCH"), pair.Money).AdaptUsdToUsdt()
	}

	return pair.AdaptUsdToUsdt()
}
