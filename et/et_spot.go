package et

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]int{
	KLINE_M1:   60,
	KLINE_M5:   300,
	KLINE_M15:  900,
	KLINE_M30:  1800,
	KLINE_H1:   3600,
	KLINE_H4:   14400,
	KLINE_DAY:  86400,
	KLINE_WEEK: 604800,
	//KLINE_MONTH: "1mon",
}

type Et struct {
	httpClient *http.Client
	baseUrl    string
	accessKey  string
	secretKey  string
}

func NewSpotAPI(client *http.Client, apiKey, secretKey string) SpotAPI {
	et := &Et{
		httpClient: client,
		baseUrl:    "http://47.108.94.209:19000",
		accessKey:  apiKey,
		secretKey:  secretKey,
	}
	return et
}

func (et *Et) GetExchangeName() string {
	return ET
}

func (et *Et) SetURL(exurl string) {
	et.baseUrl = exurl
}

func (et *Et) GetURL() string {
	return et.baseUrl
}

func (et *Et) buildHeaders() (headers map[string]string) {
	headers = make(map[string]string)
	headers["ApiKey"] = et.accessKey
	headers["ApiSecret"] = et.secretKey
	headers["Content-Type"] = "application/json"
	return headers
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

func (et *Et) GetTradeFee() (tf *TradeFee, err error) {
	tf = &TradeFee{
		Maker: 0.002,
		Taker: 0.002,
	}
	return tf, nil
}

func (et *Et) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	// 易通返回的数据结构如下
	type MarketListResp struct {
		Name      string `json:"name"`
		Stock     string `json:"stock"` // 币币
		Money     string `json:"money"` // 法币
		FeePrec   int    `json:"fee_prec"`
		StockPrec int    `json:"stock_prec"`
		MoneyPrec int    `json:"money_prec"`
		MinAmount string `json:"min_amount"`

		// 是否是自选
		IsDefine bool `json:"is_define"`
	}
	type Result struct {
		Code int              `json:"code"`
		Data []MarketListResp `json:"data"`
	}

	var resp Result
	err := HttpGet4(et.httpClient, et.baseUrl+"/userapi/market/list", nil, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 2000 {
		return nil, fmt.Errorf("result code is %v", resp.Code)
	}

	tf, err := et.GetTradeFee()
	if err != nil {
		return nil, err
	}
	ssm := make(map[string]SymbolSetting)
	for _, v := range resp.Data {
		symbol := strings.ToLower(v.Name)
		minAmount, _ := strconv.ParseFloat(v.MinAmount, 64)
		ssm[symbol] = SymbolSetting{
			Symbol:      symbol,
			Base:        strings.ToLower(v.Stock),
			Quote:       strings.ToLower(v.Money),
			MinSize:     math.Pow10(-v.StockPrec),
			MinPrice:    math.Pow10(-v.MoneyPrec),
			MinNotional: minAmount,
			MakerFee:    tf.Maker,
			TakerFee:    tf.Taker,
		}
	}

	return ssm, nil
}

func (et *Et) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	all, err := et.GetAllCurrencyStatus()
	if err != nil {
		return CurrencyStatus{
			Deposit:  false,
			Withdraw: false,
		}, err
	}

	return all[currency.Symbol()], nil
}

func (et *Et) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	ss, err := et.GetAllCurrencyPair()
	if err != nil {
		return nil, err
	}

	currencyMap := GetCurrencyMap(ss)

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

