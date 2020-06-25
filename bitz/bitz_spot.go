package bitz

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]string{
	KLINE_M1:    "1min",
	KLINE_M5:    "5min",
	KLINE_M15:   "15min",
	KLINE_M30:   "30min",
	KLINE_H1:    "60min",
	KLINE_H4:    "4hour",
	KLINE_DAY:   "1day",
	KLINE_WEEK:  "1week",
	KLINE_MONTH: "1mon",
}

type Bitz struct {
	httpClient *http.Client
	baseUrl    string
	accessKey  string
	secretKey  string
	tradePWD   string // 交易密码，币币交易的限价单需要
}

/**
 * spot
 */
func NewSpotAPI(client *http.Client, apikey, secretkey string) SpotAPI {
	bitz := new(Bitz)
	bitz.baseUrl = "https://api.bitzspeed.com/"
	bitz.httpClient = client
	bitz.accessKey = apikey
	bitz.secretKey = secretkey
	return bitz
}

func (bitz *Bitz) GetExchangeName() string {
	return BITZ
}

func (bitz *Bitz) SetURL(exurl string) {
	bitz.baseUrl = exurl
}

func (bitz *Bitz) GetURL() string {
	return bitz.baseUrl
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

func (bitz *Bitz) GetTradeFee(symbols string) (tf *TradeFee, err error) {
	return &TradeFee{
		Symbol:          symbols,
		ActualMakerRate: 0.002,
		ActualTakerRate: 0.002,
	}, nil
}

func (bitz *Bitz) GetTradeFeeMap() (tfmap map[string]TradeFee, err error) {
	ssm, err := bitz.GetAllCurrencyPair()
	if err != nil {
		return nil, err
	}

	tfmap = make(map[string]TradeFee)
	for k, v := range ssm {
		tfmap[k] = TradeFee{
			Symbol:          k,
			ActualMakerRate: v.MakerFee,
			ActualTakerRate: v.TakerFee,
		}
	}

	return tfmap, nil
}

func (bitz *Bitz) getDataMap(reqUrl string) (map[string]interface{}, error) {
	respmap, err := HttpGet(bitz.httpClient, reqUrl)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	return datamap, nil
}

func (bitz *Bitz) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	url := bitz.baseUrl + "Market/symbolList"
	datamap, err := bitz.getDataMap(url)
	if err != nil {
		return nil, err
	}

	// TODO FIXME: 用一个就可以了，反正写死的
	tf, err := bitz.GetTradeFee("btc_usdt")
	if err != nil {
		tf = &TradeFee{
			ActualMakerRate: 0.002,
			ActualTakerRate: 0.002,
		}
	}

	ssm := make(map[string]SymbolSetting)
	for k, v := range datamap {
		obj := v.(map[string]interface{})
		symbol := k

		ssm[symbol] = SymbolSetting{
			Symbol:      symbol,
			Base:        strings.ToUpper(ToString(obj["coinFrom"])),
			Quote:       strings.ToUpper(ToString(obj["coinTo"])),
			MinSize:     math.Pow10(-ToInt(obj["numberFloat"])),
			MinPrice:    math.Pow10(-ToInt(obj["priceFloat"])),
			MinNotional: ToFloat64(obj["minTrade"]),
			MakerFee:    tf.ActualMakerRate,
			TakerFee:    tf.ActualTakerRate,
		}
	}

	return ssm, nil
}

func (bitz *Bitz) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	all, err := bitz.GetAllCurrencyStatus()
	if err == nil {
		return all[currency.Symbol()], nil
	}

	return CurrencyStatus{}, errors.New("Asset not found")
}

func (bitz *Bitz) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	ssm, err := bitz.GetAllCurrencyPair()
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

func (bitz *Bitz) parseTicker(pair CurrencyPair, tickmap map[string]interface{}, ts int64) (*Ticker, error) {
	ticker := new(Ticker)
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	ticker.Open = ToFloat64(tickmap["open"])
	ticker.Last = ToFloat64(tickmap["now"])
	ticker.High = ToFloat64(tickmap["high"])
	ticker.Low = ToFloat64(tickmap["low"])
	ticker.Vol = ToFloat64(tickmap["volume"])
	ticker.Buy = ToFloat64(tickmap["bidPrice"])
	ticker.Sell = ToFloat64(tickmap["askPrice"])
	ticker.TS = ts

	return ticker, nil
}

