package aofex

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]string{
	KLINE_M1:   "1min",
	KLINE_M5:   "5min",
	KLINE_M15:  "15min",
	KLINE_M30:  "30min",
	KLINE_H1:   "hour",
	KLINE_DAY:  "1day",
	KLINE_WEEK: "1week",
}

type Aofex struct {
	httpClient *http.Client
	baseUrl    string
	token      string
	secretKey  string
}

/**
 * spot
 */
func NewSpotAPI(client *http.Client, token, secretkey string) SpotAPI {
	aofex := new(Aofex)
	aofex.baseUrl = "https://openapi.aofex.com/"
	aofex.httpClient = client
	aofex.token = token
	aofex.secretKey = secretkey
	return aofex
}

func (aofex *Aofex) GetExchangeName() string {
	return AOFEX
}

func (aofex *Aofex) SetURL(exurl string) {
	aofex.baseUrl = exurl
}

func (aofex *Aofex) GetURL() string {
	return aofex.baseUrl
}

func (aofex *Aofex) getDataMap(reqUrl string) (map[string]interface{}, error) {
	respmap, err := HttpGet(aofex.httpClient, reqUrl)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["errno"].(float64)
	if !ok {
		return nil, errors.New("errno assert error")
	}

	if code != 0 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["errmsg"])
	}

	datamap, ok := respmap["result"].(map[string]interface{})
	if !ok {
		return nil, errors.New("result assert error")
	}

	return datamap, nil
}

func (aofex *Aofex) getDataArray(reqUrl string) ([]interface{}, error) {
	respmap, err := HttpGet(aofex.httpClient, reqUrl)
	if err != nil {
		return nil, err
	}

	code, ok := respmap["errno"].(float64)
	if !ok {
		return nil, errors.New("errno assert error")
	}

	if code != 0 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["errmsg"])
	}

	dataArr, ok := respmap["result"].([]interface{})
	if !ok {
		return nil, errors.New("result assert error")
	}

	return dataArr, nil
}

func (aofex *Aofex) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	url := aofex.baseUrl + "openApi/market/symbols"
	dataArr, err := aofex.getDataArray(url)
	if err != nil {
		return nil, err
	}
	/*
		{
		      "id":1223,
		      "symbol": "BTC-USDT",
		      "base_currency": "BTC",
		      "quote_currency": "USDT",
		      "min_size": 0.0000001,
		      "max_size": 10000,
		      "min_price": 0.001,
		      "max_price":1000,
		      "maker_fee":0.002,
		      "taker_fee":0.002
		          },
	*/

	ssm := make(map[string]SymbolSetting)
	for _, v := range dataArr {
		obj := v.(map[string]interface{})
		symbol := strings.Replace(ToString(obj["symbol"]), "-", "/", -1)

		ssm[symbol] = SymbolSetting{
			Symbol:   symbol,
			Base:     strings.ToUpper(ToString(obj["base_currency"])),
			Quote:    strings.ToUpper(ToString(obj["quote_currency"])),
			MinSize:  ToFloat64(obj["min_size"]),
			MinPrice: ToFloat64(obj["min_price"]),
			//MinNotional: ToFloat64(obj["minTrade"]),
			MakerFee: ToFloat64(obj["maker_fee"]),
			TakerFee: ToFloat64(obj["taker_fee"]),
		}
	}

	return ssm, nil
}

func (aofex *Aofex) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	all, err := aofex.GetAllCurrencyStatus()
	if err == nil {
		return all[currency.Symbol()], nil
	}

	return CurrencyStatus{}, errors.New("Asset not found")
}

func (aofex *Aofex) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	ssm, err := aofex.GetAllCurrencyPair()
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

func (aofex *Aofex) parseTicker(pair CurrencyPair, tickmap map[string]interface{}, ts int64) (*Ticker, error) {
	ticker := new(Ticker)
	ticker.Market = pair
	ticker.Symbol = pair.ToLowerSymbol("/")
	ticker.Open = ToFloat64(tickmap["open"])
	ticker.Last = ToFloat64(tickmap["close"])
	ticker.High = ToFloat64(tickmap["high"])
	ticker.Low = ToFloat64(tickmap["low"])
	ticker.Vol = ToFloat64(tickmap["amount"])
	ticker.TS = ToInt64(tickmap["id"]) * 1000

	return ticker, nil
}

