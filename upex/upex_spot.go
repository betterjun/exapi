package upex

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	"github.com/json-iterator/go"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]string{
	KLINE_M1:   "1",
	KLINE_M5:   "5",
	KLINE_M15:  "15",
	KLINE_M30:  "30",
	KLINE_H1:   "60",
	KLINE_DAY:  "1440",
	KLINE_WEEK: "10080",
}

type Upex struct {
	httpClient *http.Client
	baseUrl    string
	apiKey     string
	secretKey  string

	// 用于币种转换
	symbolMap map[string]CurrencyPair
}

/**
 * spot
 */
func NewSpotAPI(client *http.Client, apiKey, secretkey string) SpotAPI {
	upex := new(Upex)
	upex.baseUrl = "https://apiv3.upex.io/exchange-open-api/open/api"
	upex.httpClient = client
	upex.apiKey = apiKey
	upex.secretKey = secretkey
	return upex
}

func (upex *Upex) GetExchangeName() string {
	return UPEX
}

func (upex *Upex) SetURL(exurl string) {
	upex.baseUrl = exurl
}

func (upex *Upex) GetURL() string {
	return upex.baseUrl
}

func (upex *Upex) getDataMap(reqUrl string) (map[string]interface{}, error) {
	respmap, err := HttpGet(upex.httpClient, reqUrl)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["code"].(string)
	if !ok {
		return nil, errors.New("code assert error")
	}

	if code != "0" {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	return datamap, nil
}

func (upex *Upex) getDataArray(reqUrl string) ([]interface{}, error) {
	respmap, err := HttpGet(upex.httpClient, reqUrl)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["code"].(string)
	if !ok {
		return nil, errors.New("code assert error")
	}

	if code != "0" {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	dataArr, ok := respmap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	return dataArr, nil
}

func (upex *Upex) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	url := upex.baseUrl + "/common/symbols"
	dataArr, err := upex.getDataArray(url)
	if err != nil {
		return nil, err
	}
	/*
		{"symbol":"btcusdt","count_coin":"USDT","amount_precision":4,"base_coin":"BTC","price_precision":2},
	*/

	ssm := make(map[string]SymbolSetting)
	for _, v := range dataArr {
		obj := v.(map[string]interface{})
		base := ToString(obj["base_coin"])
		quote := ToString(obj["count_coin"])
		symbol := base + "/" + quote

		ssm[symbol] = SymbolSetting{
			Symbol:   symbol,
			Base:     base,
			Quote:    quote,
			MinSize:  math.Pow10(-ToInt(obj["amount_precision"])),
			MinPrice: math.Pow10(-ToInt(obj["price_precision"])),
			//MinNotional: ToFloat64(obj["minTrade"]),
			MakerFee: 0.002,
			TakerFee: 0.002,
		}
	}

	return ssm, nil
}

func (upex *Upex) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	all, err := upex.GetAllCurrencyStatus()
	if err == nil {
		return all[currency.Symbol()], nil
	}

	return CurrencyStatus{}, errors.New("Asset not found")
}

func (upex *Upex) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	ssm, err := upex.GetAllCurrencyPair()
	if err != nil {
		return nil, err
	}

	all = make(map[string]CurrencyStatus)
	for _, v := range ssm {
		if _, ok := all[v.Base]; !ok {
			all[v.Base] = CurrencyStatus{
				Deposit:  true,
				Withdraw: true,
			}
		}

		if _, ok := all[v.Quote]; !ok {
			all[v.Quote] = CurrencyStatus{
				Deposit:  true,
				Withdraw: true,
			}
		}
	}

	return all, nil
}

func (upex *Upex) parseTicker(pair CurrencyPair, tickmap map[string]interface{}, ts int64) (*Ticker, error) {
	/*
		{"high":11448.60000000,"vol":27728.78950000,"last":11263.5600000000000000,"low":10961.00000000,"buy":11263.56,"sell":11263.57,"rose":0.01959818,"time":1596243313630}
	*/
	ticker := new(Ticker)
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	//ticker.Open = ToFloat64(tickmap["open"])
	ticker.Last = ToFloat64(tickmap["last"])
	ticker.High = ToFloat64(tickmap["high"])
	ticker.Low = ToFloat64(tickmap["low"])
	ticker.Vol = ToFloat64(tickmap["vol"])
	ticker.Buy = ToFloat64(tickmap["buy"])
	ticker.Sell = ToFloat64(tickmap["sell"])
	if ts == 0 {
		ticker.TS = ToInt64(tickmap["time"])
	} else {
		ticker.TS = ts
	}
	rose := ToFloat64(tickmap["rose"])
	ticker.Open = ticker.Last / (rose + 1)

	return ticker, nil
}

