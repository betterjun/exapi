package zb

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

const (
	MARKET_URL = "http://api.zb.live/data/v1/"
	TRADE_URL  = "https://trade.zb.live/api/"
)

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

type Zb struct {
	httpClient *http.Client
	accessKey,
	secretKey string
}

func NewSpotAPI(client *http.Client, apiKey, secretKey string) SpotAPI {
	zb := &Zb{
		accessKey:  apiKey,
		secretKey:  secretKey,
		httpClient: client}
	return zb
}

func (zb *Zb) GetExchangeName() string {
	return ZB
}

func (zb *Zb) SetURL(exurl string) {
	// 不支持
}

func (zb *Zb) GetURL() string {
	return TRADE_URL
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

func (zb *Zb) GetTradeFee() (tf *TradeFee, err error) {
	tf = &TradeFee{
		Maker: 0.002,
		Taker: 0.002,
	}
	return tf, nil
}

func (zb *Zb) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	resp, err := HttpGet(zb.httpClient, MARKET_URL+"markets")
	if err != nil {
		return nil, err
	}

	tf, err := zb.GetTradeFee()
	if err != nil {
		return nil, err
	}
	ssm := make(map[string]SymbolSetting)
	for k, v := range resp {
		symbol := strings.ToLower(strings.Replace(k, "_", "", -1))
		currencies := strings.Split(k, "_")

		obj, _ := v.(map[string]interface{})
		ssm[symbol] = SymbolSetting{
			Symbol: symbol,
			Base:   strings.ToLower(currencies[0]),
			Quote:  strings.ToLower(currencies[1]),
			// 用浮点数的pow会有精度损失，在后面取整数位数时会不准
			//MinSize:  math.Pow(0.1, ToFloat64(obj["amountScale"])),
			//MinPrice:math.Pow(0.1, ToFloat64(obj["priceScale"])),
			MinSize:  math.Pow10(-ToInt(obj["amountScale"])),
			MinPrice: math.Pow10(-ToInt(obj["priceScale"])),
			MakerFee: tf.Maker,
			TakerFee: tf.Taker,
		}
	}

	return ssm, nil
}

func (zb *Zb) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	all, err := zb.GetAllCurrencyStatus()
	if err != nil {
		return CurrencyStatus{
			Deposit:  false,
			Withdraw: false,
		}, err
	}

	return all[currency.Symbol()], nil
}

func (zb *Zb) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	// 注意：此地址是在中币的网页提币界面取到的，随时可能有变化
	resp, err := HttpGet(zb.httpClient, "https://vip.zb.com/api/web/common/V1_0_0/getCurrencyConfig")
	if err != nil {
		return nil, err
	}

	resMsg, ok := resp["resMsg"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("resMsg assert failed")
	}

	code := resMsg["code"].(float64)
	if code != 1000 {
		return nil, fmt.Errorf("resMsg failed, code %v", code)
	}

	datas, ok := resp["datas"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("datas assert failed")
	}

	all = make(map[string]CurrencyStatus)
	for k, v := range datas {
		obj, _ := v.(map[string]interface{})
		all[strings.ToUpper(k)] = CurrencyStatus{
			Deposit:  ToBool(obj["isPayIn"]),
			Withdraw: ToBool(obj["isPayOut"]),
		}
	}

	return all, nil
}

func (zb *Zb) GetTicker(pair CurrencyPair) (*Ticker, error) {
	symbol := pair.ToSymbol("_")
	resp, err := HttpGet(zb.httpClient, MARKET_URL+fmt.Sprintf("ticker?market=%s", symbol))
	if err != nil {
		return nil, err
	}

	// {"date":"1582816202835","ticker":{"high":"9043.8","vol":"90454.4745","last":"8884.85","low":"8530.25","buy":"8886.36","sell":"8886.44"}}
	tickermap := resp["ticker"].(map[string]interface{})

	ticker := new(Ticker)
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	//ticker.Open
	ticker.Last, _ = strconv.ParseFloat(tickermap["last"].(string), 64)
	ticker.High, _ = strconv.ParseFloat(tickermap["high"].(string), 64)
	ticker.Low, _ = strconv.ParseFloat(tickermap["low"].(string), 64)
	ticker.Vol, _ = strconv.ParseFloat(tickermap["vol"].(string), 64)
	ticker.Buy, _ = strconv.ParseFloat(tickermap["buy"].(string), 64)
	ticker.Sell, _ = strconv.ParseFloat(tickermap["sell"].(string), 64)
	ticker.TS, _ = strconv.ParseInt(resp["date"].(string), 10, 64)

	return ticker, nil
}