func (aofex *Aofex) GetTicker(pair CurrencyPair) (*Ticker, error) {
	url := aofex.baseUrl + "openApi/market/detail?symbol=" + pair.ToSymbol("-")

	datamap, err := aofex.getDataMap(url)
	if err != nil {
		return nil, err
	}

	return aofex.parseTicker(pair, datamap, time.Now().Unix())
}

func (aofex *Aofex) GetAllTicker() ([]Ticker, error) {
	url := aofex.baseUrl + "openApi/market/24kline"
	dataArr, err := aofex.getDataArray(url)
	if err != nil {
		return nil, err
	}

	ts := time.Now().Unix()
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

		tickmap, ok := obj["data"].(map[string]interface{})
		if !ok {
			Error("parse ticker failed: dataArr data assert failed")
			continue
		}

		pair := NewCurrencyPairFromString(strings.Replace(symbol, "-", "/", -1))
		t, err := aofex.parseTicker(pair, tickmap, ts)
		if err != nil {
			Error("parse ticker failed:%v", err)
			continue
		}
		tickers = append(tickers, *t)
	}

	return tickers, nil
}

func (aofex *Aofex) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	url := aofex.baseUrl + "openApi/market/depth?symbol=" + pair.ToSymbol("-")
	datamap, err := aofex.getDataMap(url)
	if err != nil {
		return nil, err
	}

	dep := parseDepthData(datamap)
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")

	return dep, nil
}

func (aofex *Aofex) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	url := aofex.baseUrl + "openApi/market/trade?symbol=" + pair.ToSymbol("-")
	datamap, err := aofex.getDataMap(url)
	if err != nil {
		return nil, err
	}

	dataArr, ok := datamap["data"].([]interface{})
	if !ok {
		return nil, errors.New("data assert error")
	}

	/*
	   "id": 17592256642623,
	              "amount": 0.04,
	              "price": 1997,
	              "direction": "buy",
	              "ts": 1502448920106
	*/
	trades := make([]Trade, 0, len(dataArr))
	for _, d := range dataArr {
		obj := d.(map[string]interface{})

		trades = append(trades, Trade{
			Tid:    ToInt64(obj["id"]),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Amount: ToFloat64(obj["amount"]),
			Price:  ToFloat64(obj["price"]),
			Side:   AdaptTradeSide(ToString(obj["direction"])),
			TS:     ToInt64(obj["ts"]) * 1000})
	}

	return trades, nil
}

//倒序
func (aofex *Aofex) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	periodS, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", aofex.GetExchangeName(), period)
	}
	url := aofex.baseUrl + "openApi/market/kline?symbol=%s&period=%v&size=%v"
	symbol := pair.ToSymbol("-")
	datamap, err := aofex.getDataMap(fmt.Sprintf(url, symbol, periodS, size))
	if err != nil {
		return nil, err
	}

	dataArr, ok := datamap["data"].([]interface{})
	if !ok {
		return nil, err
	}
	/*
	   "id": 1499184000,
	           "amount": 37593.0266,
	           "count": 0,
	           "open": 1935.2000,
	           "close": 1879.0000,
	           "low": 1856.0000,
	           "high": 1940.0000,
	           "vol": 71031537.97866500
	*/
	klines := make([]Kline, 0, len(dataArr))
	for i := len(dataArr) - 1; i >= 0; i-- {
		item := dataArr[i].(map[string]interface{})
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

func (aofex *Aofex) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := aofex.placeOrder(amount, price, pair, "buy-limit")
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

func (aofex *Aofex) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	orderId, err := aofex.placeOrder(amount, price, pair, "sell-limit")
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

func (aofex *Aofex) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := aofex.placeOrder(amount, "", pair, "buy-market")
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

func (aofex *Aofex) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	orderId, err := aofex.placeOrder(amount, "", pair, "sell-market")
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

func (aofex *Aofex) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	requrl := aofex.baseUrl + "openApi/entrust/cancel"
	params := map[string]string{}
	params["order_ids"] = orderId

	_, err := aofex.httpPost(requrl, params)
	if err != nil {
		return false, err
	}

	return true, nil
}