func (et *Et) GetTicker(pair CurrencyPair) (*Ticker, error) {
	symbol := pair.ToSymbol("/")
	resp, err := HttpGet(et.httpClient, et.baseUrl+fmt.Sprintf("/userapi/market/ticker?market=%s", symbol))
	if err != nil {
		return nil, err
	}

	code, _ := resp["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	// {"code":2000,"msg":"success","data":{"open":"1.49993","last":"6000","high":"6000","low":"1.49993","deal":"8401.9127594687257","volume":"2.67523249"},"hcode":200}
	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	ticker := new(Ticker)
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	ticker.Open, _ = strconv.ParseFloat(data["open"].(string), 64)
	ticker.Last, _ = strconv.ParseFloat(data["last"].(string), 64)
	ticker.High, _ = strconv.ParseFloat(data["high"].(string), 64)
	ticker.Low, _ = strconv.ParseFloat(data["low"].(string), 64)
	ticker.Vol, _ = strconv.ParseFloat(data["volume"].(string), 64)
	//ticker.Buy, _ = strconv.ParseFloat(data["buy"].(string), 64)
	//ticker.Sell, _ = strconv.ParseFloat(data["sell"].(string), 64)
	ticker.TS = time.Now().UnixNano() / int64(time.Millisecond)

	return ticker, nil
}

func (et *Et) GetAllTicker() ([]Ticker, error) {
	resp, err := HttpGet(et.httpClient, et.baseUrl+"/userapi/market/allticker")
	if err != nil {
		return nil, err
	}

	code, _ := resp["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	// {"code":2000,"msg":"success","data":{"open":"1.49993","last":"6000","high":"6000","low":"1.49993","deal":"8401.9127594687257","volume":"2.67523249"},"hcode":200}
	dataArr, _ := resp["data"].([]interface{})
	if dataArr == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	tickers := make([]Ticker, 0, len(dataArr))
	for _, v := range dataArr {
		data, _ := v.(map[string]interface{})
		symbol := ToString(data["name"])
		pair := NewCurrencyPairFromString(symbol)
		ticker := Ticker{}
		ticker.Market = pair
		ticker.Symbol = pair.ToLowerSymbol("/")
		ticker.Open, _ = strconv.ParseFloat(data["open"].(string), 64)
		ticker.Last, _ = strconv.ParseFloat(data["last"].(string), 64)
		ticker.High, _ = strconv.ParseFloat(data["high"].(string), 64)
		ticker.Low, _ = strconv.ParseFloat(data["low"].(string), 64)
		ticker.Vol, _ = strconv.ParseFloat(data["volume"].(string), 64)
		//ticker.Buy, _ = strconv.ParseFloat(data["buy"].(string), 64)
		//ticker.Sell, _ = strconv.ParseFloat(data["sell"].(string), 64)
		ticker.TS = time.Now().UnixNano() / int64(time.Millisecond)

		tickers = append(tickers, ticker)
	}

	return tickers, nil
}

func (et *Et) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	symbol := pair.ToSymbol("/")
	resp, err := HttpGet(et.httpClient, et.baseUrl+fmt.Sprintf("/userapi/market/depth?market=%s&limit=%d&interval=%d", symbol, size, step))
	if err != nil {
		return nil, err
	}

	code, _ := resp["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	asks, isok1 := data["asks"].([]interface{})
	bids, isok2 := data["bids"].([]interface{})

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

func (et *Et) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	symbol := pair.ToSymbol("/")
	resp, err := HttpGet(et.httpClient, et.baseUrl+fmt.Sprintf("/userapi/market/trade?market=%v&limit=%v", symbol, size))
	if err != nil {
		return nil, err
	}

	code, _ := resp["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	data, _ := resp["data"].([]interface{})
	if data == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	/*
		[
		    {"id":1250277,"time":1588671358.288203,"type":"buy","amount":"0.1","price":"6000"}
		]
	*/

	trades := make([]Trade, 0)
	for _, v := range data {
		obj, _ := v.(map[string]interface{})
		t := Trade{
			Tid:    ToInt64(obj["id"]),
			Side:   BUY,
			Amount: ToFloat64(obj["amount"]),
			Price:  ToFloat64(obj["price"]),
			TS:     int64(ToFloat64(obj["time"]) * 1000.0),
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

func (et *Et) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	symbol := pair.ToSymbol("/")
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", et.GetExchangeName(), period)
	}
	resp, err := HttpGet(et.httpClient, et.baseUrl+fmt.Sprintf("/userapi/market/kline?market=%v&interval=%v", symbol, periodS))
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

	code, _ := resp["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	data, _ := resp["data"].([]interface{})
	if data == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	// [1588668960,"6852.61","6852.61","6852.61","6852.61","0","0","BTC/USDT"]
	klines := make([]Kline, 0)
	for _, v := range data {
		obj, _ := v.([]interface{})
		t := Kline{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			TS:     ToInt64(obj[0]),
			Open:   ToFloat64(obj[1]),
			Close:  ToFloat64(obj[2]),
			High:   ToFloat64(obj[3]),
			Low:    ToFloat64(obj[4]),
			Vol:    ToFloat64(obj[5]),
		}
		klines = append(klines, t)
	}

	return klines, nil
}

func (et *Et) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	return et.placeOrder(amount, price, pair, 2)
}

func (et *Et) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	return et.placeOrder(amount, price, pair, 1)
}

func (et *Et) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	return et.placeMarketOrder(amount, pair, 2)
}

func (et *Et) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	return et.placeMarketOrder(amount, pair, 1)
}

func (et *Et) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	oid, _ := strconv.ParseFloat(orderId, 64)
	reqMap := map[string]interface{}{
		"market":   pair.ToSymbol("/"),
		"order_id": oid,
	}

	resp, err := HttpPostForm6(et.httpClient, et.baseUrl+"/userapi/order/cancel", reqMap, et.buildHeaders())
	if err != nil {
		return false, err
	}

	// {"code":2000,"msg":"success","data":"ok","hcode":200}

	response := make(map[string]interface{})
	if err := json.Unmarshal(resp, &response); err != nil {
		fmt.Printf("解析json失败：%+v", err)
		return false, err
	}

	code, _ := response["code"].(float64)
	if code != 2000 {
		// 一点都没有成交的订单，被取消后，会返回 order not found
		msg, _ := response["msg"].(string)
		if strings.Contains(msg, "order not found") {
			return true, nil
		}

		return false, fmt.Errorf("exchange error code is %v", code)
	}

	return true, nil
}

