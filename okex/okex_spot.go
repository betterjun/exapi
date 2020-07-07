package okex

import (
	"bytes"

	"errors"
	"fmt"
	. "github.com/betterjun/exapi"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"net/http"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]int{
	KLINE_M1:   60,
	KLINE_M5:   300,
	KLINE_M15:  900,
	KLINE_M30:  1800,
	KLINE_H1:   3600,
	KLINE_H4:   14400,
	KLINE_DAY:  86400,
	KLINE_WEEK: 604800,
	//KLINE_MONTH: "1M",
}

type OKExSpot struct {
	HttpClient    *http.Client
	Endpoint      string
	ApiKey        string
	ApiSecretKey  string
	ApiPassphrase string //for okex.com v3 api
}

type placeOrderParam struct {
	ClientOid     string  `json:"client_oid"`
	Type          string  `json:"type"`
	Side          string  `json:"side"`
	InstrumentId  string  `json:"instrument_id"`
	OrderType     int     `json:"order_type"`
	Price         float64 `json:"price"`
	Size          float64 `json:"size"`
	Notional      float64 `json:"notional"`
	MarginTrading string  `json:"margin_trading,omitempty"`
}

type placeOrderResponse struct {
	OrderId      string `json:"order_id"`
	ClientOid    string `json:"client_oid"`
	Result       bool   `json:"result"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

const (
	/*
	  http headers
	*/
	OK_ACCESS_KEY        = "OK-ACCESS-KEY"
	OK_ACCESS_SIGN       = "OK-ACCESS-SIGN"
	OK_ACCESS_TIMESTAMP  = "OK-ACCESS-TIMESTAMP"
	OK_ACCESS_PASSPHRASE = "OK-ACCESS-PASSPHRASE"

	/**
	  paging params
	*/
	OK_FROM  = "OK-FROM"
	OK_TO    = "OK-TO"
	OK_LIMIT = "OK-LIMIT"

	CONTENT_TYPE = "Content-Type"
	ACCEPT       = "Accept"
	COOKIE       = "Cookie"
	LOCALE       = "locale="

	APPLICATION_JSON      = "application/json"
	APPLICATION_JSON_UTF8 = "application/json; charset=UTF-8"

	/*
	  i18n: internationalization
	*/
	ENGLISH            = "en_US"
	SIMPLIFIED_CHINESE = "zh_CN"
	//zh_TW || zh_HK
	TRADITIONAL_CHINESE = "zh_HK"

	/*
	  http methods
	*/
	GET    = "GET"
	POST   = "POST"
	DELETE = "DELETE"

	/*
	 others
	*/
	ResultDataJsonString = "resultDataJsonString"
	ResultPageJsonString = "resultPageJsonString"

	BTC_USD_SWAP = "BTC-USD-SWAP"
	LTC_USD_SWAP = "LTC-USD-SWAP"
	ETH_USD_SWAP = "ETH-USD-SWAP"
	ETC_USD_SWAP = "ETC-USD-SWAP"
	BCH_USD_SWAP = "BCH-USD-SWAP"
	BSV_USD_SWAP = "BSV-USD-SWAP"
	EOS_USD_SWAP = "EOS-USD-SWAP"
	XRP_USD_SWAP = "XRP-USD-SWAP"

	/*Rest Endpoint*/
	Endpoint              = "https://www.okex.com"
	GET_ACCOUNTS          = "/api/swap/v3/accounts"
	PLACE_ORDER           = "/api/swap/v3/order"
	CANCEL_ORDER          = "/api/swap/v3/cancel_order/%s/%s"
	GET_ORDER             = "/api/swap/v3/orders/%s/%s"
	GET_POSITION          = "/api/swap/v3/%s/position"
	GET_DEPTH             = "/api/swap/v3/instruments/%s/depth?size=%d"
	GET_TICKER            = "/api/swap/v3/instruments/%s/ticker"
	GET_ALL_TICKER        = "/api/swap/v3/instruments/ticker"
	GET_UNFINISHED_ORDERS = "/api/swap/v3/orders/%s?status=%d&limit=%d"
)

func NewSpotAPI(client *http.Client, apiKey, secretKey, apiPass string) SpotAPI {
	okex := &OKExSpot{
		HttpClient:    client,
		Endpoint:      "https://www.okex.com",
		ApiKey:        apiKey,
		ApiSecretKey:  secretKey,
		ApiPassphrase: apiPass,
	}
	return okex
}

func (ok *OKExSpot) GetExchangeName() string {
	return OKEX
}

func (ok *OKExSpot) SetURL(exurl string) {
	ok.Endpoint = exurl
}

func (ok *OKExSpot) GetURL() string {
	return ok.Endpoint
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

func (ok *OKExSpot) GetTradeFee() (tf *TradeFee, err error) {
	urlPath := "/api/spot/v3/trade_fee"

	tf = &TradeFee{}
	err = ok.doRequest("GET", urlPath, "", tf)
	if err != nil {
		return nil, err
	}
	return tf, nil
}

func (ok *OKExSpot) GetAllCurrencyPair() (map[string]SymbolSetting, error) {
	urlPath := "/api/spot/v3/instruments"
	var response []struct {
		InstrumentId  string `json:"instrument_id"`
		BaseCurrency  string `json:"base_currency"`
		QuoteCurrency string `json:"quote_currency"`
		MinSize       string `json:"min_size"`
		SizeIncrement string `json:"size_increment"`
		TickSize      string `json:"tick_size"`
	}
	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	tf, err := ok.GetTradeFee()
	if err != nil {
		tf = &TradeFee{
			Maker: 0.001,
			Taker: 0.0015,
		}
	}
	ssm := make(map[string]SymbolSetting)
	for _, v := range response {
		symbol := strings.Replace(v.InstrumentId, "-", "/", -1)
		ssm[symbol] = SymbolSetting{
			Symbol:      symbol,
			Base:        v.BaseCurrency,
			Quote:       v.QuoteCurrency,
			MinSize:     ToFloat64(v.SizeIncrement),
			MinPrice:    ToFloat64(v.TickSize),
			MinNotional: ToFloat64(v.MinSize),
			MakerFee:    tf.Maker,
			TakerFee:    tf.Taker,
		}
	}

	return ssm, nil
}

func (ok *OKExSpot) GetCurrencyStatus(currency Currency) (CurrencyStatus, error) {
	urlPath := "/api/account/v3/currencies"
	var response []struct {
		Currency    string `json:"currency"`
		CanDeposit  string `json:"can_deposit"`  // 是否可充值，0表示不可充值，1表示可以充值
		CanWithdraw string `json:"can_withdraw"` // 是否可提币，0表示不可提币，1表示可以提币
	}

	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return CurrencyStatus{}, err
	}

	for _, v := range response {
		if v.Currency == currency.Symbol() {
			return CurrencyStatus{
				Deposit:  ToBool(v.CanDeposit),
				Withdraw: ToBool(v.CanWithdraw),
			}, nil
		}
	}

	return CurrencyStatus{}, errors.New("Asset not found")
}

func (ok *OKExSpot) GetAllCurrencyStatus() (all map[string]CurrencyStatus, err error) {
	urlPath := "/api/account/v3/currencies"
	var response []struct {
		Currency    string `json:"currency"`
		CanDeposit  string `json:"can_deposit"`  // 是否可充值，0表示不可充值，1表示可以充值
		CanWithdraw string `json:"can_withdraw"` // 是否可提币，0表示不可提币，1表示可以提币
	}

	err = ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	all = make(map[string]CurrencyStatus)
	for _, v := range response {
		all[v.Currency] = CurrencyStatus{
			Deposit:  ToBool(v.CanDeposit),
			Withdraw: ToBool(v.CanWithdraw),
		}
	}

	return all, nil
}

func (ok *OKExSpot) GetTicker(pair CurrencyPair) (*Ticker, error) {
	urlPath := fmt.Sprintf("/api/spot/v3/instruments/%s/ticker", pair.ToSymbol("-"))
	var response struct {
		Open24h       float64 `json:"open_24h,string"`
		Last          float64 `json:"last,string"`
		High24h       float64 `json:"high_24h,string"`
		Low24h        float64 `json:"low_24h,string"`
		BestBid       float64 `json:"best_bid,string"`
		BestAsk       float64 `json:"best_ask,string"`
		BaseVolume24h float64 `json:"base_volume_24h,string"`
		Timestamp     string  `json:"timestamp"`
	}
	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	date, _ := time.Parse(time.RFC3339, response.Timestamp)
	return &Ticker{
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Open:   response.Open24h,
		Last:   response.Last,
		High:   response.High24h,
		Low:    response.Low24h,
		Vol:    response.BaseVolume24h,
		Buy:    response.BestBid,
		Sell:   response.BestAsk,
		TS:     int64(time.Duration(date.UnixNano() / int64(time.Millisecond)))}, nil
}

func (ok *OKExSpot) GetAllTicker() ([]Ticker, error) {
	urlPath := fmt.Sprintf("/api/spot/v3/instruments/ticker")
	type response struct {
		InstrumentID  string  `json:"instrument_id"`
		Open24h       float64 `json:"open_24h,string"`
		Last          float64 `json:"last,string"`
		High24h       float64 `json:"high_24h,string"`
		Low24h        float64 `json:"low_24h,string"`
		BestBid       float64 `json:"best_bid,string"`
		BestAsk       float64 `json:"best_ask,string"`
		BaseVolume24h float64 `json:"base_volume_24h,string"`
		Timestamp     string  `json:"timestamp"`
	}
	responses := make([]response, 0)
	err := ok.doRequest("GET", urlPath, "", &responses)
	if err != nil {
		return nil, err
	}

	tickers := make([]Ticker, 0, len(responses))
	for _, res := range responses {
		date, _ := time.Parse(time.RFC3339, res.Timestamp)
		arr := strings.Split(res.InstrumentID, "-")
		pair := NewCurrencyPairFromString(strings.Join(arr, "/"))
		tickers = append(tickers, Ticker{
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Open:   res.Open24h,
			Last:   res.Last,
			High:   res.High24h,
			Low:    res.Low24h,
			Vol:    res.BaseVolume24h,
			Buy:    res.BestBid,
			Sell:   res.BestAsk,
			TS:     int64(time.Duration(date.UnixNano() / int64(time.Millisecond)))})
	}

	return tickers, nil
}

func (ok *OKExSpot) GetDepth(pair CurrencyPair, size int, step int) (*Depth, error) {
	urlPath := fmt.Sprintf("/api/spot/v3/instruments/%s/book?size=%d", pair.ToSymbol("-"), size)

	var response struct {
		Asks      [][]interface{} `json:"asks"`
		Bids      [][]interface{} `json:"bids"`
		Timestamp string          `json:"timestamp"`
	}

	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	dep := new(Depth)
	dep.Market = pair
	dep.Symbol = pair.ToLowerSymbol("/")
	t, _ := time.Parse(time.RFC3339, response.Timestamp)
	dep.TS = int64(time.Duration(t.UnixNano() / int64(time.Millisecond)))

	for _, itm := range response.Asks {
		dep.AskList = append(dep.AskList, DepthRecord{
			Price:  ToFloat64(itm[0]),
			Amount: ToFloat64(itm[1]),
		})
	}

	for _, itm := range response.Bids {
		dep.BidList = append(dep.BidList, DepthRecord{
			Price:  ToFloat64(itm[0]),
			Amount: ToFloat64(itm[1]),
		})
	}

	//sort.Sort(sort.Reverse(dep.AskList))

	return dep, nil
}

func (ok *OKExSpot) GetTrades(pair CurrencyPair, size int) ([]Trade, error) {
	urlPath := fmt.Sprintf("/api/spot/v3/instruments/%s/trades", pair.ToSymbol("-"))
	var response []struct {
		Time      string  `json:"time"`
		Timestamp string  `json:"timestamp"`
		TradeID   float64 `json:"trade_id,string"`
		Price     float64 `json:"price,string"`
		Size      float64 `json:"size,string"`
		Side      string  `json:"side"`
	}
	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	trades := make([]Trade, 0)
	for _, v := range response {
		date, _ := time.Parse(time.RFC3339, v.Time)
		//date, _ := time.Parse(time.RFC3339, v.Timestamp)

		t := Trade{
			Tid:    int64(v.TradeID),
			Side:   BUY,
			Amount: v.Size,
			Price:  v.Price,
			TS:     int64(time.Duration(date.UnixNano() / int64(time.Millisecond))),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
		}
		if v.Side == "sell" {
			t.Side = SELL
		}
		trades = append(trades, t)
	}

	return trades, nil
}

func (ok *OKExSpot) GetKlineRecords(pair CurrencyPair, period KlinePeriod, size, since int) ([]Kline, error) {
	granularity, isOk := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if isOk != true {
		return nil, fmt.Errorf("unsupported %v KlinePeriod:%v", ok.GetExchangeName(), period)
	}

	urlPath := "/api/spot/v3/instruments/%s/candles?granularity=%d"

	if since > 0 {
		sinceTime := time.Unix(int64(since), 0).UTC()
		if since/int(time.Second) != 1 { //如果不为秒，转为秒
			sinceTime = time.Unix(int64(since)/int64(time.Second), 0).UTC()
		}
		urlPath += "&start=" + sinceTime.Format(time.RFC3339)
	}

	var response [][]interface{}
	err := ok.doRequest("GET", fmt.Sprintf(urlPath, pair.ToSymbol("-"), granularity), "", &response)
	if err != nil {
		return nil, err
	}

	var klines []Kline
	for i := len(response) - 1; i >= 0; i-- {
		itm := response[i]
		t, _ := time.Parse(time.RFC3339, fmt.Sprint(itm[0]))
		klines = append(klines, Kline{
			TS:     t.Unix(),
			Market: pair,
			Symbol: pair.ToLowerSymbol("/"),
			Open:   ToFloat64(itm[1]),
			High:   ToFloat64(itm[2]),
			Low:    ToFloat64(itm[3]),
			Close:  ToFloat64(itm[4]),
			Vol:    ToFloat64(itm[5])})
	}

	return klines, nil
}

func (ok *OKExSpot) LimitBuy(pair CurrencyPair, price, amount string) (*Order, error) {
	return ok.placeOrder("limit", &Order{
		Price:  ToFloat64(price),
		Amount: ToFloat64(amount),
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Side:   BUY,
	})
}

func (ok *OKExSpot) LimitSell(pair CurrencyPair, price, amount string) (*Order, error) {
	return ok.placeOrder("limit", &Order{
		Price:  ToFloat64(price),
		Amount: ToFloat64(amount),
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Side:   SELL,
	})
}

func (ok *OKExSpot) MarketBuy(pair CurrencyPair, amount string) (*Order, error) {
	return ok.placeOrder("market", &Order{
		Amount: ToFloat64(amount),
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Side:   BUY_MARKET,
	})
}

func (ok *OKExSpot) MarketSell(pair CurrencyPair, amount string) (*Order, error) {
	return ok.placeOrder("market", &Order{
		Amount: ToFloat64(amount),
		Market: pair,
		Symbol: pair.ToLowerSymbol("/"),
		Side:   SELL_MARKET,
	})
}

func (ok *OKExSpot) Cancel(orderId string, pair CurrencyPair) (bool, error) {
	urlPath := "/api/spot/v3/cancel_orders/" + orderId
	param := struct {
		InstrumentId string `json:"instrument_id"`
	}{pair.ToLowerSymbol("-")}
	reqBody, _, _ := ok.buildRequestBody(param)
	var response struct {
		ClientOid string `json:"client_oid"`
		OrderId   string `json:"order_id"`
		Result    bool   `json:"result"`
	}
	err := ok.doRequest("POST", urlPath, reqBody, &response)
	if err != nil {
		return false, err
	}
	if response.Result {
		return true, nil
	}
	return false, fmt.Errorf("cancel fail, unknown error")
}

func (ok *OKExSpot) GetOrder(orderId string, pair CurrencyPair) (*Order, error) {
	urlPath := "/api/spot/v3/orders/" + orderId + "?instrument_id=" + pair.ToSymbol("-")
	//param := struct {
	//	InstrumentId string `json:"instrument_id"`
	//}{pair.AdaptUsdToUsdt().ToLower().ToSymbol("-")}
	//reqBody, _, _ := ok.buildRequestBody(param)
	var response map[string]interface{}
	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	ordInfo := ok.parseOrder(response, pair)

	return ordInfo, nil
}

func (ok *OKExSpot) GetPendingOrders(pair CurrencyPair) ([]Order, error) {
	urlPath := fmt.Sprintf("/api/spot/v3/orders_pending?instrument_id=%s", pair.ToSymbol("-"))
	var response []map[string]interface{}
	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	var orders []Order
	for _, v := range response {
		ord := ok.parseOrder(v, pair)
		orders = append(orders, *ord)
	}

	return orders, nil
}

func (ok *OKExSpot) parseOrder(orderMap map[string]interface{}, pair CurrencyPair) (ordInfo *Order) {
	ordInfo = &Order{
		//Cid:        response.ClientOid,
		OrderID:    ToString(orderMap["order_id"]),
		Price:      ToFloat64(orderMap["price"]),
		Amount:     ToFloat64(orderMap["size"]),
		AvgPrice:   ToFloat64(orderMap["price_avg"]),
		DealAmount: ToFloat64(orderMap["filled_size"]),
		Status:     ok.adaptOrderState(ToInt(orderMap["state"])),
		Market:     pair,
		Symbol:     pair.ToLowerSymbol("/"),
	}

	t := ToString(orderMap["type"])
	switch ToString(orderMap["side"]) {
	case "buy":
		if t == "market" {
			ordInfo.Side = BUY_MARKET
			ordInfo.Price = ordInfo.AvgPrice
			ordInfo.Amount = ordInfo.DealAmount
		} else {
			ordInfo.Side = BUY
		}
	case "sell":
		if t == "market" {
			ordInfo.Side = SELL_MARKET
			ordInfo.Price = ordInfo.AvgPrice
			ordInfo.Amount = ordInfo.DealAmount
		} else {
			ordInfo.Side = SELL
		}
	}

	date, err := time.Parse(time.RFC3339, ToString(orderMap["timestamp"]))
	if err != nil {
		println(err)
	} else {
		ordInfo.TS = int64(date.UnixNano() / int64(time.Millisecond))
	}

	return ordInfo
}

func (ok *OKExSpot) GetFinishedOrders(pair CurrencyPair) ([]Order, error) {
	// 查询完全成交的订单
	urlPath := "/api/spot/v3/orders/" + "?instrument_id=" + pair.ToSymbol("-") + "&state=2"

	var response []map[string]interface{}
	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0, len(response))
	for _, v := range response {
		ord := ok.parseOrder(v, pair)
		orders = append(orders, *ord)
	}

	return orders, nil
}

func (ok *OKExSpot) GetOrderDeal(orderId string, pair CurrencyPair) ([]OrderDeal, error) {
	// 查询完全成交的订单
	urlPath := "/api/spot/v3/fills?order_id=" + orderId + "&instrument_id=" + pair.ToSymbol("-")

	var response []dealResponse
	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	deals := make([]OrderDeal, 0, len(response))
	for _, v := range response {
		// 一笔成交，会返回两条数据
		if strings.ToUpper(v.Currency) == pair.Stock.Symbol() {
			deal := ok.adaptDeal(v)
			deal.Market = pair
			deal.Symbol = pair.ToLowerSymbol("/")
			deals = append(deals, *deal)
		}
	}

	return deals, nil
}

func (ok *OKExSpot) GetUserTrades(currencyPair CurrencyPair) ([]Trade, error) {
	return nil, ErrorUnsupported
}

func (ok *OKExSpot) GetAccount() (*Account, error) {
	urlPath := "/api/spot/v3/accounts"
	var response []struct {
		Frozen    float64 `json:"frozen,string"`
		Hold      float64 `json:"hold,string"`
		Currency  string
		Balance   float64 `json:"balance,string"`
		Available float64 `json:"available,string"`
		Holds     float64 `json:"holds,string"`
	}

	err := ok.doRequest("GET", urlPath, "", &response)
	if err != nil {
		return nil, err
	}

	account := &Account{
		Exchange:    ok.GetExchangeName(),
		SubAccounts: make(map[Currency]SubAccount, 2)}

	for _, itm := range response {
		currency := NewCurrency(itm.Currency)
		account.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			FrozenAmount: itm.Hold,
			Amount:       itm.Available,
		}
	}

	return account, nil
}

func (ok *OKExSpot) uuid() string {
	return strings.Replace(uuid.New().String(), "-", "", 32)
}

func (ok *OKExSpot) doRequest(httpMethod, uri, reqBody string, response interface{}) error {
	url := ok.Endpoint + uri
	sign, timestamp := ok.doParamSign(httpMethod, uri, reqBody)
	resp, err := NewHttpRequest(ok.HttpClient, httpMethod, url, reqBody, map[string]string{
		CONTENT_TYPE: APPLICATION_JSON_UTF8,
		ACCEPT:       APPLICATION_JSON,
		//COOKIE:               LOCALE + "en_US",
		OK_ACCESS_KEY:        ok.ApiKey,
		OK_ACCESS_PASSPHRASE: ok.ApiPassphrase,
		OK_ACCESS_SIGN:       sign,
		OK_ACCESS_TIMESTAMP:  fmt.Sprint(timestamp)})
	if err != nil {
		return err
	} else {
		return json.Unmarshal(resp, &response)
	}
}

func (ok *OKExSpot) adaptOrderState(state int) TradeStatus {
	switch state {
	case -2:
		return ORDER_FAIL
	case -1:
		return ORDER_CANCEL
	case 0:
		return ORDER_UNFINISH
	case 1:
		return ORDER_PART_FINISH
	case 2:
		return ORDER_FINISH
	case 3:
		return ORDER_UNFINISH
	case 4:
		return ORDER_CANCEL_ING
	}
	return ORDER_UNFINISH
}

/*
 Get a http request body is a json string and a byte array.
*/
func (ok *OKExSpot) buildRequestBody(params interface{}) (string, *bytes.Reader, error) {
	if params == nil {
		return "", nil, errors.New("illegal parameter")
	}
	data, err := json.Marshal(params)
	if err != nil {
		return "", nil, errors.New("json convert string error")
	}

	jsonBody := string(data)
	binBody := bytes.NewReader(data)

	return jsonBody, binBody, nil
}

func (ok *OKExSpot) doParamSign(httpMethod, uri, requestBody string) (string, string) {
	timestamp := ok.isoTime()
	preText := fmt.Sprintf("%s%s%s%s", timestamp, strings.ToUpper(httpMethod), uri, requestBody)
	sign, _ := GetParamHmacSHA256Base64Sign(ok.ApiSecretKey, preText)
	return sign, timestamp
}

/*
 Get a iso time
  eg: 2018-03-16T18:02:48.284Z
*/
func (ok *OKExSpot) isoTime() string {
	utcTime := time.Now().UTC()
	iso := utcTime.String()
	isoBytes := []byte(iso)
	iso = string(isoBytes[:10]) + "T" + string(isoBytes[11:23]) + "Z"
	return iso
}

/**
Must Set Client Oid
*/
func (ok *OKExSpot) BatchPlaceOrders(orders []Order) ([]placeOrderResponse, error) {
	var param []placeOrderParam
	var response map[string][]placeOrderResponse

	for _, ord := range orders {
		param = append(param, placeOrderParam{
			InstrumentId: ord.Market.ToSymbol("-"),
			//ClientOid:    ord.Cid,
			Side:  strings.ToLower(ord.Side.String()),
			Size:  ord.Amount,
			Price: ord.Price,
			Type:  "limit",
			//OrderType:    ord.Side,
		})
	}
	reqBody, _, _ := ok.buildRequestBody(param)
	err := ok.doRequest("POST", "/api/spot/v3/batch_orders", reqBody, &response)
	if err != nil {
		return nil, err
	}

	var ret []placeOrderResponse

	for _, v := range response {
		ret = append(ret, v...)
	}

	return ret, nil
}

func (ok *OKExSpot) placeOrder(ty string, ord *Order) (*Order, error) {
	urlPath := "/api/spot/v3/orders"
	param := placeOrderParam{
		ClientOid:    ok.uuid(),
		InstrumentId: ord.Market.ToLowerSymbol("-"),
	}

	var response placeOrderResponse

	switch ord.Side {
	case BUY, SELL:
		param.Side = strings.ToLower(ord.Side.String())
		param.Price = ord.Price
		param.Size = ord.Amount
	case SELL_MARKET:
		param.Side = "sell"
		param.Size = ord.Amount
	case BUY_MARKET:
		param.Side = "buy"
		param.Notional = ord.Amount
	default:
		param.Size = ord.Amount
		param.Price = ord.Price
	}

	switch ty {
	case "limit":
		param.Type = "limit"
	case "market":
		param.Type = "market"
		//case "post_only":
		//	param.OrderType = ORDER_FEATURE_POST_ONLY
		//case "fok":
		//	param.OrderType = ORDER_FEATURE_FOK
		//case "ioc":
		//	param.OrderType = ORDER_FEATURE_IOC
	}

	jsonStr, _, _ := ok.buildRequestBody(param)
	err := ok.doRequest("POST", urlPath, jsonStr, &response)
	if err != nil {
		return nil, err
	}

	if !response.Result {
		return nil, errors.New(response.ErrorMessage)
	}

	//ord.Cid = response.ClientOid
	ord.OrderID = response.OrderId

	return ord, nil
}

/*
ledger_id	String	账单ID
trade_id	String	成交ID
instrument_id	String	币对名称
price	String	成交价格
size	String	成交数量
order_id	String	订单ID
timestamp	String	订单成交时间
exec_type	String	流动性方向（T 或 M）
fee	String	手续费
side	String	账单方向（buy、sell）
currency	String	币种
*/
type dealResponse struct {
	TradeId      string  `json:"trade_id"`
	InstrumentId string  `json:"instrument_id"`
	Price        float64 `json:"price,string"`
	Size         float64 `json:"size,string"`
	OrderId      string  `json:"order_id"`
	Timestamp    string  `json:"timestamp"`
	Side         string  `json:"side"`
	Currency     string  `json:"currency"`
}

func (ok *OKExSpot) adaptDeal(response dealResponse) *OrderDeal {
	deal := &OrderDeal{
		OrderID:      response.OrderId,
		DealID:       response.TradeId,
		Price:        response.Price,
		FilledAmount: response.Size,
	}

	deal.FilledCashAmount = deal.FilledAmount * deal.Price
	switch response.Side {
	case "buy":
		deal.Side = BUY
	case "sell":
		deal.Side = SELL
	}

	date, err := time.Parse(time.RFC3339, response.Timestamp)
	if err != nil {
		println(err)
	} else {
		deal.TS = int64(date.UnixNano() / int64(time.Millisecond))
	}

	return deal
}