// 交易所有问题：一点都没成交或部分成交的订单，查询不到数据
func (aofex *Aofex) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	requrl := aofex.baseUrl + "openApi/entrust/detail"
	params := map[string]string{}
	params["order_sn"] = orderId

	respmap, err := aofex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	datamap, ok := respmap["result"].(map[string]interface{})
	if !ok {
		return nil, errors.New("result assert error")
	}

	entrust, ok := datamap["entrust"].(map[string]interface{})
	if !ok {
		return nil, errors.New("entrust assert error")
	}

	order := aofex.parseOrder(entrust, pair)
	return &order, nil
}

func (aofex *Aofex) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	requrl := aofex.baseUrl + "openApi/entrust/currentList"
	params := map[string]string{}
	params["symbol"] = pair.ToSymbol("-")
	params["limit"] = fmt.Sprint(100)

	respmap, err := aofex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	dataArr, ok := respmap["result"].([]interface{})
	if !ok {
		return nil, errors.New("result assert error")
	}

	orders := make([]Order, 0, len(dataArr))
	for _, v := range dataArr {
		obj, _ := v.(map[string]interface{})
		orders = append(orders, aofex.parseOrder(obj, pair))
	}

	return orders, nil
}

func (aofex *Aofex) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	requrl := aofex.baseUrl + "openApi/entrust/historyList"
	params := map[string]string{}
	params["symbol"] = pair.ToSymbol("-")
	params["limit"] = fmt.Sprint(100)

	respmap, err := aofex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	dataArr, ok := respmap["result"].([]interface{})
	if !ok {
		return nil, errors.New("result assert error")
	}

	orders := make([]Order, 0, len(dataArr))
	for _, v := range dataArr {
		obj, _ := v.(map[string]interface{})
		orders = append(orders, aofex.parseOrder(obj, pair))
	}

	return orders, nil
}

func (aofex *Aofex) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	requrl := aofex.baseUrl + "openApi/entrust/detail"
	params := map[string]string{}
	params["order_sn"] = orderId

	respmap, err := aofex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	datamap, ok := respmap["result"].(map[string]interface{})
	if !ok {
		return nil, errors.New("result assert error")
	}

	entrust, ok := datamap["entrust"].(map[string]interface{})
	if !ok {
		return nil, errors.New("entrust assert error")
	}

	order := aofex.parseOrder(entrust, pair)

	// 取消的订单，没有成交明细
	if order.Status == ORDER_CANCEL {
		return nil, nil
	}

	trades, ok := datamap["trades"].([]interface{})
	if !ok {
		return nil, errors.New("trades assert error")
	}

	return aofex.parseOrderDeal(orderId, order.Side, trades, pair)
}

func (aofex *Aofex) GetUserTrades(pair CurrencyPair) ([]Trade, error) {
	return nil, errors.New("exchange not supported yet")
}

func (aofex *Aofex) GetAccount() (*Account, error) {
	requrl := aofex.baseUrl + "openApi/wallet/list"
	params := map[string]string{}
	respmap, err := aofex.httpGet(requrl, params)
	if err != nil {
		return nil, err
	}

	dataArr, ok := respmap["result"].([]interface{})
	if !ok {
		return nil, errors.New("result assert error")
	}

	acc := new(Account)
	acc.SubAccounts = make(map[Currency]SubAccount)
	acc.Exchange = aofex.GetExchangeName()

	for _, v := range dataArr {
		/*
				 "currency": "BTC",
			      "available": "0.2323",
			      "frozen": "0"
		*/
		balancemap := v.(map[string]interface{})
		currencySymbol := balancemap["currency"].(string)
		currency := NewCurrency(currencySymbol)

		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(balancemap["available"]),
			FrozenAmount: ToFloat64(balancemap["frozen"]),
			LoanAmount:   0,
		}
	}

	return acc, nil
}