type RspOrdersOrders struct {
	OrderType        uint8
	State            uint8
	Price            float64
	Amount           float64
	FilledAmount     float64
	FilledCashAmount float64
	FilledFees       float64
	CreatedAt        uint64
	FinishedAt       uint64
	CanceledAt       uint64
	Source           []uint8
	Symbol           []uint8
	AccountId        []uint8
	OrderId          []uint8
}

type RspOrders struct {
	Status uint8
	Orders []RspOrdersOrders
}

func (et *Et) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	reqURL := fmt.Sprintf("market=%v&order_id=%v", pair.ToSymbol("/"), orderId)
	response, err := HttpGet2(et.httpClient, et.baseUrl+"/userapi/order/detail?"+reqURL, et.buildHeaders())
	if err != nil {
		if err.Error() == "order not found" { // 易通未成交的订单，撤单后，查询不到
			order := &Order{}
			order.OrderID = orderId
			order.Status = ORDER_CANCEL
			return order, nil
		}
		fmt.Printf("解析json失败：%+v", err)
		return nil, err
	}

	code, _ := response["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	data, _ := response["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	order := &Order{}
	parseOrder(order, data)

	return order, nil
}

func (et *Et) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	reqURL := fmt.Sprintf("market=%v&side=%v&limit=%v", pair.ToSymbol("/"), 0, 500)
	response, err := HttpGet2(et.httpClient, et.baseUrl+"/userapi/order/pending?"+reqURL, et.buildHeaders())
	if err != nil {
		fmt.Printf("解析json失败：%+v", err)
		return nil, err
	}

	code, _ := response["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	list, _ := response["data"].([]interface{})
	if list == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	var orders []Order
	for _, v := range list {
		data, _ := v.(map[string]interface{})
		order := Order{}
		parseOrder(&order, data)
		orders = append(orders, order)
	}

	return orders, nil
}