func (upex *Upex) GetTicker(pair CurrencyPair) (*Ticker, error) {
	url := upex.baseUrl + "/get_ticker?symbol=" + pair.ToLowerSymbol("")

	datamap, err := upex.getDataMap(url)
	if err != nil {
		return nil, err
	}

	return upex.parseTicker(pair, datamap, 0)
}

func (upex *Upex) GetAllTicker() ([]Ticker, error) {
	if upex.symbolMap == nil {
		err := upex.initSymbolToPair()
		if err != nil {
			return nil, err
		}
	}

	url := upex.baseUrl + "/get_allticker"
	datamap, err := upex.getDataMap(url)
	if err != nil {
		return nil, err
	}

	dataArr, ok := datamap["ticker"].([]interface{})
	if !ok {
		return nil, errors.New("ticker assert error")
	}

	ts := ToInt64(datamap["date"])
	tickers := make([]Ticker, 0, len(dataArr))
	for _, v := range dataArr {
		obj, ok := v.(map[string]interface{})
		if !ok {
			Error("parse ticker failed: dataArr element assert failed")
			continue
		}

		symbol, ok := obj["symbol"].(string)
		if !ok {
			Error("parse ticker failed: dataArr symbol assert failed")
			continue
		}

		t, err := upex.parseTicker(upex.symbolToPair(symbol), obj, ts)
		if err != nil {
			Error("parse ticker failed:%v", err)
			continue
		}
		tickers = append(tickers, *t)
	}

	return tickers, nil
}

func (upex *Upex) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	url := upex.baseUrl + "/market_dept?type=step0&symbol=" + pair.ToLowerSymbol("")
	datamap, err := upex.getDataMap(url)
	if err != nil {
		return nil, err
	}

	tickMap, ok := datamap["tick"].(map[string]interface{})
	if !ok {
		return nil, errors.New("tick assert error")
	}

	dep := parseDepthData(tickMap)
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")

	return dep, nil
}

func (upex *Upex) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	//url := upex.baseUrl + "openApi/market/trade?symbol=" + pair.ToSymbol("-")
	//datamap, err := upex.getDataMap(url)
	//if err != nil {
	//	return nil, err
	//}
	//
	//dataArr, ok := datamap["data"].([]interface{})
	//if !ok {
	//	return nil, errors.New("data assert error")
	//}
	//
	///*
	//   "id": 17592256642623,
	//              "amount": 0.04,
	//              "price": 1997,
	//              "direction": "buy",
	//              "ts": 1502448920106
	//*/
	//trades := make([]Trade, 0, len(dataArr))
	//for _, d := range dataArr {
	//	obj := d.(map[string]interface{})
	//
	//	trades = append(trades, Trade{
	//		Tid:    ToInt64(obj["id"]),
	//		Market: pair,
	//		Symbol: pair.ToLowerSymbol("/"),
	//		Amount: ToFloat64(obj["amount"]),
	//		Price:  ToFloat64(obj["price"]),
	//		Side:   AdaptTradeSide(ToString(obj["direction"])),
	//		TS:     ToInt64(obj["ts"]) * 1000})
	//}
	//
	//return trades, nil

	return nil, ErrorUnsupported
}

//倒序
func (upex *Upex) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	//periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	//if isOk != true {
	//	return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", upex.GetExchangeName(), period)
	//}
	//url := upex.baseUrl + "openApi/market/kline?symbol=%s&period=%v&size=%v"
	//symbol := pair.ToSymbol("-")
	//datamap, err := upex.getDataMap(fmt.Sprintf(url, symbol, periodS, size))
	//if err != nil {
	//	return nil, err
	//}
	//
	//dataArr, ok := datamap["data"].([]interface{})
	//if !ok {
	//	return nil, err
	//}
	///*
	//   "id": 1499184000,
	//           "amount": 37593.0266,
	//           "count": 0,
	//           "open": 1935.2000,
	//           "close": 1879.0000,
	//           "low": 1856.0000,
	//           "high": 1940.0000,
	//           "vol": 71031537.97866500
	//*/
	//klines := make([]Kline, 0, len(dataArr))
	//for i := len(dataArr) - 1; i >= 0; i-- {
	//	item := dataArr[i].(map[string]interface{})
	//	klines = append(klines, Kline{
	//		Market: pair,
	//		Symbol: pair.ToLowerSymbol("/"),
	//		Open:   ToFloat64(item["open"]),
	//		Close:  ToFloat64(item["close"]),
	//		High:   ToFloat64(item["high"]),
	//		Low:    ToFloat64(item["low"]),
	//		Vol:    ToFloat64(item["vol"]),
	//		TS:     ToInt64(item["id"])})
	//}
	//return klines, nil
	return nil, ErrorUnsupported
}