func parseMicroTime(respmap map[string]interface{}) (ts int64, err error) {
	//     "microtime": "0.23065700 1532671288",
	microtime, ok := respmap["microtime"].(string)
	if !ok {
		return 0, errors.New("invalid microtime")
	}
	fs := strings.Split(microtime, " ")
	if len(fs) != 2 {
		return 0, errors.New("invalid microtime")
	}
	m, _ := strconv.ParseFloat(fs[0], 64)
	s, _ := strconv.ParseFloat(fs[1], 64)
	ts = int64((s + m) * 1000)
	return ts, nil
}

func (bitz *Bitz) GetTicker(pair CurrencyPair) (*Ticker, error) {
	url := bitz.baseUrl + "Market/ticker?symbol=" + pair.ToLowerSymbol("_")
	respmap, err := HttpGet(bitz.httpClient, url)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	ts, err := parseMicroTime(respmap)
	if err != nil {
		return nil, err
	}

	return bitz.parseTicker(pair, datamap, ts)
}

func (bitz *Bitz) GetAllTicker() ([]Ticker, error) {
	url := bitz.baseUrl + "Market/tickerall"
	respmap, err := HttpGet(bitz.httpClient, url)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	ts, err := parseMicroTime(respmap)
	if err != nil {
		return nil, err
	}

	tickers := make([]Ticker, 0, len(datamap))
	for k, v := range datamap {
		tickmap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		pair := NewCurrencyPairFromString(strings.Replace(k, "_", "/", -1))
		t, err := bitz.parseTicker(pair, tickmap, ts)
		if err != nil {
			Error("parse ticker failed:%v", err)
			continue
		}
		tickers = append(tickers, *t)
	}

	return tickers, nil
}

func (bitz *Bitz) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	url := bitz.baseUrl + "Market/depth?symbol=" + pair.ToLowerSymbol("_")
	respmap, err := HttpGet(bitz.httpClient, url)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	ts, err := parseMicroTime(respmap)
	if err != nil {
		return nil, err
	}

	dep := parseDepthData(datamap)
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")
	dep.TS = ts

	return dep, nil
}

func (bitz *Bitz) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	url := bitz.baseUrl + "Market/order?symbol=" + pair.ToLowerSymbol("_")
	respmap, err := HttpGet(bitz.httpClient, url)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	dataArr, ok := respmap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	/*
	   "id": 105526523,
	   "t": "12:58:48",  //时间
	   "T": 1535796654,
	   "p": "0.01053000",  //价格
	   "n": "11.9096", //数量
	   "s": "buy"      //类型
	*/
	trades := make([]Trade, 0, len(dataArr))
	for _, d := range dataArr {
		obj := d.(map[string]interface{})

		trades = append(trades, Trade{
			Tid:    ToInt64(obj["id"]),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Amount: ToFloat64(obj["n"]),
			Price:  ToFloat64(obj["p"]),
			Side:   AdaptTradeSide(ToString(obj["s"])),
			TS:     ToInt64(obj["T"]) * 1000})
	}

	return trades, nil
}

//倒序
func (bitz *Bitz) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", bitz.GetExchangeName(), period)
	}
	url := bitz.baseUrl + "Market/kline?symbol=%s&resolution=%v&size=%v"
	symbol := pair.ToLowerSymbol("_")
	datamap, err := bitz.getDataMap(fmt.Sprintf(url, symbol, periodS, size))
	if err != nil {
		return nil, err
	}

	bars, ok := datamap["bars"].([]interface{})
	if !ok {
		return nil, errors.New("response format error")
	}

	var klines []Kline
	for i := len(bars) - 1; i >= 0; i-- {
		item := bars[i].(map[string]interface{})
		klines = append(klines, Kline{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Open:   ToFloat64(item["open"]),
			Close:  ToFloat64(item["close"]),
			High:   ToFloat64(item["high"]),
			Low:    ToFloat64(item["low"]),
			Vol:    ToFloat64(item["volume"]),
			TS:     ToInt64(item["time"])})
	}
	return klines, nil
}

