package huobi

import (
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	jsoniter "github.com/json-iterator/go"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var HBPOINT = NewCurrency("HBPOINT")

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]string{
	KLINE_M1:  "1min",
	KLINE_M5:  "5min",
	KLINE_M15: "15min",
	KLINE_M30: "30min",
	KLINE_H1:  "60min",
	// KLINE_H4
	KLINE_DAY:   "1day",
	KLINE_WEEK:  "1week",
	KLINE_MONTH: "1mon",
}

const (
	HB_POINT_ACCOUNT = "point"
	HB_SPOT_ACCOUNT  = "spot"
)

type AccountInfo struct {
	Id    string
	Type  string
	State string
}

type HuoBiPro struct {
	httpClient *http.Client
	baseUrl    string
	accountId  string
	accessKey  string
	secretKey  string
	//ECDSAPrivateKey string
}

type HuoBiProSymbol struct {
	BaseCurrency    string
	QuoteCurrency   string
	PricePrecision  float64
	AmountPrecision float64
	SymbolPartition string
	Symbol          string
}

func NewHuoBiPro(client *http.Client, apikey, secretkey, accountId string) *HuoBiPro {
	hbpro := new(HuoBiPro)
	hbpro.baseUrl = "https://api.huobi.pro"
	hbpro.httpClient = client
	hbpro.accessKey = apikey
	hbpro.secretKey = secretkey
	hbpro.accountId = accountId
	return hbpro
}

/**
 * spot
 */
func NewSpotAPI(client *http.Client, apikey, secretkey string) SpotAPI {
	hb := NewHuoBiPro(client, apikey, secretkey, "")
	return hb
}

func (hbpro *HuoBiPro) updateAccountID() {
	if len(hbpro.accountId) == 0 {
		accinfo, err := hbpro.GetAccountInfo(HB_SPOT_ACCOUNT)
		if err != nil {
			hbpro.accountId = ""
		} else {
			hbpro.accountId = accinfo.Id
		}
	}
}

func (hbpro *HuoBiPro) GetExchangeName() string {
	return HUOBI
}

func (hbpro *HuoBiPro) SetURL(exurl string) {
	hbpro.baseUrl = exurl
}

func (hbpro *HuoBiPro) GetURL() string {
	return hbpro.baseUrl
}

/*
"symbol": "btcusdt",
"makerFeeRate":"0.002",
"takerFeeRate":"0.002",
"actualMakerRate": "0.002",
"actualTakerRate":"0.002
*/
type TradeFee struct {
	Symbol          string  `json:"symbol"`
	ActualMakerRate float64 `json:"actualMakerRate,string"` // 挂单手续费
	ActualTakerRate float64 `json:"actualTakerRate,string"` // 吃单手续费
}

func (hbpro *HuoBiPro) GetTradeFee(symbols string) (tf *TradeFee, err error) {
	path := "/v2/reference/transact-fee-rate"
	params := &url.Values{}
	params.Set("symbols", symbols)
	hbpro.buildPostForm("GET", path, params)
	respmap, err := HttpGet(hbpro.httpClient, hbpro.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}

	// a failure example:
	// {"ts":1579423405172,"status":"error","err-code":"invalid-parameter","err-msg":"invalid symbol"}
	if respmap["code"].(float64) != 200 {
		return nil, errors.New(respmap["message"].(string))
	}

	// a success example:
	// {"status":"ok","ch":"market.btcusdt.detail.merged","ts":1579423189207,"tick":{"amount":36731.43723323591,"open":8843.22,"close":9078.0,"high":9198.96,"id":208132871759,"count":295788,"low":8843.22,"version":208132871759,"ask":[9078.0,4.109982687155761],"vol":3.310314436109146E8,"bid":[9077.81,1.06]}}
	dataArr, ok := respmap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	tf = &TradeFee{}
	for _, v := range dataArr {
		obj := v.(map[string]interface{})
		tf.ActualMakerRate = ToFloat64(obj["actualMakerRate"])
		tf.ActualTakerRate = ToFloat64(obj["actualTakerRate"])
		break
	}

	return tf, nil
}

