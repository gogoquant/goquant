package api

import (
	"errors"
	"fmt"
	goex "github.com/nntaoli-project/GoEx"
	"github.com/nntaoli-project/GoEx/builder"
	"github.com/xiyanxiyan10/quantcore/constant"
	"github.com/xiyanxiyan10/quantcore/model"
	"github.com/xiyanxiyan10/quantcore/util"
	"time"
)

// FutureExchange the exchange struct of futureExchange.com
type FutureExchange struct {
	BaseExchange
	stockTypeMap        map[string]goex.CurrencyPair
	stockTypeMapReverse map[goex.CurrencyPair]string

	tradeTypeMap        map[int]string
	tradeTypeMapReverse map[string]int
	exchangeTypeMap     map[string]string

	records map[string][]constant.Record
	host    string
	logger  model.Logger
	option  constant.Option

	limit     float64
	lastSleep int64
	lastTimes int64

	apiBuilder *builder.APIBuilder
	api        goex.FutureRestAPI
}

// NewFutureExchange create an exchange struct of futureExchange.com
func NewFutureExchange(opt constant.Option) *FutureExchange {
	futureExchange := FutureExchange{
		stockTypeMap: map[string]goex.CurrencyPair{
			"BTC/USD": goex.BTC_USD,
		},
		stockTypeMapReverse: map[goex.CurrencyPair]string{},
		tradeTypeMapReverse: map[string]int{},
		tradeTypeMap: map[int]string{
			goex.OPEN_BUY:   constant.TradeTypeLong,
			goex.OPEN_SELL:  constant.TradeTypeShort,
			goex.CLOSE_BUY:  constant.TradeTypeLongClose,
			goex.CLOSE_SELL: constant.TradeTypeShortClose,
		},
		exchangeTypeMap: map[string]string{
			constant.Fmex:    goex.FMEX,
			constant.HuobiDm: goex.HBDM,
		},
		records:   make(map[string][]constant.Record),
		host:      "https://www.futureExchange.com/api/v1/",
		logger:    model.Logger{TraderID: opt.TraderID, ExchangeType: opt.Type},
		option:    opt,
		limit:     10.0,
		lastSleep: time.Now().UnixNano(),
		//apiBuilder: builder.NewAPIBuilder().HttpTimeout(5 * time.Second),
	}
	futureExchange.SetRecordsPeriodMap(map[string]int{
		"M1":  goex.KLINE_PERIOD_1MIN,
		"M5":  goex.KLINE_PERIOD_5MIN,
		"M15": goex.KLINE_PERIOD_15MIN,
		"M30": goex.KLINE_PERIOD_30MIN,
		"H1":  goex.KLINE_PERIOD_1H,
		"H2":  goex.KLINE_PERIOD_4H,
		"H4":  goex.KLINE_PERIOD_4H,
		"D1":  goex.KLINE_PERIOD_1DAY,
		"W1":  goex.KLINE_PERIOD_1WEEK,
	})
	futureExchange.SetMinAmountMap(map[string]float64{
		"BTC/USD": 0.001,
	})
	return &futureExchange
}

// GetType get the type of this exchange
func (e *FutureExchange) Init() error {
	for k, v := range e.stockTypeMap {
		e.stockTypeMapReverse[v] = k
	}
	for k, v := range e.tradeTypeMap {
		e.tradeTypeMapReverse[v] = k
	}
	e.apiBuilder = builder.NewAPIBuilder().HttpTimeout(5 * time.Second)
	if e.apiBuilder == nil {
		return errors.New("api builder fail")
	}
	exchangeName := e.exchangeTypeMap[e.option.Type]
	e.api = e.apiBuilder.APIKey(e.option.AccessKey).APISecretkey(e.option.SecretKey).BuildFuture(exchangeName)
	return nil
}

// SetStockTypeMap ...
func (e *FutureExchange) SetStockTypeMap(m map[string]goex.CurrencyPair) {
	e.stockTypeMap = m
}

// GetStockTypeMap ...
func (e *FutureExchange) GetStockTypeMap() map[string]goex.CurrencyPair {
	return e.stockTypeMap
}

// Log print something to console
func (e *FutureExchange) Log(msgs ...interface{}) {
	e.logger.Log(constant.INFO, e.GetStockType(), 0.0, 0.0, msgs...)
}

// GetType get the type of this exchange
func (e *FutureExchange) GetType() string {
	return e.option.Type
}

// GetName get the name of this exchange
func (e *FutureExchange) GetName() string {
	return e.option.Name
}