func (zb *Zb) GetAllTicker() ([]Ticker, error) {
	resp, err := HttpGet(zb.httpClient, MARKET_URL+"allTicker")
	if err != nil {
		return nil, err
	}

	tickers := make([]Ticker, 0)
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	for k, v := range resp {
		tickermap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		base, quote := getSymbol(k)
		if len(base) == 0 || len(quote) == 0 {
			continue
		}
		ticker := Ticker{}
		ticker.Symbol = base + "/" + quote
		ticker.Market = NewCurrencyPairFromString(ticker.Symbol)
		ticker.Last, _ = strconv.ParseFloat(tickermap["last"].(string), 64)
		ticker.High, _ = strconv.ParseFloat(tickermap["high"].(string), 64)
		ticker.Low, _ = strconv.ParseFloat(tickermap["low"].(string), 64)
		ticker.Vol, _ = strconv.ParseFloat(tickermap["vol"].(string), 64)
		ticker.Buy, _ = strconv.ParseFloat(tickermap["buy"].(string), 64)
		ticker.Sell, _ = strconv.ParseFloat(tickermap["sell"].(string), 64)
		ticker.TS = ts
		tickers = append(tickers, ticker)
	}

	return tickers, nil
}

func getSymbol(market string) (base, quote string) {
	fiatCoin := []string{"usdt", "qc", "btc"}

	for _, v := range fiatCoin {
		if strings.HasSuffix(market, v) {
			p := strings.LastIndex(market, v)
			return market[0:p], v
		}
	}

	return "", ""
}

func (zb *Zb) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	symbol := pair.ToSymbol("_")
	resp, err := HttpGet(zb.httpClient, MARKET_URL+fmt.Sprintf("depth?market=%s&size=%d", symbol, size))
	if err != nil {
		return nil, err
	}

	// {"asks":[[8891.68,0.0002],[8890.69,0.0010],[8890.0,0.3833]],"bids":[[8888.71,0.0007],[8888.34,0.0007],[8887.72,0.0004]],"timestamp":1582816568}
	asks, isok1 := resp["asks"].([]interface{})
	bids, isok2 := resp["bids"].([]interface{})

	if isok2 != true || isok1 != true {
		return nil, errors.New("no depth data!")
	}

	depth := new(Depth)
	depth.Market = pair
	depth.Symbol = pair.ToLowerSymbol("/")
	depth.TS = ToInt64(resp["timestamp"]) * 1000

	for _, e := range bids {
		var r DepthRecord
		ee := e.([]interface{})
		r.Price = ee[0].(float64)
		r.Amount = ee[1].(float64)

		depth.BidList = append(depth.BidList, r)
	}

	for _, e := range asks {
		var r DepthRecord
		ee := e.([]interface{})
		r.Price = ee[0].(float64)
		r.Amount = ee[1].(float64)

		depth.AskList = append(depth.AskList, r)
	}
	sort.Sort(depth.AskList)

	return depth, nil
}

func (zb *Zb) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	symbol := pair.ToSymbol("_")
	resp, err := HttpGet3(zb.httpClient, MARKET_URL+fmt.Sprintf("trades?market=%v", symbol), nil)
	if err != nil {
		return nil, err
	}

	/*
		[
		    {
		        "amount": 0.541,
		        "date": 1472711925,
		        "price": 81.87,
		        "tid": 16497097,
		        "trade_type": "ask",
		        "type": "sell"
		    }...
		]
	*/

	trades := make([]Trade, 0)
	for _, v := range resp {
		obj, _ := v.(map[string]interface{})
		t := Trade{
			Tid:    ToInt64(obj["tid"]),
			Side:   BUY,
			Amount: ToFloat64(obj["amount"]),
			Price:  ToFloat64(obj["price"]),
			TS:     ToInt64(obj["date"]) * 1000,
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

func (zb *Zb) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	symbol := pair.ToSymbol("_")
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", zb.GetExchangeName(), period)
	}
	resp, err := HttpGet(zb.httpClient, MARKET_URL+fmt.Sprintf("kline?market=%v&type=%v", symbol, periodS))
	if err != nil {
		return nil, err
	}

	/*
			data : K线内容
		moneyType : 买入货币
		symbol : 卖出货币
		data : 内容说明
		[
		1417536000000, 时间戳
		2370.16, 开
		2380, 高
		2352, 低
		2367.37, 收
		17259.83 交易量
		]
	*/

	dataArr, ok := resp["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("data assert failed")
	}

	// [1582211700000,9571.74,9592.78,9566.05,9588.33,485.9384]
	klines := make([]Kline, 0)
	for _, v := range dataArr {
		obj, _ := v.([]interface{})
		t := Kline{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			TS:     ToInt64(obj[0]) / 1000,
			Open:   ToFloat64(obj[1]),
			Close:  ToFloat64(obj[4]),
			High:   ToFloat64(obj[2]),
			Low:    ToFloat64(obj[3]),
			Vol:    ToFloat64(obj[5]),
		}
		klines = append(klines, t)
	}

	return klines, nil
}