func (hbpro *HuoBiPro) GetTradeFeeMap() (tfmap map[string]TradeFee, err error) {
	path := "/v2/reference/transact-fee-rate"
	params := &url.Values{}
	//params.Set("symbols", symbols) // 火币目前只支持一次最多查询10个交易对
	hbpro.buildPostForm("GET", path, params)
	respmap, err := HttpGet(hbpro.httpClient, hbpro.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}

	// a failure example:
	// {"ts":1579423405172,"status":"error","err-code":"invalid-parameter","err-msg":"invalid symbol"}
	if respmap["code"].(float64) != 200 {
		return nil, errors.New(respmap["message"].(string))
	}

	// a success example:
	// {"status":"ok","ch":"market.btcusdt.detail.merged","ts":1579423189207,"tick":{"amount":36731.43723323591,"open":8843.22,"close":9078.0,"high":9198.96,"id":208132871759,"count":295788,"low":8843.22,"version":208132871759,"ask":[9078.0,4.109982687155761],"vol":3.310314436109146E8,"bid":[9077.81,1.06]}}
	dataArr, ok := respmap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	tfmap = make(map[string]TradeFee)
	for _, v := range dataArr {
		obj := v.(map[string]interface{})
		symbol := ToString(obj["symbol"])
		tfmap[symbol] = TradeFee{
			Symbol:          symbol,
			ActualMakerRate: ToFloat64(obj["actualMakerRate"]),
			ActualTakerRate: ToFloat64(obj["actualTakerRate"]),
		}
	}

	return tfmap, nil
}

func (hbpro *HuoBiPro) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	url := hbpro.baseUrl + "/v1/common/symbols"
	respmap, err := HttpGet(hbpro.httpClient, url)
	if err != nil {
		return nil, err
	}

	// a failure example:
	// {"ts":1579423405172,"status":"error","err-code":"invalid-parameter","err-msg":"invalid symbol"}
	if respmap["status"].(string) == "error" {
		return nil, errors.New(respmap["err-msg"].(string))
	}

	// a success example:
	// {"status":"ok","ch":"market.btcusdt.detail.merged","ts":1579423189207,"tick":{"amount":36731.43723323591,"open":8843.22,"close":9078.0,"high":9198.96,"id":208132871759,"count":295788,"low":8843.22,"version":208132871759,"ask":[9078.0,4.109982687155761],"vol":3.310314436109146E8,"bid":[9077.81,1.06]}}
	dataArr, ok := respmap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	// TODO FIXME: 火币的费率查询，目前最多支持10个币种对。这里用了一个tricky，查一个币种费率，所有币种都用一个费率。
	tf, err := hbpro.GetTradeFee("btcusdt")
	if err != nil {
		tf = &TradeFee{
			ActualMakerRate: 0.002,
			ActualTakerRate: 0.002,
		}
	}

	ssm := make(map[string]SymbolSetting)
	for _, v := range dataArr {
		obj := v.(map[string]interface{})
		base := strings.ToUpper(ToString(obj["base-currency"]))
		quote := strings.ToUpper(ToString(obj["quote-currency"]))
		symbol := base + "/" + quote
		ssm[symbol] = SymbolSetting{
			Symbol:      symbol,
			Base:        base,
			Quote:       quote,
			MinSize:     math.Pow10(-ToInt(obj["amount-precision"])),
			MinPrice:    math.Pow10(-ToInt(obj["price-precision"])),
			MinNotional: ToFloat64(obj["min-order-value"]),
			MakerFee:    tf.ActualMakerRate,
			TakerFee:    tf.ActualTakerRate,
		}
	}

	return ssm, nil
}

func (hbpro *HuoBiPro) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	url := hbpro.baseUrl + "/v2/reference/currencies?currency=" + currency.LowerSymbol()
	respmap, err := HttpGet(hbpro.httpClient, url)
	if err != nil {
		return CurrencyStatus{}, err
	}

	if respmap["code"].(float64) != 200 {
		return CurrencyStatus{}, errors.New(respmap["message"].(string))
	}

	dataArr, ok := respmap["data"].([]interface{})
	if !ok {
		return CurrencyStatus{}, errors.New("data assert error")
	}

	for _, v := range dataArr {
		obj := v.(map[string]interface{})
		symbol := ToString(obj["currency"])
		if strings.ToUpper(symbol) == currency.Symbol() {
			chains, ok := obj["chains"].([]interface{})
			if !ok {
				return CurrencyStatus{}, errors.New("chains assert error")
			}

			cs := CurrencyStatus{}
			for _, c := range chains {
				info := c.(map[string]interface{})
				ds := ToString(info["depositStatus"])
				ws := ToString(info["withdrawStatus"])

				cs.Deposit = cs.Deposit || ds == "allowed"
				cs.Withdraw = cs.Withdraw || ws == "allowed"
			}

			return cs, nil
		}
	}

	return CurrencyStatus{}, errors.New("Asset not found")
}