// GetDepth ...
func (e *FutureExchange) GetDepth(size int) interface{} {
	var resDepth constant.Depth
	stockType := e.GetStockType()
	exchangeStockType, ok := e.stockTypeMap[stockType]
	if !ok {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetDepth() error, the error number is stockType")
		return false
	}
	depth, err := e.api.GetFutureDepth(exchangeStockType, e.GetContractType(), size)
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetDepth() error, the error number is ", err.Error())
		return false
	}
	resDepth.Time = depth.UTime.Unix()
	for _, ask := range depth.AskList {
		var resAsk constant.DepthRecord
		resAsk.Amount = ask.Amount
		resAsk.Price = ask.Price
		resDepth.Asks = append(resDepth.Asks, resAsk)
	}
	for _, bid := range depth.BidList {
		var resBid constant.DepthRecord
		resBid.Amount = bid.Amount
		resBid.Price = bid.Price
		resDepth.Bids = append(resDepth.Bids, resBid)
	}
	return resDepth
}

// GetPosition ...
func (e *FutureExchange) GetPosition() interface{} {
	resPositionVec := []constant.Position{}
	stockType := e.GetStockType()
	exchangeStockType, ok := e.stockTypeMap[stockType]
	if !ok {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetPosition() error, the error number is stockType")
		return false
	}
	positions, err := e.api.GetFuturePosition(exchangeStockType, e.GetContractType())
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetPosition() error, the error number is ", err.Error())
		return false
	}
	for _, position := range positions {
		var resPosition constant.Position
		if position.BuyAmount > 0 {
			resPosition.Price = position.BuyPriceAvg
			resPosition.Amount = position.BuyAmount
			resPosition.MarginLevel = position.LeverRate
			resPosition.Profit = position.BuyProfitReal
			resPosition.ForcePrice = position.ForceLiquPrice
			resPosition.TradeType = constant.TradeTypeBuy
			resPosition.ContractType = position.ContractType
			resPosition.StockType = position.Symbol.CurrencyA.Symbol + "/" + position.Symbol.CurrencyB.Symbol
			resPositionVec = append(resPositionVec, resPosition)
		}
		if position.SellAmount > 0 {
			resPosition.Price = position.SellPriceAvg
			resPosition.Amount = position.SellAmount
			resPosition.MarginLevel = position.LeverRate
			resPosition.ForcePrice = position.ForceLiquPrice
			resPosition.TradeType = constant.TradeTypeSell
			resPosition.ContractType = e.contractType
			resPosition.StockType = position.Symbol.CurrencyA.Symbol + "/" + position.Symbol.CurrencyB.Symbol
			resPositionVec = append(resPositionVec, resPosition)
		}
	}
	return resPositionVec
}

// SetLimit set the limit calls amount per second of this exchange
func (e *FutureExchange) SetLimit(times interface{}) float64 {
	e.limit = conver.Float64Must(times)
	return e.limit
}

// AutoSleep auto sleep to achieve the limit calls amount per second of this exchange
func (e *FutureExchange) AutoSleep() {
	now := time.Now().UnixNano()
	interval := 1e+9/e.limit*conver.Float64Must(e.lastTimes) - conver.Float64Must(now-e.lastSleep)
	if interval > 0.0 {
		time.Sleep(time.Duration(conver.Int64Must(interval)))
	}
	e.lastTimes = 0
	e.lastSleep = now
}

// GetMinAmount get the min trade amount of this exchange
func (e *FutureExchange) GetMinAmount(stock string) float64 {
	return e.minAmountMap[stock]
}

// GetAccount get the account detail of this exchange
func (e *FutureExchange) GetAccount() interface{} {
	account, err := e.api.GetFutureUserinfo()
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetAccount() error, the error number is ", err.Error())
		return false
	}
	var resAccount constant.Account
	resAccount.SubAccounts = make(map[string]constant.SubAccount)
	for k, v := range account.FutureSubAccounts {
		var subAccount constant.SubAccount
		stockType := k.Symbol
		subAccount.AccountRights = v.AccountRights
		subAccount.KeepDeposit = v.KeepDeposit
		subAccount.ProfitReal = v.ProfitReal
		subAccount.ProfitUnreal = v.ProfitUnreal
		subAccount.RiskRate = v.RiskRate
		resAccount.SubAccounts[stockType] = subAccount
	}
	return resAccount
}