func (et *Et) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	reqURL := fmt.Sprintf("market=%v&side=%v&limit=%v", pair.ToSymbol("/"), 0, 100)
	response, err := HttpGet2(et.httpClient, et.baseUrl+"/userapi/order/finished?"+reqURL, et.buildHeaders())
	if err != nil {
		fmt.Printf("解析json失败：%+v", err)
		return nil, err
	}

	code, _ := response["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	list, _ := response["data"].([]interface{})
	if list == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	var orders []Order
	for _, v := range list {
		data, _ := v.(map[string]interface{})
		order := Order{}
		parseOrder(&order, data)
		orders = append(orders, order)
	}

	return orders, nil
}

func (et *Et) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	reqURL := fmt.Sprintf("orderid=%v&offset=%v&limit=%v", orderId, 0, 100)
	response, err := HttpGet2(et.httpClient, et.baseUrl+"/userapi/order/deal?"+reqURL, et.buildHeaders())
	if err != nil {
		fmt.Printf("解析json失败：%+v", err)
		return nil, err
	}

	code, _ := response["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	list, _ := response["data"].([]interface{})
	if list == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	deals := make([]OrderDeal, 0, len(list))
	for _, v := range list {
		obj := v.(map[string]interface{})
		deal := OrderDeal{
			OrderID:      orderId,
			DealID:       fmt.Sprint(ToInt64(obj["deal_order_id"])),
			TS:           int64(ToFloat64(obj["time"]) * 1000),
			Price:        ToFloat64(obj["price"]),
			FilledAmount: ToFloat64(obj["amount"]),
			Market:       pair,
			Symbol:       pair.ToLowerSymbol("/"),
		}

		deal.FilledCashAmount = deal.Price * deal.FilledAmount
		side := ToInt(obj["side"])
		switch side {
		case 1:
			deal.Side = SELL
		case 2:
			deal.Side = BUY
		default:
		}
		deals = append(deals, deal)
	}

	return deals, nil
}

func (et *Et) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	panic("not supported yet")
}