func (hbpro *HuoBiPro) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	url := hbpro.baseUrl + "/v2/reference/currencies"
	respmap, err := HttpGet(hbpro.httpClient, url)
	if err != nil {
		return nil, err
	}

	if respmap["code"].(float64) != 200 {
		return nil, errors.New(respmap["message"].(string))
	}

	dataArr, ok := respmap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	all = make(map[string]CurrencyStatus)
	for _, v := range dataArr {
		obj := v.(map[string]interface{})
		chains, ok := obj["chains"].([]interface{})
		if !ok {
			continue
		}

		cs := CurrencyStatus{}
		for _, c := range chains {
			info := c.(map[string]interface{})
			ds := ToString(info["depositStatus"])
			ws := ToString(info["withdrawStatus"])

			cs.Deposit = cs.Deposit || ds == "allowed"
			cs.Withdraw = cs.Withdraw || ws == "allowed"
		}

		all[strings.ToUpper(ToString(obj["currency"]))] = cs
	}

	return all, nil
}

func (hbpro *HuoBiPro) GetTicker(pair CurrencyPair) (*Ticker, error) {
	url := hbpro.baseUrl + "/market/detail/merged?symbol=" + pair.ToLowerSymbol("")
	respmap, err := HttpGet(hbpro.httpClient, url)
	if err != nil {
		return nil, err
	}

	// a failure example:
	// {"ts":1579423405172,"status":"error","err-code":"invalid-parameter","err-msg":"invalid symbol"}
	if respmap["status"].(string) == "error" {
		return nil, errors.New(respmap["err-msg"].(string))
	}

	// a success example:
	// {"status":"ok","ch":"market.btcusdt.detail.merged","ts":1579423189207,"tick":{"amount":36731.43723323591,"open":8843.22,"close":9078.0,"high":9198.96,"id":208132871759,"count":295788,"low":8843.22,"version":208132871759,"ask":[9078.0,4.109982687155761],"vol":3.310314436109146E8,"bid":[9077.81,1.06]}}
	tickmap, ok := respmap["tick"].(map[string]interface{})
	if !ok {
		return nil, errors.New("tick assert error")
	}

	ticker := new(Ticker)
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	ticker.Open = ToFloat64(tickmap["open"])
	ticker.Last = ToFloat64(tickmap["close"])
	ticker.High = ToFloat64(tickmap["high"])
	ticker.Low = ToFloat64(tickmap["low"])
	ticker.Vol = ToFloat64(tickmap["vol"])
	bid, isOk := tickmap["bid"].([]interface{})
	if isOk != true || len(bid) == 0 {
		return nil, errors.New("no bid")
	}
	ask, isOk := tickmap["ask"].([]interface{})
	if isOk != true || len(ask) == 0 {
		return nil, errors.New("no ask")
	}
	ticker.Buy = ToFloat64(bid[0])
	ticker.Sell = ToFloat64(ask[0])
	ticker.TS = ToInt64(respmap["ts"])

	return ticker, nil
}

func (hbpro *HuoBiPro) GetAllTicker() ([]Ticker, error) {
	url := hbpro.baseUrl + "/market/tickers"
	respmap, err := HttpGet(hbpro.httpClient, url)
	if err != nil {
		return nil, err
	}

	// a failure example:
	// {"ts":1579423405172,"status":"error","err-code":"invalid-parameter","err-msg":"invalid symbol"}
	if respmap["status"].(string) == "error" {
		return nil, errors.New(respmap["err-msg"].(string))
	}

	// a success example:
	// {"status":"ok","ch":"market.btcusdt.detail.merged","ts":1579423189207,"tick":{"amount":36731.43723323591,"open":8843.22,"close":9078.0,"high":9198.96,"id":208132871759,"count":295788,"low":8843.22,"version":208132871759,"ask":[9078.0,4.109982687155761],"vol":3.310314436109146E8,"bid":[9077.81,1.06]}}
	data, ok := respmap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	ts := ToInt64(respmap["ts"])

	tickers := make([]Ticker, 0, len(data))
	for _, v := range data {
		tickmap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		base, quote := getSymbol(ToString(tickmap["symbol"]))
		if len(base) == 0 || len(quote) == 0 {
			continue
		}
		ticker := Ticker{}
		ticker.Symbol = base + "/" + quote
		ticker.Market = NewCurrencyPairFromString(ticker.Symbol)
		ticker.Open = ToFloat64(tickmap["open"])
		ticker.Last = ToFloat64(tickmap["close"])
		ticker.High = ToFloat64(tickmap["high"])
		ticker.Low = ToFloat64(tickmap["low"])
		ticker.Vol = ToFloat64(tickmap["vol"])
		ticker.Buy = ToFloat64(tickmap["bid"])
		ticker.Sell = ToFloat64(tickmap["ask"])
		ticker.TS = ts
		tickers = append(tickers, ticker)
	}

	return tickers, nil
}