func (e *FutureExchange) Buy(price, amount string, msg ...interface{}) interface{} {
	var err error
	var openType int
	stockType := e.GetStockType()
	exchangeStockType, ok := e.stockTypeMap[stockType]
	if !ok {
		e.logger.Log(constant.ERROR, e.GetStockType(), conver.Float64Must(amount), conver.Float64Must(amount), "Buy() error, the error number is stockType")
		return false
	}
	level := e.GetMarginLevel()
	var matchPrice = 0
	if price == "-1" {
		matchPrice = 1
	}
	if e.direction == constant.TradeTypeLong {
		openType = goex.OPEN_BUY
	} else if e.direction == constant.TradeTypeShortClose {
		openType = goex.CLOSE_SELL
	} else {
		e.logger.Log(constant.ERROR, e.GetStockType(), conver.Float64Must(amount), conver.Float64Must(amount), "Buy() error, the error number is tradeType")
		return false
	}
	orderId, err := e.api.PlaceFutureOrder(exchangeStockType, e.GetContractType(),
		price, amount, openType, matchPrice, level)

	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), conver.Float64Must(amount), conver.Float64Must(amount), "Sell() error, the error number is ", err.Error())
		return false
	}
	priceFloat := conver.Float64Must(price)
	amountFloat := conver.Float64Must(amount)
	e.logger.Log(e.direction, stockType, priceFloat, amountFloat, msg...)
	return orderId
}

func (e *FutureExchange) Sell(price, amount string, msg ...interface{}) interface{} {
	var err error
	var openType int
	stockType := e.GetStockType()
	exchangeStockType, ok := e.stockTypeMap[stockType]
	if !ok {
		e.logger.Log(constant.ERROR, e.GetStockType(), conver.Float64Must(amount), conver.Float64Must(amount), "Sell() error, the error number is stockType")
		return false
	}
	level := e.GetMarginLevel()
	var matchPrice = 0
	if price == "-1" {
		matchPrice = 1
	}
	if e.direction == constant.TradeTypeShort {
		openType = goex.OPEN_SELL
	} else if e.direction == constant.TradeTypeLongClose {
		openType = goex.CLOSE_BUY
	} else {
		e.logger.Log(constant.ERROR, e.GetStockType(), conver.Float64Must(amount), conver.Float64Must(amount), "Sell() error, the error number is tradeType")
		return false
	}
	orderId, err := e.api.PlaceFutureOrder(exchangeStockType, e.GetContractType(),
		price, amount, openType, matchPrice, level)

	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), conver.Float64Must(amount), conver.Float64Must(amount), "Sell() error, the error number is ", err.Error())
		return false
	}
	priceFloat := conver.Float64Must(price)
	amountFloat := conver.Float64Must(amount)
	e.logger.Log(e.direction, stockType, priceFloat, amountFloat, msg...)
	return orderId
}

// GetOrder get details of an order
func (e *FutureExchange) GetOrder(id string) interface{} {
	exchangeStockType, ok := e.stockTypeMap[e.GetStockType()]
	if !ok {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0, conver.Float64Must(id), "GetOrder() error, the error number is stockType")
		return false
	}
	orders, err := e.api.GetUnfinishFutureOrders(exchangeStockType, e.GetContractType())
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, conver.Float64Must(id), "GetOrder() error, the error number is ", err.Error())
		return false
	}
	for _, order := range orders {
		var TradeType string
		if order.OrderType == 0 {
			TradeType = constant.TradeTypeBuy
		} else {
			TradeType = constant.TradeTypeSell
		}
		if id != order.OrderID2 {
			continue
		}
		return constant.Order{
			Id:         order.OrderID2,
			Price:      order.Price,
			Amount:     order.Amount,
			DealAmount: order.DealAmount,
			TradeType:  TradeType,
			StockType:  e.GetStockType(),
		}
	}
	return false
}

// GetOrders get all unfilled orders
func (e *FutureExchange) GetOrders() interface{} {
	exchangeStockType, ok := e.stockTypeMap[e.GetStockType()]
	if !ok {
		e.logger.Log(constant.ERROR, "", 0, 0, "GetOrders() error, the error number is stockType")
		return false
	}
	orders, err := e.api.GetUnfinishFutureOrders(exchangeStockType, e.GetStockType())
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetOrders() error, the error number is ", err.Error())
		return false
	}
	resOrders := []constant.Order{}
	for _, order := range orders {
		var TradeType string
		if order.OrderType == 0 {
			TradeType = constant.TradeTypeBuy
		} else {
			TradeType = constant.TradeTypeSell
		}
		resOrder := constant.Order{
			Id:         order.OrderID2,
			Price:      order.Price,
			Amount:     order.Amount,
			DealAmount: order.DealAmount,
			TradeType:  TradeType,
			StockType:  e.GetStockType(),
		}
		resOrders = append(resOrders, resOrder)
	}
	return resOrders
}