func (aofex *Aofex) placeOrder(amount, price string, pair CurrencyPair, orderType string) (string, error) {
	/*
		参数名	是否必需	类型	示例	说明
		symbol	是	string	BTC-USDT	交易对
		type	是	string	buy-limit	订单类型：buy-market：市价买, sell-market：市价卖, buy-limit：限价买, sell-limit：限价卖
		amount	是	float	1.343432	限价单表示下单数量，市价买单时表示买多少钱(usdt)，市价卖单时表示卖多少币(btc)
		price	否	float	32.3232145	下单价格，市价单不传该参数
	*/
	requrl := aofex.baseUrl + "openApi/entrust/add"
	params := map[string]string{}
	params["symbol"] = pair.ToSymbol("-")
	params["type"] = orderType
	params["amount"] = amount
	if strings.Contains(orderType, "limit") {
		params["price"] = price
	}

	respmap, err := aofex.httpPost(requrl, params)
	if err != nil {
		return "", err
	}

	datamap, ok := respmap["result"].(map[string]interface{})
	if !ok {
		return "", errors.New("result assert error")
	}

	return ToString(datamap["order_sn"]), nil
}

func (aofex *Aofex) parseOrder(ordmap map[string]interface{}, pair CurrencyPair) Order {
	/*
			"order_id":121,
			        "order_sn":"BL123456789987523",
			        "symbol":"MCO-BTC",
			        "ctime":"2018-10-02 10:33:33",
			        "type":"2",
			        "side":"buy",
			        "price":"0.123456",
			        "number":"1.0000",
			        "total_price":"0.123456",
			        "deal_number":"0.00000",
			        "deal_price":"0.00000",
			        "status":1


		参数名	是否必有	类型	示例	说明
		order_id	是	integer	 	订单id
		order_sn	是	string	 	订单编号
		symbol	是	string	 	交易对
		ctime	是	string	 	委托时间
		type	是	int	1	委托类型:1=市价,2=限价
		side	是	string	buy	委托方向:buy/sell
		price	是	String	 	价格:市价(type=1)时为0
		number	是	String	3.12	委托数量:市价买入(type=1，side=buy)时为0
		total_price	是	String	 	委托总额:市价卖出(type=1，side=sell)时为0
		deal_number	是	String	 	已成交数量
		deal_price	是	String	 	已成交均价
		status	是	int	1	状态:1=挂单中,2=部分成交,3=已成交,4=撤销中,5=部分撤销,6=已撤销
	*/
	// TODO 市价的amount和price目前会有问题

	//fmt.Printf("ordmap1:%+v\n", ordmap)
	//fmt.Printf("ordmap2:%#v\n", ordmap)

	ord := Order{
		OrderID:    ToString(ordmap["order_sn"]),
		Amount:     ToFloat64(ordmap["number"]),
		AvgPrice:   ToFloat64(ordmap["deal_price"]),
		Price:      ToFloat64(ordmap["price"]),
		DealAmount: ToFloat64(ordmap["deal_number"]),
	}

	date, err := time.ParseInLocation("2006-01-02 15:04:05", ToString(ordmap["ctime"]), time.Local)
	//date, err := time.Parse("2006-01-02 15:04:05", ToString(ordmap["ctime"]))
	if err == nil {
		ord.TS = date.Unix() * 1000
	}

	state := ordmap["status"].(float64)
	switch state {
	case 1:
		ord.Status = ORDER_UNFINISH
	case 2, 5:
		ord.Status = ORDER_PART_FINISH
	case 3:
		ord.Status = ORDER_FINISH
	case 4, 6:
		ord.Status = ORDER_CANCEL
		// 注意：取消的订单（一点都没成交），返回的成交价和成交量，等于下单价和下单量，这个是有问题的，需要手动置0
		ord.DealAmount = 0
		ord.AvgPrice = 0
	default:
		ord.Status = ORDER_UNFINISH
	}

	// 委托类型:1=市价,2=限价
	tradeType, _ := ordmap["type"].(float64)
	// 委托方向:buy/sell
	side := ordmap["side"].(string)
	switch side {
	case "sell":
		if tradeType == 1 {
			ord.Side = SELL_MARKET
			ord.Price = ord.AvgPrice
			ord.Amount = ord.DealAmount
		} else {
			ord.Side = SELL
		}
	case "buy":
		if tradeType == 1 {
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

func (aofex *Aofex) parseOrderDeal(orderId string, side TradeSide, trades []interface{}, pair CurrencyPair) ([]OrderDeal, error) {
	/*
					"trades":[{
			        "id": 18,
			        "ctime":"2018-10-02 10:33:33",
			        "price":"0.123456",
			        "number":"1.0000",
			        "total_price":"0.123456",
			        "fee":"0.00001",
			          }


				参数名	是否必有	类型	示例	说明
			id	是	int	11	成交id
			ctime	是	string	 	成交时间
			price	是	string	 	价格
			number	是	string	 	数量
			total_price	是	string	 	成交额
			fee	是	string	 	手续费

		OrderID          string       `json:"order_id"`                  // 订单id
			DealID           string       `json:"deal_id"`                   // 本次成交id
			TS               int64        `json:"ts"`                        // 时间，单位为毫秒(millisecond)
			Price            float64      `json:"price,string"`              // 委托价格
			FilledAmount     float64      `json:"filled_amount,string"`      // 本次成交量
			FilledCashAmount float64      `json:"filled_cash_amount,string"` // 本次成交金额
			UnFilledAmount   float64      `json:"unfilled_amount,string"`    // 未成交的量
			Side             TradeSide    `json:"side"`                      // 交易方向
			Market           CurrencyPair `json:"market"`                    // 交易对
			Symbol           string       `json:"symbol"`                    // 交易对
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
			FilledAmount:     ToFloat64(dealMap["number"]),
			FilledCashAmount: ToFloat64(dealMap["total_price"]),
			Side:             side,
			Market:           pair,
			Symbol:           pair.ToLowerSymbol("/"),
		}
		date, err := time.ParseInLocation("2006-01-02 15:04:05", ToString(dealMap["ctime"]), time.Local)
		if err == nil {
			deal.TS = date.Unix() * 1000
		}
		// 测试发现，没有dealID，只好用下面的来做dealid
		if len(deal.DealID) <= 1 {
			h := md5.New()
			h.Write([]byte(fmt.Sprintf("%v-%v-%v-%v", deal.OrderID, deal.TS, deal.Price, deal.FilledAmount)))
			deal.DealID = hex.EncodeToString(h.Sum(nil))
		}

		deals = append(deals, deal)
	}

	return deals, nil
}

func parseDepthData(tick map[string]interface{}) *Depth {
	bids, _ := tick["bids"].([]interface{})
	asks, _ := tick["asks"].([]interface{})

	depth := new(Depth)
	depth.TS = ToInt64(tick["ts"])

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

func (aofex *Aofex) buildHeaders(params map[string]string) map[string]string {
	/*
		$header[] = 'Nonce: 1534927978_ab43c';
		$header[] = 'Token: 57ba172a6be125cca2f449826f9980ca';
		$header[] = 'Signature: v490hupi0s0bckcp6ivb69p921';
	*/

	nonce := genNonce()
	headers := make(map[string]string)
	headers["Nonce"] = nonce
	headers["Token"] = aofex.token
	headers["Signature"] = aofex.sign(nonce, params)

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

func (aofex *Aofex) sign(nonce string, params map[string]string) string {
	data := make([]string, 0)
	data = append(data, aofex.token)
	data = append(data, aofex.secretKey)
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

func (aofex *Aofex) httpPost(requrl string, params map[string]string) (map[string]interface{}, error) {
	var strRequestUrl string
	if nil == params {
		strRequestUrl = requrl
	} else {
		strRequestUrl = requrl + "?" + map2UrlQuery(params)
	}

	respData, err := HttpPostForm4(aofex.httpClient, strRequestUrl, params, aofex.buildHeaders(params))
	if err != nil {
		return nil, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(respData, &respmap)
	if err != nil {
		log.Printf("json.Unmarshal failed : %v, resp %s\n", err, string(respData))
		return nil, err
	}

	code, ok := respmap["errno"].(float64)
	if !ok {
		return nil, errors.New("errno assert error")
	}

	if code != 0 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["errmsg"])
	}

	return respmap, nil
}

func (aofex *Aofex) httpGet(requrl string, params map[string]string) (map[string]interface{}, error) {
	var strRequestUrl string
	if nil == params {
		strRequestUrl = requrl
	} else {
		strRequestUrl = requrl + "?" + map2UrlQuery(params)
	}

	respmap, err := HttpGet2(aofex.httpClient, strRequestUrl, aofex.buildHeaders(params))
	if err != nil {
		return nil, err
	}

	code, ok := respmap["errno"].(float64)
	if !ok {
		return nil, errors.New("errno assert error")
	}

	if code != 0 {
		return nil, fmt.Errorf("code:%v,msg:%v", code, respmap["errmsg"])
	}

	return respmap, nil
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