func getSymbol(market string) (base, quote string) {
	fiatCoin := []string{"usdt", "husd", "btc", "eth", "ht", "trx"}

	for _, v := range fiatCoin {
		if strings.HasSuffix(market, v) {
			p := strings.LastIndex(market, v)
			return market[0:p], v
		}
	}

	return "", ""
}

/*
取值	说明
step0	无聚合
step1	聚合度为报价精度*10
step2	聚合度为报价精度*100
step3	聚合度为报价精度*1000
step4	聚合度为报价精度*10000
step5	聚合度为报价精度*100000
*/
func (hbpro *HuoBiPro) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	url := hbpro.baseUrl + "/market/depth?symbol=" + strings.ToLower(pair.ToSymbol(""))
	//if size != 0 {
	//	url += fmt.Sprintf("&depth=%v", size)
	//}
	url += fmt.Sprintf("&type=step%v", step)
	respmap, err := HttpGet(hbpro.httpClient, url)
	if err != nil {
		return nil, err
	}

	if "ok" != respmap["status"].(string) {
		return nil, errors.New(respmap["err-msg"].(string))
	}

	tick, _ := respmap["tick"].(map[string]interface{})

	dep := hbpro.parseDepthData(tick)
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")
	dep.TS = ToInt64(respmap["ts"])

	return dep, nil
}

func (hbpro *HuoBiPro) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	var (
		trades []Trade
		ret    struct {
			Status string
			ErrMsg string `json:"err-msg"`
			Data   []struct {
				Ts   int64
				Data []struct {
					//Id        big.Int
					TradeID   int64 `json:"trade-id"`
					Amount    float64
					Price     float64
					Direction string
					Ts        int64
				}
			}
		}
	)

	url := fmt.Sprintf(hbpro.baseUrl+"/market/history/trade?size=%v&symbol=%v", size, pair.ToLowerSymbol(""))
	err := HttpGet4(hbpro.httpClient, url, map[string]string{}, &ret)
	if err != nil {
		return nil, err
	}

	if ret.Status != "ok" {
		return nil, errors.New(ret.ErrMsg)
	}

	for _, d := range ret.Data {
		for _, t := range d.Data {
			trades = append(trades, Trade{
				//Tid:    parseHuobiTradeID(t.Id),
				Tid:    t.TradeID,
				Market: pair,
				Symbol: pair.ToLowerSymbol("/"),
				Amount: t.Amount,
				Price:  t.Price,
				Side:   AdaptTradeSide(t.Direction),
				TS:     t.Ts})
		}
	}

	return trades, nil
}

//倒序
func (hbpro *HuoBiPro) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", hbpro.GetExchangeName(), period)
	}
	url := hbpro.baseUrl + "/market/history/kline?period=%s&size=%d&symbol=%s"
	symbol := pair.ToLowerSymbol("")
	ret, err := HttpGet(hbpro.httpClient, fmt.Sprintf(url, periodS, size, symbol))
	if err != nil {
		return nil, err
	}

	data, ok := ret["data"].([]interface{})
	if !ok {
		return nil, errors.New("response format error")
	}

	var klines []Kline
	for i := len(data) - 1; i >= 0; i-- {
		item := data[i].(map[string]interface{})
		klines = append(klines, Kline{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Open:   ToFloat64(item["open"]),
			Close:  ToFloat64(item["close"]),
			High:   ToFloat64(item["high"]),
			Low:    ToFloat64(item["low"]),
			Vol:    ToFloat64(item["vol"]),
			TS:     ToInt64(item["id"])})
	}
	return klines, nil
}