func (upex *Upex) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := upex.placeOrder(amount, price, pair, "BUY", "1")
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

func (upex *Upex) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := upex.placeOrder(amount, price, pair, "SELL", "1")
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

func (upex *Upex) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := upex.placeOrder(amount, "", pair, "BUY", "2")
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

func (upex *Upex) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := upex.placeOrder(amount, "", pair, "SELL", "2")
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

func (upex *Upex) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	requrl := upex.baseUrl + "/cancel_order"
	params := map[string]string{}
	params["order_id"] = orderId
	params["symbol"] = pair.ToLowerSymbol("")

	_, err := upex.httpPost(requrl, params)
	if err != nil {
		return false, err
	}

	return true, nil
}

// 交易所有问题：一点都没成交或部分成交的订单，查询不到数据
func (upex *Upex) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	requrl := upex.baseUrl + "/order_info"
	params := map[string]string{}
	params["order_id"] = orderId
	params["symbol"] = pair.ToLowerSymbol("")

	respmap, err := upex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	entrust, ok := datamap["order_info"].(map[string]interface{})
	if !ok {
		return nil, errors.New("order_info assert error")
	}

	order := upex.parseOrder(entrust, pair)
	return &order, nil
}

func (upex *Upex) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	requrl := upex.baseUrl + "/v2/new_order"
	params := map[string]string{}
	params["symbol"] = pair.ToLowerSymbol("")
	params["pageSize"] = fmt.Sprint(100)
	params["page"] = fmt.Sprint(1)

	respmap, err := upex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	dataArr, ok := datamap["resultList"].([]interface{})
	if !ok {
		return nil, errors.New("resultList assert error")
	}

	orders := make([]Order, 0, len(dataArr))
	for _, v := range dataArr {
		obj, _ := v.(map[string]interface{})
		orders = append(orders, upex.parseOrder(obj, pair))
	}

	return orders, nil
}

func (upex *Upex) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	requrl := upex.baseUrl + "/v2/all_order"
	params := map[string]string{}
	params["symbol"] = pair.ToSymbol("-")
	params["pageSize"] = fmt.Sprint(100)
	params["page"] = fmt.Sprint(1)

	respmap, err := upex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	dataArr, ok := datamap["orderList"].([]interface{})
	if !ok {
		return nil, errors.New("orderList assert error")
	}

	orders := make([]Order, 0, len(dataArr))
	for _, v := range dataArr {
		obj, _ := v.(map[string]interface{})
		orders = append(orders, upex.parseOrder(obj, pair))
	}

	return orders, nil
}

func (upex *Upex) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	requrl := upex.baseUrl + "/order_info"
	params := map[string]string{}
	params["order_id"] = orderId
	params["symbol"] = pair.ToLowerSymbol("")

	respmap, err := upex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	entrust, ok := datamap["order_info"].(map[string]interface{})
	if !ok {
		return nil, errors.New("order_info assert error")
	}

	order := upex.parseOrder(entrust, pair)

	// 取消的订单，没有成交明细
	if order.Status == ORDER_CANCEL {
		return nil, nil
	}

	trades, ok := datamap["trade_list"].([]interface{})
	if !ok {
		return nil, errors.New("trade_list assert error")
	}

	return upex.parseOrderDeal(orderId, order.Side, trades, pair)
}

func (upex *Upex) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	return nil, ErrorUnsupported
}

func (upex *Upex) GetAccount() (*Account, error) {
	requrl := upex.baseUrl + "/user/account"
	params := map[string]string{}
	respmap, err := upex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	dataArr, ok := datamap["coin_list"].([]interface{})
	if !ok {
		return nil, errors.New("coin_list assert error")
	}

	acc := new(Account)
	acc.SubAccounts = make(map[Currency]SubAccount)
	acc.Exchange = upex.GetExchangeName()

	for _, v := range dataArr {
		/*
					{
			"coin": "btc",
			"normal": 32323.233,
			"locked": 32323.233,
			"btcValuatin": 112.33
			}
		*/
		balancemap := v.(map[string]interface{})
		currencySymbol := balancemap["coin"].(string)
		currency := NewCurrency(currencySymbol)

		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(balancemap["normal"]),
			FrozenAmount: ToFloat64(balancemap["locked"]),
			LoanAmount:   0,
		}
	}

	return acc, nil
}