// GetTrades get all filled orders recently
func (e *FutureExchange) GetTrades(params ...interface{}) interface{} {
	var traders []constant.Trader
	exchangeStockType, ok := e.stockTypeMap[e.GetStockType()]
	if !ok {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0, 0, "GetTrades() error, the error number is stockType")
		return false
	}
	APITraders, err := e.api.GetTrades(e.GetContractType(), exchangeStockType, 0)
	if err != nil {
		return false
	}
	for _, APITrader := range APITraders {
		trader := constant.Trader{
			Id:        APITrader.Tid,
			TradeType: e.tradeTypeMap[int(APITrader.Type)],
			Amount:    APITrader.Amount,
			Price:     APITrader.Price,
			StockType: e.stockTypeMapReverse[APITrader.Pair],
			Time:      APITrader.Date,
		}
		traders = append(traders, trader)
	}
	return traders
}

// CancelOrder cancel an order
func (e *FutureExchange) CancelOrder(orderID string) bool {
	exchangeStockType, ok := e.stockTypeMap[e.GetStockType()]
	if !ok {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0, conver.Float64Must(orderID), "CancelOrder() error, the error number is stockType")
		return false
	}
	result, err := e.api.FutureCancelOrder(exchangeStockType, e.GetContractType(), orderID)
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, conver.Float64Must(orderID), "CancelOrder() error, the error number is ", err.Error())
		return false
	}
	if !result {
		return false
	}
	e.logger.Log(constant.TradeTypeCancel, e.GetStockType(), 0, conver.Float64Must(orderID), "CancelOrder() success")
	return true
}

// GetTicker get market ticker
func (e *FutureExchange) GetTicker() interface{} {
	exchangeStockType, ok := e.stockTypeMap[e.GetStockType()]
	if !ok {
		e.logger.Log(constant.ERROR, "", 0, 0, "GetTicker() error, the error number is stockType")
		return false
	}
	exTicker, err := e.api.GetFutureTicker(exchangeStockType, e.GetContractType())
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetTicker() error, the error number is ", err.Error())
		return nil
	}
	//force covert
	tickStr := fmt.Sprint(exTicker.Date)
	ticker := constant.Ticker{
		Last: exTicker.Last,
		Buy:  exTicker.Buy,
		Sell: exTicker.Sell,
		High: exTicker.High,
		Low:  exTicker.Low,
		Vol:  exTicker.Vol,
		Time: conver.Int64Must(tickStr),
	}
	return ticker
}

// GetRecords get candlestick data
// params[0] period
// params[1] size
// params[2] since
func (e *FutureExchange) GetRecords(params ...interface{}) interface{} {
	exchangeStockType, ok := e.stockTypeMap[e.GetStockType()]
	var period = -1
	var size = 0
	var since = 0
	var periodStr = "M15"

	if len(params) > 1 && conver.StringMust(params[0]) != "" {
		periodStr = conver.StringMust(params[0])
	}

	period, ok = e.recordsPeriodMap[periodStr]
	if !ok {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0, 0, "GetRecords() error, the error number is stockType")
		return false
	}

	if len(params) > 2 && conver.IntMust(params[1]) > 0 {
		size = conver.IntMust(params[1])
	}

	if len(params) > 3 && conver.IntMust(params[2]) > 0 {
		since = conver.IntMust(params[2])
	}

	klineVec, err := e.api.GetKlineRecords(e.GetContractType(), exchangeStockType, period, size, since)
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetRecords() error, the error number is ", err.Error())
		return nil
	}
	timeLast := int64(0)
	if len(e.records[periodStr]) > 0 {
		timeLast = e.records[periodStr][len(e.records[periodStr])-1].Time
	}
	var recordsNew []constant.Record
	for i := len(klineVec); i > 0; i-- {
		kline := klineVec[i-1]
		recordTime := kline.Timestamp
		if recordTime > timeLast {
			recordsNew = append([]constant.Record{{
				Time:   recordTime,
				Open:   kline.Open,
				High:   kline.High,
				Low:    kline.Low,
				Close:  kline.Close,
				Volume: kline.Vol2,
			}}, recordsNew...)
		} else if timeLast > 0 && recordTime == timeLast {
			e.records[periodStr][len(e.records[periodStr])-1] = constant.Record{
				Time:   recordTime,
				Open:   kline.Open,
				High:   kline.High,
				Low:    kline.Low,
				Close:  kline.Close,
				Volume: kline.Vol2,
			}
		} else {
			break
		}
	}
	e.records[periodStr] = append(e.records[periodStr], recordsNew...)
	if len(e.records[periodStr]) > size {
		e.records[periodStr] = e.records[periodStr][len(e.records[periodStr])-size : len(e.records[periodStr])]
	}
	return e.records[periodStr]
}