func (hbpro *HuoBiPro) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := hbpro.placeOrder(amount, price, pair, "buy-limit")
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

func (hbpro *HuoBiPro) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := hbpro.placeOrder(amount, price, pair, "sell-limit")
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

func (hbpro *HuoBiPro) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := hbpro.placeOrder(amount, "", pair, "buy-market")
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

func (hbpro *HuoBiPro) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := hbpro.placeOrder(amount, "", pair, "sell-market")
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

func (hbpro *HuoBiPro) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	path := fmt.Sprintf("/v1/order/orders/%s/submitcancel", orderId)
	params := url.Values{}
	hbpro.buildPostForm("POST", path, &params)
	resp, err := HttpPostForm3(hbpro.httpClient, hbpro.baseUrl+path+"?"+params.Encode(), hbpro.toJson(params),
		map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn"})
	if err != nil {
		return false, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return false, err
	}

	if respmap["status"].(string) != "ok" {
		return false, errors.New(string(resp))
	}

	return true, nil
}

func (hbpro *HuoBiPro) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	path := "/v1/order/orders/" + orderId
	params := url.Values{}
	hbpro.buildPostForm("GET", path, &params)
	respmap, err := HttpGet(hbpro.httpClient, hbpro.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].(map[string]interface{})
	order := hbpro.parseOrder(datamap)
	order.Market = pair
	order.Symbol = pair.ToLowerSymbol("/")
	return &order, nil
}

func (hbpro *HuoBiPro) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	return hbpro.getOrders(queryOrdersParams{
		pair:   pair,
		states: "pre-submitted,submitted,partial-filled",
		size:   500,
		//direct:""
	})
}

func (hbpro *HuoBiPro) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	return hbpro.getOrders(queryOrdersParams{
		pair:   pair,
		states: "filled,canceled,partial-canceled",
		size:   200,
		//direct:""
	})
}

func (hbpro *HuoBiPro) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	path := "/v1/order/orders/" + orderId + "/matchresults"
	params := url.Values{}
	hbpro.buildPostForm("GET", path, &params)
	respmap, err := HttpGet(hbpro.httpClient, hbpro.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].([]interface{})
	deals := make([]OrderDeal, 0, len(datamap))
	for _, v := range datamap {
		obj := v.(map[string]interface{})
		deal := OrderDeal{
			OrderID:      orderId,
			DealID:       fmt.Sprint(ToInt64(obj["id"])),
			TS:           ToInt64(obj["created-at"]),
			Price:        ToFloat64(obj["price"]),
			FilledAmount: ToFloat64(obj["filled-amount"]),
			Market:       pair,
			Symbol:       pair.ToLowerSymbol("/"),
		}

		deal.FilledCashAmount = deal.Price * deal.FilledAmount
		t := ToString(obj["type"])
		switch t {
		case "buy-limit":
			deal.Side = BUY
		case "sell-limit":
			deal.Side = SELL
		case "buy-market":
			deal.Side = BUY_MARKET
		case "sell-market":
			deal.Side = SELL_MARKET
		}
		deals = append(deals, deal)
	}

	return deals, nil
}

func (hbpro *HuoBiPro) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	return nil, ErrorUnsupported
}

func (hbpro *HuoBiPro) GetAccount() (*Account, error) {
	hbpro.updateAccountID()
	path := fmt.Sprintf("/v1/account/accounts/%s/balance", hbpro.accountId)
	params := &url.Values{}
	params.Set("accountId-id", hbpro.accountId)
	hbpro.buildPostForm("GET", path, params)

	urlStr := hbpro.baseUrl + path + "?" + params.Encode()
	//println(urlStr)
	respmap, err := HttpGet(hbpro.httpClient, urlStr)

	if err != nil {
		return nil, err
	}

	//log.Println(respmap)

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].(map[string]interface{})
	if datamap["state"].(string) != "working" {
		return nil, errors.New(datamap["state"].(string))
	}

	list := datamap["list"].([]interface{})
	acc := new(Account)
	acc.SubAccounts = make(map[Currency]SubAccount, 6)
	acc.Exchange = hbpro.GetExchangeName()

	subAccMap := make(map[Currency]*SubAccount)

	for _, v := range list {
		balancemap := v.(map[string]interface{})
		currencySymbol := balancemap["currency"].(string)
		currency := NewCurrency(currencySymbol)
		typeStr := balancemap["type"].(string)
		balance := ToFloat64(balancemap["balance"])
		if subAccMap[currency] == nil {
			subAccMap[currency] = new(SubAccount)
		}
		subAccMap[currency].Currency = currency
		switch typeStr {
		case "trade":
			subAccMap[currency].Amount = balance
		case "frozen":
			subAccMap[currency].FrozenAmount = balance
		}
	}

	for k, v := range subAccMap {
		acc.SubAccounts[k] = *v
	}

	return acc, nil
}