func (upex *Upex) initSymbolToPair() error {
	ssm, err := upex.GetAllCurrencyPair()
	if err != nil {
		Error("%v", err)
		return err
	}

	upex.symbolMap = make(map[string]CurrencyPair)
	for _, v := range ssm {
		s := strings.ToLower(v.Base + v.Quote)
		upex.symbolMap[s] = NewCurrencyPair(NewCurrency(v.Base), NewCurrency(v.Quote))
	}

	return nil
}

func (upex *Upex) symbolToPair(symbol string) (pair CurrencyPair) {
	return upex.symbolMap[symbol]
}

func (upex *Upex) placeOrder(amount, price string, pair CurrencyPair, side, orderType string) (string, error) {
	/*

		参数名	是否必需	类型	示例	说明
		symbol	是	string	BTC-USDT	交易对
		side	是  string BUY SELL
		type	是	string	1：限价, 2市价
		volume	是	float	type为1，表示购买数量；type为2，买表示总价格，卖表示个数
		price	否	float	type为1 必填
	*/
	requrl := upex.baseUrl + "/create_order"
	params := map[string]string{}
	params["symbol"] = pair.ToSymbol("-")
	params["side"] = side
	params["type"] = orderType
	params["volume"] = amount
	if orderType == "1" {
		params["price"] = price
	}

	respmap, err := upex.httpPost(requrl, params)
	if err != nil {
		return "", err
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return "", errors.New("result assert error")
	}

	return ToString(datamap["order_id"]), nil
}

func (upex *Upex) parseOrder(ordmap map[string]interface{}, pair CurrencyPair) Order {
	/*
		"id":184880,
		"side":"BUY",
		"total_price":"340.00000000",
		"created_at":1544608016995,
		"avg_price":"0.00000000",
		"countCoin":"USDT",
		"source":3,"type":1,
		"side_msg":"??",
		"volume":"0.10000000",
		"price":"3400.00000000",
		"source_msg":"API",
		"status_msg":"???",
		"deal_volume":"0.00000000",
		"remain_volume":"0.10000000",
		"baseCoin":"BTC",
		"tradeList":[],
		"status":4
	*/
	//fmt.Printf("ordmap1:%+v\n", ordmap)
	//fmt.Printf("ordmap2:%#v\n", ordmap)

	ord := Order{
		OrderID:    ToString(ordmap["id"]),
		Amount:     ToFloat64(ordmap["volume"]),
		AvgPrice:   ToFloat64(ordmap["avg_price"]),
		Price:      ToFloat64(ordmap["price"]),
		DealAmount: ToFloat64(ordmap["deal_volume"]),
		TS:         ToInt64(ordmap["created_at"]),
	}

	state := ordmap["status"].(float64)
	switch state {
	case 0, 1:
		ord.Status = ORDER_UNFINISH
	case 2:
		ord.Status = ORDER_FINISH
	case 3:
		ord.Status = ORDER_PART_FINISH
	case 4, 5:
		ord.Status = ORDER_CANCEL
	default:
		ord.Status = ORDER_UNFINISH
	}

	// 委托类型:2=市价,1=限价
	tradeType, _ := ordmap["type"].(float64)
	// 委托方向:buy/sell
	side := ordmap["side"].(string)
	switch side {
	case "SELL":
		if tradeType == 2 {
			ord.Side = SELL_MARKET
			ord.Price = ord.AvgPrice
			ord.Amount = ord.DealAmount
		} else {
			ord.Side = SELL
		}
	case "BUY":
		if tradeType == 2 {
			ord.Side = BUY_MARKET
			ord.Price = ord.AvgPrice
			ord.Amount = ord.DealAmount
		} else {
			ord.Side = BUY
		}
	}

	ord.Market = pair
	ord.Symbol = pair.ToLowerSymbol("/")
	return ord
}