func (bitz *Bitz) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := bitz.placeLimitOrder(amount, price, pair, "1")
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

func (bitz *Bitz) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := bitz.placeLimitOrder(amount, price, pair, "2")
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

func (bitz *Bitz) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := bitz.placeMarketOrder(amount, pair, "1")
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

func (bitz *Bitz) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := bitz.placeMarketOrder(amount, pair, "2")
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

func (bitz *Bitz) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	requrl := bitz.baseUrl + "Trade/cancelEntrustSheet"
	params := map[string]string{}
	params["entrustSheetId"] = orderId

	respmap, err := bitz.httpPostRequest(requrl, params)
	if err != nil {
		return false, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return false, errors.New("status assert error")
	}

	if code != 200 {
		return false, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	return true, nil
}

func (bitz *Bitz) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	requrl := bitz.baseUrl + "Trade/getEntrustSheetInfo"
	params := map[string]string{}
	params["entrustSheetId"] = orderId

	respmap, err := bitz.httpPostRequest(requrl, params)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	order := bitz.parseOrder(datamap, pair)
	return &order, nil
}

func (bitz *Bitz) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	requrl := bitz.baseUrl + "Trade/getUserNowEntrustSheet"
	params := map[string]string{}
	params["coinFrom"] = pair.Stock.LowerSymbol()
	params["coinTo"] = pair.Money.LowerSymbol()
	params["pageSize"] = fmt.Sprint(100)

	respmap, err := bitz.httpPostRequest(requrl, params)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	dataArr, ok := datamap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}
	orders := make([]Order, 0, len(dataArr))
	for _, v := range dataArr {
		obj, _ := v.(map[string]interface{})
		orders = append(orders, bitz.parseOrder(obj, pair))
	}

	return orders, nil
}

func (bitz *Bitz) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	requrl := bitz.baseUrl + "Trade/getUserHistoryEntrustSheet"
	params := map[string]string{}
	params["coinFrom"] = pair.Stock.LowerSymbol()
	params["coinTo"] = pair.Money.LowerSymbol()
	params["pageSize"] = fmt.Sprint(100)

	respmap, err := bitz.httpPostRequest(requrl, params)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	dataArr, ok := datamap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}
	orders := make([]Order, 0, len(dataArr))
	for _, v := range dataArr {
		obj, _ := v.(map[string]interface{})
		orders = append(orders, bitz.parseOrder(obj, pair))
	}

	return orders, nil
}

func (bitz *Bitz) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	return nil, errors.New("exchange not supported yet")
}

func (bitz *Bitz) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	return nil, errors.New("exchange not supported yet")
}

func (bitz *Bitz) GetAccount() (*Account, error) {
	requrl := bitz.baseUrl + "Assets/getUserAssets"
	params := map[string]string{}

	respmap, err := bitz.httpPostRequest(requrl, params)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return nil, errors.New("status assert error")
	}

	if code != 200 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	acc := new(Account)
	acc.SubAccounts = make(map[Currency]SubAccount)
	acc.Exchange = bitz.GetExchangeName()

	list := datamap["info"].([]interface{})
	for _, v := range list {
		/*
			"name": "zpr",      //币种名称
			                   "num": "37.49067275",  //数量
			                   "over": "37.49067275", //可用
			                   "lock": "0.00000000",  //冻结
		*/
		balancemap := v.(map[string]interface{})
		currencySymbol := balancemap["name"].(string)
		currency := NewCurrency(currencySymbol)

		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(balancemap["over"]),
			FrozenAmount: ToFloat64(balancemap["lock"]),
			LoanAmount:   0,
		}
	}

	return acc, nil
}