func (et *Et) GetAccount() (*Account, error) {
	resp, err := HttpGet2(et.httpClient, et.baseUrl+"/userapi/account/balance", et.buildHeaders())
	if err != nil {
		return nil, err
	}

	code, _ := resp["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	balances, _ := data["balances"].([]interface{})
	if balances == nil {
		return nil, fmt.Errorf("exchange error balances is nil")
	}

	acc := new(Account)
	acc.Exchange = et.GetExchangeName()
	acc.NetAsset = 0
	acc.Asset = 0
	acc.SubAccounts = make(map[Currency]SubAccount)

	for _, v := range balances {
		obj, _ := v.(map[string]interface{})
		symbol := ToString(obj["symbol"])
		currency := NewCurrency(symbol)
		state := ToInt(obj["state"])
		balance := ToFloat64(obj["balance"])

		b, ok := acc.SubAccounts[currency]
		if ok {
			switch state {
			case 0:
				b.Amount = balance
			case 1:
				b.FrozenAmount = balance
			}
		} else {
			b = SubAccount{}
			b.Currency = currency
			switch state {
			case 0:
				b.Amount = balance
			case 1:
				b.FrozenAmount = balance
			}
		}
		acc.SubAccounts[currency] = b
	}

	return acc, nil
}

func parseOrder(order *Order, data map[string]interface{}) {
	order.OrderID = fmt.Sprintf("%v", ToInt64(data["id"]))
	order.Price = ToFloat64(data["price"])
	order.Amount = ToFloat64(data["amount"])
	order.TS = int64(ToFloat64(data["ctime"]) * 1000)

	status := ToInt(data["status"])
	switch status {
	case 0:
		order.Status = ORDER_UNFINISH
	case 1:
		order.Status = ORDER_PART_FINISH
	case 2:
		order.Status = ORDER_FINISH
	case 3:
		order.Status = ORDER_PART_FINISH
	case 4:
		order.Status = ORDER_CANCEL
	}

	order.DealAmount = ToFloat64(data["deal_stock"])
	dealMoney := ToFloat64(data["deal_money"])
	if order.DealAmount > 0 {
		order.AvgPrice = dealMoney / order.DealAmount
	} else {
		order.AvgPrice = 0
	}

	/*
		Fee        float64      `json:"fee,string"`         // 手续费
		TS         int64        `json:"ts"`                 // 时间，单位为毫秒(millisecond)
	*/

	kind := ToInt(data["type"])
	side := ToInt(data["side"])
	if kind == 1 {
		switch side {
		case 1:
			order.Side = SELL
		case 2:
			order.Side = BUY
		default:
		}
	} else {
		switch side {
		case 1:
			order.Side = SELL_MARKET
		case 2:
			order.Side = BUY_MARKET
		default:
		}

		order.Price = order.AvgPrice
		order.Amount = order.DealAmount
	}

	order.Symbol = strings.ToLower(ToString(data["market"]))
	order.Market = NewCurrencyPairFromString(order.Symbol)
}

// side: 1卖单 2买单
func (et *Et) placeOrder(amount, price string, pair CurrencyPair, side int) (*Order, error) {
	reqMap := map[string]interface{}{
		"market": pair.ToSymbol("/"),
		"side":   side,
		"amount": amount,
		"price":  price,
		"source": "gdxt",
	}

	resp, err := HttpPostForm6(et.httpClient, et.baseUrl+"/userapi/order/place", reqMap, et.buildHeaders())
	if err != nil {
		return nil, err
	}
	fmt.Println(string(resp))

	response := make(map[string]interface{})
	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, err
	}

	code, _ := response["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	data, _ := response["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	order := new(Order)
	order.OrderID = fmt.Sprintf("%v", ToInt64(data["id"]))
	order.Price, _ = strconv.ParseFloat(price, 64)
	order.Amount, _ = strconv.ParseFloat(amount, 64)
	order.TS = int64(time.Now().UnixNano() / 1000000)
	order.Status = ORDER_UNFINISH
	order.Market = pair
	order.Symbol = pair.ToLowerSymbol("/")

	switch side {
	case 1:
		order.Side = SELL
	case 2:
		order.Side = BUY
	}

	return order, nil
}

// side: 1卖单 2买单
func (et *Et) placeMarketOrder(amount string, pair CurrencyPair, side int) (*Order, error) {
	reqMap := map[string]interface{}{
		"market": pair.ToSymbol("/"),
		"side":   side,
		"amount": amount,
	}

	resp, err := HttpPostForm6(et.httpClient, et.baseUrl+"/userapi/order/placemarket", reqMap, et.buildHeaders())
	if err != nil {
		return nil, err
	}
	fmt.Println(string(resp))

	response := make(map[string]interface{})
	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, err
	}

	code, _ := response["code"].(float64)
	if code != 2000 {
		return nil, fmt.Errorf("exchange error code is %v", code)
	}

	data, _ := response["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("exchange error data is nil")
	}

	order := new(Order)
	order.OrderID = fmt.Sprintf("%v", ToInt64(data["id"]))
	order.Amount, _ = strconv.ParseFloat(amount, 64)
	order.TS = int64(time.Now().UnixNano() / 1000000)
	order.Status = ORDER_UNFINISH
	order.Market = pair
	order.Symbol = pair.ToLowerSymbol("/")

	switch side {
	case 1:
		order.Side = SELL_MARKET
	case 2:
		order.Side = BUY_MARKET
	}

	return order, nil
}