func (zb *Zb) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	return zb.placeOrder(amount, price, pair, 1)
}

func (zb *Zb) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	return zb.placeOrder(amount, price, pair, 0)
}

func (zb *Zb) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	// TODO 目前没有找到相关接口
	return nil, fmt.Errorf("unsupport the market order")
}

func (zb *Zb) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	// TODO 目前没有找到相关接口
	return nil, fmt.Errorf("unsupport the market order")
}

func (zb *Zb) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("method", "cancelOrder")
	params.Set("id", orderId)
	params.Set("currency", symbol)
	zb.buildPostForm(&params)

	resp, err := HttpPostForm(zb.httpClient, TRADE_URL+"cancelOrder", params)
	if err != nil {
		return false, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return false, err
	}

	code := respmap["code"].(float64)
	if code == 1000 {
		return true, nil
	}

	return false, errors.New(fmt.Sprintf("%.0f", code))
}

func (zb *Zb) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("method", "getOrder")
	params.Set("id", orderId)
	params.Set("currency", symbol)
	zb.buildPostForm(&params)

	resp, err := HttpPostForm(zb.httpClient, TRADE_URL+"getOrder", params)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	ordermap := make(map[string]interface{})
	err = json.Unmarshal(resp, &ordermap)
	if err != nil {
		return nil, err
	}

	order := new(Order)
	order.Market = pair
	order.Symbol = pair.ToLowerSymbol("/")
	parseOrder(order, ordermap)

	return order, nil
}

func (zb *Zb) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	params := url.Values{}
	symbol := pair.ToSymbol("_")
	params.Set("method", "getUnfinishedOrdersIgnoreTradeType")
	params.Set("currency", symbol)
	params.Set("pageIndex", "1")
	params.Set("pageSize", "100")
	zb.buildPostForm(&params)

	resp, err := HttpPostForm(zb.httpClient, TRADE_URL+"getUnfinishedOrdersIgnoreTradeType", params)
	if err != nil {
		return nil, err
	}

	respstr := string(resp)
	if strings.Contains(respstr, "\"code\":3001") {
		log.Println(respstr)
		return nil, nil
	}

	var resps []interface{}
	err = json.Unmarshal(resp, &resps)
	if err != nil {
		return nil, err
	}

	var orders []Order
	for _, v := range resps {
		ordermap := v.(map[string]interface{})
		order := Order{}
		order.Market = pair
		order.Symbol = pair.ToLowerSymbol("/")
		parseOrder(&order, ordermap)
		orders = append(orders, order)
	}

	return orders, nil
}

func (zb *Zb) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	allOrder, err := zb.GetOrders(pair, 100)
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0, len(allOrder))
	for _, v := range allOrder {
		if v.Status == ORDER_FINISH || v.Status == ORDER_PART_FINISH || v.Status == ORDER_CANCEL {
			orders = append(orders, v)
		}
	}

	return orders, nil
}

func (zb *Zb) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	panic("not supported yet")
}

func (zb *Zb) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	panic("not supported yet")
}

func (zb *Zb) GetAccount() (*Account, error) {
	params := url.Values{}
	params.Set("method", "getAccountInfo")
	zb.buildPostForm(&params)
	resp, err := HttpPostForm(zb.httpClient, TRADE_URL+"getAccountInfo", params)
	if err != nil {
		return nil, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	if respmap["code"] != nil && respmap["code"].(float64) != 1000 {
		return nil, errors.New(string(resp))
	}

	acc := new(Account)
	acc.Exchange = zb.GetExchangeName()
	acc.NetAsset = 0
	acc.Asset = 0
	acc.SubAccounts = make(map[Currency]SubAccount)

	resultmap := respmap["result"].(map[string]interface{})
	coins := resultmap["coins"].([]interface{})

	for _, v := range coins {
		vv := v.(map[string]interface{})
		subAcc := SubAccount{}
		subAcc.Amount = ToFloat64(vv["available"])
		subAcc.FrozenAmount = ToFloat64(vv["freez"])
		subAcc.Currency = NewCurrency(vv["key"].(string))
		acc.SubAccounts[subAcc.Currency] = subAcc
	}

	return acc, nil
}

// 返回最近的n条记录
func (zb *Zb) GetOrders(pair CurrencyPair, size int) ([]Order, error) {
	params := url.Values{}
	symbol := pair.ToSymbol("_")
	params.Set("method", "getOrdersIgnoreTradeType")
	params.Set("currency", symbol)
	params.Set("pageIndex", "1")
	params.Set("pageSize", "100")
	zb.buildPostForm(&params)

	resp, err := HttpPostForm(zb.httpClient, TRADE_URL+"getOrdersIgnoreTradeType", params)
	if err != nil {
		return nil, err
	}

	respstr := string(resp)
	if strings.Contains(respstr, "\"code\":3001") {
		log.Println(respstr)
		return nil, nil
	}

	var resps []interface{}
	err = json.Unmarshal(resp, &resps)
	if err != nil {
		return nil, err
	}

	var orders []Order
	for _, v := range resps {
		ordermap := v.(map[string]interface{})
		order := Order{}
		order.Market = pair
		order.Symbol = pair.ToLowerSymbol("/")
		parseOrder(&order, ordermap)
		orders = append(orders, order)
	}

	return orders, nil
}

func (zb *Zb) Withdraw(amount string, currency Currency, fees, receiveAddr, safePwd string) (string, error) {
	params := url.Values{}
	params.Set("method", "withdraw")
	params.Set("currency", currency.LowerSymbol())
	params.Set("amount", amount)
	params.Set("fees", fees)
	params.Set("receiveAddr", receiveAddr)
	params.Set("safePwd", safePwd)
	zb.buildPostForm(&params)

	resp, err := HttpPostForm(zb.httpClient, TRADE_URL+"withdraw", params)
	if err != nil {
		return "", err
	}

	respMap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respMap)
	if err != nil {
		return "", err
	}

	if respMap["code"].(float64) == 1000 {
		return respMap["id"].(string), nil
	}

	return "", errors.New(string(resp))
}