func (upex *Upex) parseOrderDeal(orderId string, side TradeSide, trades []interface{}, pair CurrencyPair) ([]OrderDeal, error) {
	/*
		{
		"id": 343,
		"created_at": "09-22 12:22",
		"price": 222.33,
		"volume": 222.33,
		"deal_price": 222.33,
		"fee": 222.33
		},
	*/

	deals := make([]OrderDeal, 0, len(trades))
	for _, v := range trades {
		dealMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		deal := OrderDeal{
			OrderID:          orderId,
			DealID:           fmt.Sprint(ToInt64(dealMap["id"])),
			Price:            ToFloat64(dealMap["price"]),
			FilledAmount:     ToFloat64(dealMap["volume"]),
			FilledCashAmount: ToFloat64(dealMap["deal_price"]),
			Side:             side,
			Market:           pair,
			Symbol:           pair.ToLowerSymbol("/"),
		}
		//date, err := time.ParseInLocation("2006-01-02 15:04:05", ToString(dealMap["created_at"]), time.Local)
		date, err := time.ParseInLocation("01-02 15:04", ToString(dealMap["created_at"]), time.Local)
		if err == nil {
			deal.TS = date.Unix() * 1000
		}

		deals = append(deals, deal)
	}

	return deals, nil
}

func parseDepthData(tick map[string]interface{}) *Depth {
	bids, _ := tick["bids"].([]interface{})
	asks, _ := tick["asks"].([]interface{})

	depth := new(Depth)
	depth.TS = ToInt64(tick["time"])
	if depth.TS == 0 {
		depth.TS = time.Now().Unix() * 1000
	}

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

func (upex *Upex) buildHeaders(params map[string]string) map[string]string {
	/*
		$header[] = 'Nonce: 1534927978_ab43c';
		$header[] = 'Token: 57ba172a6be125cca2f449826f9980ca';
		$header[] = 'Signature: v490hupi0s0bckcp6ivb69p921';
	*/

	nonce := genNonce()
	headers := make(map[string]string)
	headers["Nonce"] = nonce
	headers["Token"] = upex.apiKey
	headers["Signature"] = upex.sign(nonce, params)

	return headers
}

func genNonce() string {
	return fmt.Sprintf("%d_%v", time.Now().Unix(), getRandStr(5))
}

const (
	ascstr = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// 获取随机字符串
func getRandStr(size int) string {
	var bytes = make([]byte, size)
	rand.Read(bytes)
	length := byte(len(ascstr))
	for k, v := range bytes {
		bytes[k] = ascstr[v%length]
	}
	return string(bytes)
}

func (upex *Upex) sign(nonce string, params map[string]string) string {
	data := make([]string, 0)
	data = append(data, upex.apiKey)
	data = append(data, upex.secretKey)
	data = append(data, nonce)

	for k, v := range params {
		data = append(data, fmt.Sprintf("%v=%v", k, v))
	}
	sort.Strings(data)
	bt := strings.Join(data, "")

	h := sha1.New()
	h.Write([]byte(bt))
	sign := hex.EncodeToString(h.Sum(nil))
	return sign
}

func (upex *Upex) httpPost(requrl string, params map[string]string) (map[string]interface{}, error) {
	var strRequestUrl string
	strRequestUrl = requrl

	respData, err := HttpPostForm5(upex.httpClient, strRequestUrl, upex.map2UrlQuery(params), nil)
	if err != nil {
		return nil, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(respData, &respmap)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["code"].(string)
	if !ok {
		return nil, errors.New("code assert error")
	}

	if code != "0" {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	return respmap, nil
}

func (upex *Upex) httpGet(requrl string, params map[string]string) (map[string]interface{}, error) {
	var strRequestUrl string
	if nil == params {
		strRequestUrl = requrl
	} else {
		strRequestUrl = requrl + "?" + upex.map2UrlQuery(params)
	}

	respmap, err := HttpGet2(upex.httpClient, strRequestUrl, upex.buildHeaders(params))
	if err != nil {
		return nil, err
	}

	code, ok := respmap["code"].(string)
	if !ok {
		return nil, errors.New("code assert error")
	}

	if code != "0" {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	return respmap, nil
}

// 将map格式的请求参数转换为字符串格式的
// mapParams: map格式的参数键值对
// return: 查询字符串
func (upex *Upex) map2UrlQuery(mapParams map[string]string) string {
	var strParams string
	values := url.Values{}

	upex.signParams(mapParams)
	for key, value := range mapParams {
		values.Add(key, value)
	}
	strParams = values.Encode()

	return strParams
}

func (upex *Upex) signParams(params map[string]string) {
	params["api_key"] = upex.apiKey
	params["time"] = fmt.Sprint(time.Now().Unix())

	keys := make([]string, 0, len(params))
	for k, _ := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	data := make([]string, 0, len(params))
	for _, k := range keys {
		data = append(data, fmt.Sprintf("%v%v", k, params[k]))
	}
	bt := strings.Join(data, "")
	bt += upex.secretKey

	h := md5.New()
	h.Write([]byte(bt))
	params["sign"] = hex.EncodeToString(h.Sum(nil))
}