func (bitz *Bitz) placeLimitOrder(amount, price string, pair CurrencyPair, orderType string) (string, error) {
	/*
		type	是	string	购买类型 1买进 2 卖出
		price	是	float	委托价格
		number	是	float	委托数量
		symbol	是	string	eth_btc、ltc_btc 交易对
		tradePwd	是	string	交易密码
	*/
	requrl := bitz.baseUrl + "Trade/addEntrustSheet"
	params := map[string]string{}
	params["type"] = orderType
	params["price"] = price
	params["number"] = amount
	params["symbol"] = pair.ToLowerSymbol("_")
	//params["tradePwd"] = bitz.tradePWD
	params["tradePwd"] = "nhbitzJY2020"

	respmap, err := bitz.httpPostRequest(requrl, params)
	if err != nil {
		return "", err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return "", errors.New("status assert error")
	}

	if code != 200 {
		return "", fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return "", errors.New("data assert error")
	}

	return fmt.Sprint(ToInt64(datamap["id"])), nil
}

func (bitz *Bitz) placeMarketOrder(amount string, pair CurrencyPair, orderType string) (string, error) {
	/*
		symbol	是	string	交易对名称
		total	是	string	买时传入金额，卖时传入数量
		type	是	int	类型 1 买 2 卖
	*/
	requrl := bitz.baseUrl + "Trade/MarketTrade"
	params := map[string]string{}
	params["symbol"] = pair.ToLowerSymbol("_")
	params["total"] = amount
	params["type"] = orderType

	respmap, err := bitz.httpPostRequest(requrl, params)
	if err != nil {
		return "", err
	}

	code, ok := respmap["status"].(float64)
	if !ok {
		return "", errors.New("status assert error")
	}

	if code != 200 {
		return "", fmt.Errorf("code:%v,msg:%v", code, respmap["msg"])
	}

	datamap, ok := respmap["data"].(map[string]interface{})
	if !ok {
		return "", errors.New("data assert error")
	}

	return fmt.Sprint(ToInt64(datamap["id"])), nil
}

func (bitz *Bitz) parseOrder(ordmap map[string]interface{}, pair CurrencyPair) Order {
	// TODO 市价的amount和price目前会有问题
	ord := Order{
		OrderID:    ToString(ordmap["id"]),
		Amount:     ToFloat64(ordmap["number"]),
		Price:      ToFloat64(ordmap["price"]),
		DealAmount: ToFloat64(ordmap["numberDeal"]),
		TS:         ToInt64(ordmap["created"]) * 1000,
	}

	state := ordmap["status"].(float64)
	switch state {
	case 0:
		ord.Status = ORDER_UNFINISH
	case 1:
		ord.Status = ORDER_PART_FINISH
	case 2:
		ord.Status = ORDER_FINISH
	case 3:
		ord.Status = ORDER_CANCEL
	default:
		ord.Status = ORDER_UNFINISH
	}

	// 当前委托，历史委托，无此字段，默认用价格
	if _, ok := ordmap["averagePrice"]; ok {
		ord.AvgPrice = ToFloat64(ordmap["averagePrice"])
	} else {
		ord.AvgPrice = ord.Price
	}

	// 当前委托查询无此字段，默认为限价
	tradeType, _ := ordmap["tradeType"].(string)
	typeS := ordmap["flag"].(string)
	switch typeS {
	case "sale":
		if tradeType == "2" {
			ord.Side = SELL_MARKET
		} else {
			ord.Side = SELL
		}
	case "buy":
		if tradeType == "2" {
			ord.Side = BUY_MARKET
		} else {
			ord.Side = BUY
		}
	}

	ord.Market = pair
	ord.Symbol = pair.ToLowerSymbol("/")
	return ord
}