func (zb *Zb) CancelWithdraw(id string, currency Currency, safePwd string) (bool, error) {
	params := url.Values{}
	params.Set("method", "cancelWithdraw")
	params.Set("currency", currency.LowerSymbol())
	params.Set("downloadId", id)
	params.Set("safePwd", safePwd)
	zb.buildPostForm(&params)

	resp, err := HttpPostForm(zb.httpClient, TRADE_URL+"cancelWithdraw", params)
	if err != nil {
		return false, err
	}

	respMap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respMap)
	if err != nil {
		return false, err
	}

	if respMap["code"].(float64) == 1000 {
		return true, nil
	}

	return false, errors.New(string(resp))
}

func (zb *Zb) buildPostForm(postForm *url.Values) error {
	postForm.Set("accesskey", zb.accessKey)

	payload := postForm.Encode()
	secretkeySha, _ := GetSHA(zb.secretKey)

	sign, err := GetParamHmacMD5Sign(secretkeySha, payload)
	if err != nil {
		return err
	}

	postForm.Set("sign", sign)
	postForm.Set("reqTime", fmt.Sprintf("%d", time.Now().UnixNano()/1000000))
	return nil
}

func parseOrder(order *Order, ordermap map[string]interface{}) {
	order.OrderID = ordermap["id"].(string)
	order.Price = ordermap["price"].(float64)
	order.Amount = ordermap["total_amount"].(float64)
	order.DealAmount = ordermap["trade_amount"].(float64)
	if order.DealAmount > 0 {
		order.AvgPrice = ToFloat64(ordermap["trade_money"]) / order.DealAmount
	} else {
		order.AvgPrice = 0
	}
	//	order.Fee = ordermap["fees"].(float64)
	order.TS = int64(ordermap["trade_date"].(float64))

	_status := TradeStatus(ordermap["status"].(float64))
	switch _status {
	case 0:
		order.Status = ORDER_UNFINISH
	case 1:
		order.Status = ORDER_CANCEL
	case 2:
		order.Status = ORDER_FINISH
	case 3:
		order.Status = ORDER_UNFINISH
	}

	orType := ordermap["type"].(float64)
	switch orType {
	case 0:
		order.Side = SELL
	case 1:
		order.Side = BUY
	default:
		log.Printf("unknown order type %f", orType)
	}
}

func (zb *Zb) placeOrder(amount, price string, pair CurrencyPair, tradeType int) (*Order, error) {
	symbol := pair.ToSymbol("_")
	params := url.Values{}
	params.Set("method", "order")
	params.Set("price", price)
	params.Set("amount", amount)
	params.Set("currency", symbol)
	params.Set("tradeType", fmt.Sprintf("%d", tradeType))
	zb.buildPostForm(&params)

	resp, err := HttpPostForm(zb.httpClient, TRADE_URL+"order", params)
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	code := respmap["code"].(float64)
	if code != 1000 {
		return nil, errors.New(fmt.Sprintf("%.0f", code))
	}

	order := new(Order)
	order.OrderID, _ = respmap["id"].(string)
	order.Price, _ = strconv.ParseFloat(price, 64)
	order.Amount, _ = strconv.ParseFloat(amount, 64)
	order.TS = int64(time.Now().UnixNano() / 1000000)
	order.Status = ORDER_UNFINISH
	order.Market = pair
	order.Symbol = pair.ToLowerSymbol("/")

	switch tradeType {
	case 0:
		order.Side = SELL
	case 1:
		order.Side = BUY
	}

	return order, nil
}