func (hbpro *HuoBiPro) placeOrder(amount, price string, pair CurrencyPair, orderType string) (string, error) {
	hbpro.updateAccountID()
	path := "/v1/order/orders/place"
	params := url.Values{}
	params.Set("account-id", hbpro.accountId)
	params.Set("amount", amount)
	params.Set("symbol", strings.ToLower(pair.ToSymbol("")))
	params.Set("type", orderType)

	switch orderType {
	case "buy-limit", "sell-limit":
		params.Set("price", price)
	}

	hbpro.buildPostForm("POST", path, &params)

	resp, err := HttpPostForm3(hbpro.httpClient, hbpro.baseUrl+path+"?"+params.Encode(), hbpro.toJson(params),
		map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn"})
	if err != nil {
		return "", err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return "", err
	}

	if respmap["status"].(string) != "ok" {
		return "", errors.New(respmap["err-code"].(string))
	}

	return respmap["data"].(string), nil
}

func (hbpro *HuoBiPro) parseOrder(ordmap map[string]interface{}) Order {
	ord := Order{
		OrderID:    fmt.Sprint(ToInt(ordmap["id"])),
		Amount:     ToFloat64(ordmap["amount"]),
		Price:      ToFloat64(ordmap["price"]),
		DealAmount: ToFloat64(ordmap["field-amount"]),
		Fee:        ToFloat64(ordmap["field-fees"]),
		TS:         ToInt64(ordmap["created-at"]),
	}

	state := ordmap["state"].(string)
	switch state {
	case "submitted", "pre-submitted":
		ord.Status = ORDER_UNFINISH
	case "filled":
		ord.Status = ORDER_FINISH
	case "partial-filled":
		ord.Status = ORDER_PART_FINISH
	case "canceled", "partial-canceled":
		ord.Status = ORDER_CANCEL
	default:
		ord.Status = ORDER_UNFINISH
	}

	if ord.DealAmount > 0.0 {
		ord.AvgPrice = ToFloat64(ordmap["field-cash-amount"]) / ord.DealAmount
	}

	typeS := ordmap["type"].(string)
	switch typeS {
	case "buy-limit":
		ord.Side = BUY
	case "buy-market":
		ord.Side = BUY_MARKET
		ord.Price = ord.AvgPrice
		ord.Amount = ord.DealAmount
	case "sell-limit":
		ord.Side = SELL
	case "sell-market":
		ord.Side = SELL_MARKET
		ord.Price = ord.AvgPrice
		ord.Amount = ord.DealAmount
	}
	return ord
}

type queryOrdersParams struct {
	types,
	startDate,
	endDate,
	states,
	from,
	direct string
	size int
	pair CurrencyPair
}

func (hbpro *HuoBiPro) getOrders(queryparams queryOrdersParams) ([]Order, error) {
	path := "/v1/order/orders"
	params := url.Values{}
	params.Set("symbol", strings.ToLower(queryparams.pair.ToSymbol("")))
	params.Set("states", queryparams.states)

	if queryparams.direct != "" {
		params.Set("direct", queryparams.direct)
	}

	if queryparams.size > 0 {
		params.Set("size", fmt.Sprint(queryparams.size))
	}

	hbpro.buildPostForm("GET", path, &params)
	respmap, err := HttpGet(hbpro.httpClient, fmt.Sprintf("%s%s?%s", hbpro.baseUrl, path, params.Encode()))
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].([]interface{})
	var orders []Order
	for _, v := range datamap {
		ordmap := v.(map[string]interface{})
		ord := hbpro.parseOrder(ordmap)
		ord.Market = queryparams.pair
		ord.Symbol = queryparams.pair.ToLowerSymbol("/")
		orders = append(orders, ord)
	}

	return orders, nil
}