func (bitz *Bitz) parsePendingOrder(ordmap map[string]interface{}, pair CurrencyPair) Order {
	/*
			"id":"708279852",             //订单号
		                "uid":"2074056",
		                "price":"100.00000000",       //委托价格
		                "number":"10.0000",           //委托数量
		                "total":"0.0000000000000000", //委托总价格
		                "numberOver":"10.0000",       //剩余数量
		                "numberDeal":"0.0000",        //成交数量
		                "flag":"sale",                //买卖类型
		                "status":0, //0:未成交, 1:部分成交, 2:全部成交, 3:已经撤销
		                "isNew":"N",                   //新委托
		                "coinFrom":"bz",               //要兑换的币
		                "coinTo":"usdt",               //目标兑换
		                "created":"1533279876"
	*/
	ord := Order{
		OrderID:    ToString(ordmap["id"]),
		Amount:     ToFloat64(ordmap["number"]),
		Price:      ToFloat64(ordmap["price"]),
		DealAmount: ToFloat64(ordmap["numberDeal"]),
		TS:         ToInt64(ordmap["created"]) * 1000,
	}

	state := ordmap["status"].(float64)
	switch state {
	case 0:
		ord.Status = ORDER_UNFINISH
	case 1:
		ord.Status = ORDER_PART_FINISH
	case 2:
		ord.Status = ORDER_FINISH
	case 3:
		ord.Status = ORDER_CANCEL
	default:
		ord.Status = ORDER_UNFINISH
	}

	ord.AvgPrice = ToFloat64(ordmap["averagePrice"])

	typeS := ordmap["flag"].(string)
	switch typeS {
	case "sale":
		ord.Side = SELL
	case "buy":
		ord.Side = BUY
	}

	ord.Market = pair
	ord.Symbol = pair.ToLowerSymbol("/")
	return ord
}

func parseDepthData(tick map[string]interface{}) *Depth {
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

// Http Get请求基础函数, 通过封装Go语言Http请求, 支持火币网REST API的HTTP Get请求
// strUrl: 请求的URL
// strParams: string类型的请求参数, user=lxz&pwd=lxz
// return: 请求结果
func (bitz *Bitz) httpGetRequest(strUrl string, mapParams map[string]string) string {
	var strRequestUrl string
	if nil == mapParams {
		strRequestUrl = strUrl
	} else {
		strParams := map2UrlQuery(mapParams)
		strRequestUrl = strUrl + "?" + strParams
	}

	// 构建Request, 并且按官方要求添加Http Header
	request, err := http.NewRequest("GET", strRequestUrl, nil)
	if nil != err {
		return err.Error()
	}
	request.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.71 Safari/537.36")

	// 发出请求
	response, err := bitz.httpClient.Do(request)
	defer response.Body.Close()
	if nil != err {
		return err.Error()
	}

	// 解析响应内容
	body, err := ioutil.ReadAll(response.Body)
	if nil != err {
		return err.Error()
	}

	return string(body)
}

// Http POST请求基础函数, 通过封装Go语言Http请求, 支持火币网REST API的HTTP POST请求
// strUrl: 请求的URL
// mapParams: map类型的请求参数
// return: 请求结果
func (bitz *Bitz) httpPostRequest(strUrl string, mapParams map[string]string) (map[string]interface{}, error) {
	mapParams["apiKey"] = bitz.accessKey
	mapParams["timeStamp"] = strconv.FormatInt(time.Now().Unix(), 10)
	mapParams["nonce"] = genValidateCode(6)
	mapParams["sign"] = bitzSign(mapParams, bitz.secretKey)

	queryStr := map2UrlQuery(mapParams)

	respData, err := HttpPostForm3(bitz.httpClient, strUrl, queryStr, map[string]string{"Content-Type": "application/x-www-form-urlencoded;charset=UTF-8"})
	if err != nil {
		return nil, err
	}

	var bodyDataMap map[string]interface{}
	err = json.Unmarshal(respData, &bodyDataMap)
	if err != nil {
		log.Printf("json.Unmarshal failed : %v, resp %s\n", err, string(respData))
		return nil, err
	}

	return bodyDataMap, nil
}

func genValidateCode(width int) string {
	numeric := [10]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	r := len(numeric)
	rand.Seed(time.Now().UnixNano())

	var sb strings.Builder
	for i := 0; i < width; i++ {
		fmt.Fprintf(&sb, "%d", numeric[rand.Intn(r)])
	}
	return sb.String()
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

func bitzSign(mapParams map[string]string, secretKey string) string {
	var keys []string
	for key := range mapParams {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var queryStr string
	for _, k := range keys {
		queryStr += k + "=" + mapParams[k] + "&"
	}

	queryStr = queryStr[:len(queryStr)-1]
	queryStr += secretKey

	h := md5.New()
	h.Write([]byte(queryStr))
	sign := hex.EncodeToString(h.Sum(nil))
	return sign
}