func (hbpro *HuoBiPro) buildPostForm(reqMethod, path string, postForm *url.Values) error {
	postForm.Set("AccessKeyId", hbpro.accessKey)
	postForm.Set("SignatureMethod", "HmacSHA256")
	postForm.Set("SignatureVersion", "2")
	postForm.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05"))
	domain := strings.Replace(hbpro.baseUrl, "https://", "", len(hbpro.baseUrl))
	payload := fmt.Sprintf("%s\n%s\n%s\n%s", reqMethod, domain, path, postForm.Encode())
	sign, _ := GetParamHmacSHA256Base64Sign(hbpro.secretKey, payload)
	postForm.Set("Signature", sign)

	/**
	p, _ := pem.Decode([]byte(hbpro.ECDSAPrivateKey))
	pri, _ := secp256k1_go.PrivKeyFromBytes(secp256k1_go.S256(), p.Bytes)
	signer, _ := pri.Sign([]byte(sign))
	signAsn, _ := asn1.Marshal(signer)
	priSign := base64.StdEncoding.EncodeToString(signAsn)
	postForm.Set("PrivateSignature", priSign)
	*/

	return nil
}

func (hbpro *HuoBiPro) toJson(params url.Values) string {
	parammap := make(map[string]string)
	for k, v := range params {
		parammap[k] = v[0]
	}
	jsonData, _ := json.Marshal(parammap)
	return string(jsonData)
}

func (hbpro *HuoBiPro) parseDepthData(tick map[string]interface{}) *Depth {
	bids, _ := tick["bids"].([]interface{})
	asks, _ := tick["asks"].([]interface{})

	depth := new(Depth)
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

func (hbpro *HuoBiPro) GetCurrenciesList() ([]string, error) {
	url := hbpro.baseUrl + "/v1/common/currencys"

	ret, err := HttpGet(hbpro.httpClient, url)
	if err != nil {
		return nil, err
	}

	_, ok := ret["data"].([]interface{})
	if !ok {
		return nil, errors.New("response format error")
	}
	//fmt.Println(data)
	return nil, nil
}

func (hbpro *HuoBiPro) GetCurrenciesPrecision() ([]HuoBiProSymbol, error) {
	url := hbpro.baseUrl + "/v1/common/symbols"

	ret, err := HttpGet(hbpro.httpClient, url)
	if err != nil {
		return nil, err
	}

	data, ok := ret["data"].([]interface{})
	if !ok {
		return nil, errors.New("response format error")
	}
	var Symbols []HuoBiProSymbol
	for _, v := range data {
		_sym := v.(map[string]interface{})
		var sym HuoBiProSymbol
		sym.BaseCurrency = _sym["base-currency"].(string)
		sym.QuoteCurrency = _sym["quote-currency"].(string)
		sym.PricePrecision = _sym["price-precision"].(float64)
		sym.AmountPrecision = _sym["amount-precision"].(float64)
		sym.SymbolPartition = _sym["symbol-partition"].(string)
		sym.Symbol = _sym["symbol"].(string)
		Symbols = append(Symbols, sym)
	}
	//fmt.Println(Symbols)
	return Symbols, nil
}

func (hbpro *HuoBiPro) GetAccountInfo(acc string) (AccountInfo, error) {
	path := "/v1/account/accounts"
	params := &url.Values{}
	hbpro.buildPostForm("GET", path, params)

	//log.Println(hbpro.baseUrl + path + "?" + params.Encode())

	respmap, err := HttpGet(hbpro.httpClient, hbpro.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return AccountInfo{}, err
	}
	//log.Println(respmap)
	if respmap["status"].(string) != "ok" {
		return AccountInfo{}, errors.New(respmap["err-code"].(string))
	}

	var info AccountInfo

	data := respmap["data"].([]interface{})
	for _, v := range data {
		iddata := v.(map[string]interface{})
		if iddata["type"].(string) == acc {
			info.Id = fmt.Sprintf("%.0f", iddata["id"])
			info.Type = acc
			info.State = iddata["state"].(string)
			break
		}
	}
	//log.Println(respmap)
	return info, nil
}

func parseHuobiTradeID(id big.Int) int64 {
	//fix huobi   Weird rules of tid
	//火币交易ID规定固定23位, 导致超出int64范围，每个交易对有不同的固定填充前缀
	//实际交易ID远远没有到23位数字。
	tid := ToInt64(strings.TrimPrefix(id.String()[4:], "0"))
	if tid == 0 {
		tid = ToInt64(strings.TrimPrefix(id.String()[5:], "0"))
	}
	return tid
}
