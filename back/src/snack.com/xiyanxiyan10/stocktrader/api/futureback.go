package api

import (
	"encoding/json"
	"errors"
	"fmt"
	goex "github.com/nntaoli-project/goex"
	"math"
	dbtypes "snack.com/xiyanxiyan10/stockdb/types"
	"snack.com/xiyanxiyan10/stocktrader/constant"
	"snack.com/xiyanxiyan10/stocktrader/util"
	"sync"
	"time"
)

// ExchangeFutureBackConfig ...
type ExchangeFutureBackConfig struct {
	ExName               string
	TakerFee             float64
	MakerFee             float64
	SupportCurrencyPairs []string
	QuoteCurrency        string //净值币种
	Account              constant.Account
	BackTestStartTime    int64
	BackTestEndTime      int64
	DepthSize            int64 //回测多少档深度
	UnGzip               bool  //是否解压
}

// ExchangeFutureBack ...
type ExchangeFutureBack struct {
	BaseExchange
	*sync.RWMutex
	acc                  *constant.Account
	name                 string
	makerFee             float64
	takerFee             float64
	supportCurrencyPairs []string
	quoteCurrency        string
	pendingOrders        map[string]*constant.Order
	finishedOrders       map[string]*constant.Order
	dataLoader           map[string]*DataLoader
	stockTypeMap         map[string]goex.CurrencyPair
	currData             dbtypes.OHLC
	idGen                *util.IDGen
	sortedCurrencies     constant.Account
	longPosition         map[string]constant.Position // 多仓
	shortPosition        map[string]constant.Position // 空仓
}

// NewExchangeFutureBack2Config ...
func NewExchangeFutureBack2Config(config ExchangeBackConfig) *ExchangeFutureBack {
	sim := &ExchangeFutureBack{
		RWMutex:              new(sync.RWMutex),
		idGen:                util.NewIDGen(config.ExName),
		name:                 config.ExName,
		makerFee:             config.MakerFee,
		takerFee:             config.TakerFee,
		acc:                  &config.Account,
		supportCurrencyPairs: config.SupportCurrencyPairs,
		quoteCurrency:        config.QuoteCurrency,
		pendingOrders:        make(map[string]*constant.Order, 0),
		finishedOrders:       make(map[string]*constant.Order, 0),
		dataLoader:           make(map[string]*DataLoader, 0),
		longPosition:         make(map[string]constant.Position, 0),
		shortPosition:        make(map[string]constant.Position, 0),
	}

	for key, sub := range sim.acc.SubAccounts {
		sim.sortedCurrencies.SubAccounts[key] = sub
	}
	return sim
}

// NewExchangeFutureBack ...
func NewExchangeFutureBack(config ExchangeBackConfig) *ExchangeFutureBack {
	sim := &ExchangeFutureBack{}
	sim.back = true
	return sim
}

// Debug ..,
func (e *ExchangeFutureBack) Debug() error {
	fmt.Printf("---FutureBack info start---\n")
	fmt.Printf("currTicker:\n")
	v, err := json.Marshal(e.currData)
	if err != nil {
		fmt.Printf("convert ticker err :%s\n", err.Error())
		return err
	}
	fmt.Printf("%s\n", string(v))
	marginRatio, lft, rht := e.marginRatio()
	fmt.Printf("marginRatio %f lft %f  rht %f\n", marginRatio, lft, rht)
	fmt.Printf("longPosition:\n")
	v, err = json.Marshal(e.longPosition)
	if err != nil {
		fmt.Printf("convert longPosition err :%s\n", err.Error())
		return err
	}
	fmt.Printf("%s\n", string(v))
	fmt.Printf("shortPosition:\n")
	v, err = json.Marshal(e.shortPosition)
	if err != nil {
		fmt.Printf("convert shortPosition err :%s\n", err.Error())
		return err
	}
	fmt.Printf("%s\n", string(v))
	fmt.Printf("account:\n")
	if e.acc != nil {
		v, err = json.Marshal(e.acc)
		if err != nil {
			fmt.Printf("convert account err :%s\n", err.Error())
			return err
		}
		fmt.Printf("%s\n", string(v))
	}
	fmt.Printf("pendingOrders:\n")
	v, err = json.Marshal(e.pendingOrders)
	if err != nil {
		fmt.Printf("convert pending orders err :%s\n", err.Error())
		return err
	}
	fmt.Printf("%s\n", string(v))
	fmt.Printf("---FutureBack info end---\n")
	return nil
}

func isContain(items []string, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}

// Ready ...
func (e *ExchangeFutureBack) Ready() error {
	var account constant.Account
	e.RWMutex = new(sync.RWMutex)
	e.idGen = util.NewIDGen(e.GetExchangeName())
	e.name = e.GetExchangeName()
	e.makerFee = e.BaseExchange.maker
	e.takerFee = e.BaseExchange.taker
	e.acc = &account
	e.acc.SubAccounts = make(map[string]constant.SubAccount)
	e.pendingOrders = make(map[string]*constant.Order, 0)
	e.finishedOrders = make(map[string]*constant.Order, 0)
	e.dataLoader = make(map[string]*DataLoader, 0)
	e.longPosition = make(map[string]constant.Position, 0)
	e.shortPosition = make(map[string]constant.Position, 0)
	markets, err := e.BaseExchange.BackGetSymbols()
	if err != nil {
		return err
	}
	for stock := range e.BaseExchange.subscribeMap {
		var loader DataLoader
		e.dataLoader[stock] = &loader
		if isContain(markets, stock) == false {
			e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "stock not found in BackGetSymbols()")
			return fmt.Errorf("stock not found in BackGetSymbols()")
		}
		timeRange, err := e.BaseExchange.BackGetTimeRange()
		if err != nil {
			return err
		}
		if e.BaseExchange.start < timeRange[0] || e.BaseExchange.end > timeRange[1] {
			e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "time range not in %d - %d", timeRange[0], timeRange[1])
			return fmt.Errorf("time range not in %d - %d", timeRange[0], timeRange[1])
		}
		periodRange, err := e.BaseExchange.BackGetPeriodRange()
		if err != nil {
			return err
		}
		if e.recordsPeriodDbMap[e.BaseExchange.period] < periodRange[0] || e.recordsPeriodDbMap[e.BaseExchange.period] > periodRange[1] {
			e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "period range not in %d - %d", periodRange[0], periodRange[1])
			return fmt.Errorf("period range not in %d - %d", periodRange[0], periodRange[1])
		}
		ohlcs, err := e.BaseExchange.BackGetOHLCs(e.BaseExchange.start, e.BaseExchange.end, e.BaseExchange.period)
		if err != nil {
			return err
		}
		e.dataLoader[stock].Load(ohlcs)
	}
	currencyMap := e.BaseExchange.currencyMap
	for key, val := range currencyMap {
		var sub constant.SubAccount
		sub.Amount = val
		e.acc.SubAccounts[key] = sub
	}
	return nil
}

// position2ValDiff ...
func (ex *ExchangeFutureBack) position2ValDiff(last float64, position constant.Position) float64 {
	amount := position.Amount + position.FrozenAmount
	price := position.Price
	priceDiff := last - price
	priceRate := util.SafefloatDivide(priceDiff, price)
	valDiff := priceRate * amount
	return valDiff
}

func (e *ExchangeFutureBack) settlePositionProfit(last float64, position *constant.Position, asset *constant.SubAccount, dir int) {
	valdiff := e.position2ValDiff(last, *position)
	amountdiff := util.SafefloatDivide(valdiff*e.contractRate, last)
	if dir == 1 {
		amountdiff = 0 - amountdiff
	}
	position.Profit = amountdiff
	position.ProfitRate = util.SafefloatDivide(valdiff, position.Amount+position.FrozenAmount)
}

// settlePosition ...
func (ex *ExchangeFutureBack) settlePosition() {
	stockType := ex.BaseExchange.GetStockType()
	ticker := ex.currData
	last := ticker.Close
	stocks := stockPair2Vec(stockType)
	CurrencyA := stocks[0]
	asset := ex.acc.SubAccounts[CurrencyA]

	if _, ok := ex.longPosition[CurrencyA]; ok {
		position := ex.longPosition[CurrencyA]
		ex.settlePositionProfit(last, &position, &asset, 0)
		ex.acc.SubAccounts[CurrencyA] = asset
		ex.longPosition[CurrencyA] = position
	}

	if _, ok := ex.shortPosition[CurrencyA]; ok {
		position := ex.shortPosition[CurrencyA]
		ex.settlePositionProfit(last, &position, &asset, 1)
		ex.acc.SubAccounts[CurrencyA] = asset
		ex.shortPosition[CurrencyA] = position
	}
}

// fillOrder ...
func (ex *ExchangeFutureBack) fillOrder(isTaker bool, amount, price float64, ord *constant.Order) {
	ord.FinishedTime = ex.currData.Time / int64(time.Millisecond) //set filled time
	ord.DealAmount = ord.Amount
	dealAmount := ord.DealAmount
	ord.Status = constant.ORDER_FINISH

	fee := ex.makerFee
	if isTaker {
		fee = ex.takerFee
	}

	tradeFee := 0.0
	tradeFee = dealAmount * ex.contractRate * fee
	tradeFee = math.Floor(tradeFee*100000000) / 100000000

	ord.Fee += tradeFee

	ex.unFrozenAsset(tradeFee, dealAmount, price, *ord)
}

func (ex *ExchangeFutureBack) matchOrder(ord *constant.Order, isTaker bool) {
	ticker := ex.currData
	switch ord.TradeType {
	case constant.TradeTypeLong, constant.TradeTypeShortClose:
		if ticker.Close <= ord.Price && ticker.Volume > 0 {
			ex.fillOrder(isTaker, ticker.Volume, ticker.Close, ord)
			if ord.Status == constant.ORDER_FINISH {
				delete(ex.pendingOrders, ord.Id)
				ex.finishedOrders[ord.Id] = ord
			}
		}
	case constant.TradeTypeShort, constant.TradeTypeLongClose:
		if ticker.Close >= ord.Price && ticker.Volume > 0 {
			ex.fillOrder(isTaker, ticker.Volume, ticker.Close, ord)
			if ord.Status == constant.ORDER_FINISH {
				delete(ex.pendingOrders, ord.Id)
				ex.finishedOrders[ord.Id] = ord
			}
		}
	}
}

func (ex *ExchangeFutureBack) match() {
	ex.Lock()
	defer ex.Unlock()
	for id := range ex.pendingOrders {
		ex.matchOrder(ex.pendingOrders[id], false)
	}
}

func (ex *ExchangeFutureBack) coverPosition() {
	stockType := ex.BaseExchange.GetStockType()
	stocks := stockPair2Vec(stockType)
	CurrencyA := stocks[0]
	marginRatio, _, rht := ex.marginRatio()
	if marginRatio < 0 || rht > 0.0 && marginRatio < ex.coverRate {

		fmt.Printf("force cover %f -> %f\n", marginRatio, ex.coverRate)
		ex.Debug()
		//Force cover position
		ex.longPosition[CurrencyA] = constant.Position{}
		ex.shortPosition[CurrencyA] = constant.Position{}
		ex.acc.SubAccounts[CurrencyA] = constant.SubAccount{}
		for _, order := range ex.pendingOrders {
			order.Status = constant.ORDER_CANCEL_ING
			ex.finishedOrders[order.Id] = order
		}
		ex.pendingOrders = make(map[string]*constant.Order)
	}
	if position, ok := ex.longPosition[CurrencyA]; ok && position.Amount == 0 {
		delete(ex.longPosition, CurrencyA)
	}

	if position, ok := ex.shortPosition[CurrencyA]; ok && position.Amount == 0 {
		delete(ex.shortPosition, CurrencyA)
	}
}

func (ex *ExchangeFutureBack) marginRatio() (float64, float64, float64) {
	stockType := ex.BaseExchange.GetStockType()
	stocks := stockPair2Vec(stockType)
	CurrencyA := stocks[0]
	asset := ex.acc.SubAccounts[CurrencyA]
	longposition := ex.longPosition[CurrencyA]
	shortposition := ex.shortPosition[CurrencyA]
	lft := asset.Amount + longposition.Profit + shortposition.Profit
	rht := 0.0
	rht = rht + util.SafefloatDivide(longposition.Amount*ex.BaseExchange.contractRate, longposition.Price)
	rht = rht + util.SafefloatDivide(shortposition.Amount*ex.BaseExchange.contractRate, shortposition.Price)
	rht = rht + asset.FrozenAmount*ex.lever
	return util.SafefloatDivide(lft, rht), lft, rht
}

// LimitBuy ...
func (ex *ExchangeFutureBack) LimitBuy(amount, price, currency string) (*constant.Order, error) {
	ex.Lock()
	defer ex.Unlock()

	ord := constant.Order{
		Price: goex.ToFloat64(price),
		//OpenPrice: ex.currData.Close,
		Amount:    goex.ToFloat64(amount),
		Id:        ex.idGen.Get(),
		Time:      ex.currData.Time / int64(time.Millisecond),
		Status:    constant.ORDER_UNFINISH,
		StockType: currency,
		TradeType: ex.BaseExchange.GetDirection(),
	}

	err := ex.frozenAsset(ord)
	if err != nil {
		return nil, err
	}

	ex.pendingOrders[ord.Id] = &ord
	ex.matchOrder(&ord, true)

	var result constant.Order
	util.DeepCopyStruct(ord, &result)
	return &result, nil
}

// LimitSell ...
func (ex *ExchangeFutureBack) LimitSell(amount, price, currency string) (*constant.Order, error) {
	ex.Lock()
	defer ex.Unlock()

	ord := constant.Order{
		Price: goex.ToFloat64(price),
		//OpenPrice: ex.currData.Close,
		Amount:    goex.ToFloat64(amount),
		Id:        ex.idGen.Get(),
		Time:      ex.currData.Time / int64(time.Millisecond),
		Status:    constant.ORDER_UNFINISH,
		StockType: currency,
		TradeType: ex.BaseExchange.GetDirection(),
	}

	err := ex.frozenAsset(ord)
	if err != nil {
		return nil, err
	}

	ex.pendingOrders[ord.Id] = &ord

	ex.matchOrder(&ord, true)

	var result constant.Order
	util.DeepCopyStruct(ord, &result)

	return &result, nil
}

// MarketBuy ...
func (ex *ExchangeFutureBack) MarketBuy(amount, price, currency string) (*constant.Order, error) {
	panic("not support")
}

// MarketSell ...
func (ex *ExchangeFutureBack) MarketSell(amount, price, currency string) (*constant.Order, error) {
	panic("not support")
}

// CancelOrder ...
func (ex *ExchangeFutureBack) CancelOrder(orderID string, currency string) (bool, error) {
	ex.Lock()
	defer ex.Unlock()

	ord := ex.finishedOrders[orderID]
	if ord != nil {
		return false, ErrCancelOrderFinished
	}

	ord = ex.pendingOrders[orderID]
	if ord == nil {
		return false, ErrNotFoundOrder
	}

	delete(ex.pendingOrders, ord.Id)

	ord.Status = constant.ORDER_CANCEL
	ex.finishedOrders[ord.Id] = ord

	ex.unFrozenAsset(0, 0, 0, *ord)

	return true, nil
}

// GetOneOrder ...
func (ex *ExchangeFutureBack) GetOneOrder(orderID, currency string) (*constant.Order, error) {
	ex.RLock()
	defer ex.RUnlock()

	ord := ex.finishedOrders[orderID]
	if ord == nil {
		ord = ex.pendingOrders[orderID]
	}

	if ord != nil {
		// deep copy
		var result constant.Order
		util.DeepCopyStruct(ord, &result)

		return &result, nil
	}

	return nil, ErrNotFoundOrder
}

// GetUnfinishOrders ...
func (ex *ExchangeFutureBack) GetUnfinishOrders(currency string) ([]constant.Order, error) {
	ex.RLock()
	defer ex.RUnlock()

	var unfinishedOrders []constant.Order
	for _, ord := range ex.pendingOrders {
		unfinishedOrders = append(unfinishedOrders, *ord)
	}

	return unfinishedOrders, nil
}

// GetOrderHistorys ...
func (ex *ExchangeFutureBack) GetOrderHistorys(currency string, currentPage, pageSize int) ([]constant.Order, error) {
	ex.RLock()
	defer ex.RUnlock()

	var orders []constant.Order
	for _, ord := range ex.finishedOrders {
		if ord.StockType == currency {
			orders = append(orders, *ord)
		}
	}
	return orders, nil
}

// GetAccount ...
func (ex *ExchangeFutureBack) GetAccount() (*constant.Account, error) {
	ex.RLock()
	defer ex.RUnlock()
	var account constant.Account
	account.SubAccounts = make(map[string]constant.SubAccount)
	for key, sub := range ex.acc.SubAccounts {
		longposition := ex.longPosition[key]
		shortposition := ex.shortPosition[key]
		sub.ProfitUnreal += longposition.Profit
		sub.ProfitUnreal += shortposition.Profit
		sub.Amount = sub.ProfitUnreal
		account.SubAccounts[key] = sub
	}
	return &account, nil
}

// GetTicker ...
func (ex *ExchangeFutureBack) GetTicker(currency string) (*constant.Ticker, error) {
	loader := ex.dataLoader[currency]
	if loader == nil {
		return nil, errors.New("loader not found")
	}
	ohlc := loader.Next()
	if ohlc == nil {
		return nil, nil
	}
	ex.currData = *ohlc
	ex.match()
	ex.settlePosition()
	ex.coverPosition()
	ex.Debug()
	return &constant.Ticker{
		Vol:  ohlc.Volume,
		Time: ohlc.Time,
		Last: ohlc.Close,
		Buy:  ohlc.Close,
		Sell: ohlc.Close,
		High: ohlc.High,
		Low:  ohlc.Low,
	}, nil
}

// GetDepth ...
func (ex *ExchangeFutureBack) GetDepth(size int, currency string) (*constant.Depth, error) {
	dbdepth, err := ex.BaseExchange.BackGetDepth(ex.currData.Time, ex.currData.Time, "M5")
	if err != nil {
		return nil, err
	}
	var depth constant.Depth
	for _, ask := range dbdepth.Asks {
		var record constant.DepthRecord
		record.Amount = ask.Amount
		record.Price = ask.Price
		depth.Asks = append(depth.Asks, record)
	}

	for _, bid := range dbdepth.Bids {
		var record constant.DepthRecord
		record.Amount = bid.Amount
		record.Price = bid.Price
		depth.Bids = append(depth.Bids, record)
	}
	return &depth, nil
}

// GetExchangeName ...
func (ex *ExchangeFutureBack) GetExchangeName() string {
	return ex.name
}

// frozenAsset 冻结
func (ex *ExchangeFutureBack) frozenAsset(order constant.Order) error {
	stocks := stockPair2Vec(order.StockType)
	CurrencyA := stocks[0]
	ticker := ex.currData
	var price float64 = 1
	price = ticker.Close
	avaAmount := ex.acc.SubAccounts[CurrencyA].Amount
	//avaAmount
	longposition := ex.longPosition[CurrencyA]
	shortposition := ex.shortPosition[CurrencyA]
	if longposition.Profit < 0 {
		avaAmount += longposition.Profit
	}

	if shortposition.Profit < 0 {
		avaAmount += shortposition.Profit
	}
	lever := ex.BaseExchange.lever
	switch order.TradeType {
	case constant.TradeTypeLong, constant.TradeTypeShort:
		if avaAmount*lever*price < order.Amount*ex.BaseExchange.contractRate {
			return ErrDataInsufficient
		}
		costAmount := util.SafefloatDivide(order.Amount*ex.BaseExchange.contractRate, lever*price)
		ex.acc.SubAccounts[CurrencyA] = constant.SubAccount{
			StockType:    CurrencyA,
			Amount:       avaAmount - costAmount,
			FrozenAmount: ex.acc.SubAccounts[CurrencyA].FrozenAmount + costAmount,
			LoanAmount:   0,
		}
	case constant.TradeTypeLongClose, constant.TradeTypeShortClose:
		if order.TradeType == constant.TradeTypeLongClose {
			position := ex.longPosition[CurrencyA]
			if position.Amount < order.Amount {
				return ErrDataInsufficient
			}
			position.Amount = position.Amount - order.Amount
			position.FrozenAmount = position.FrozenAmount + order.Amount
			ex.longPosition[CurrencyA] = position
		}

		if order.TradeType == constant.TradeTypeShortClose {
			position := ex.shortPosition[CurrencyA]
			if position.Amount < order.Amount {
				return ErrDataInsufficient
			}
			position.Amount = position.Amount - order.Amount
			position.FrozenAmount = position.FrozenAmount + order.Amount
			ex.shortPosition[CurrencyA] = position
		}
	}
	return nil
}

// unFrozenAsset 解冻
func (ex *ExchangeFutureBack) unFrozenAsset(fee, matchAmount, matchPrice float64, order constant.Order) {
	stocks := stockPair2Vec(order.StockType)
	CurrencyA := stocks[0]
	assetA := ex.acc.SubAccounts[CurrencyA]
	lever := ex.BaseExchange.lever
	switch order.TradeType {
	case constant.TradeTypeLong, constant.TradeTypeShort:
		order.OpenPrice = ex.currData.Close
		costAmount := util.SafefloatDivide(order.Amount*ex.BaseExchange.contractRate, lever*order.OpenPrice)
		if order.Status == constant.ORDER_CANCEL {
			ex.acc.SubAccounts[assetA.StockType] = constant.SubAccount{
				StockType:    assetA.StockType,
				Amount:       assetA.Amount + costAmount - order.DealAmount,
				FrozenAmount: assetA.FrozenAmount - (costAmount - order.DealAmount),
				LoanAmount:   0,
			}
		} else {
			if order.TradeType == constant.TradeTypeLong {
				position := ex.longPosition[CurrencyA]
				position.Price = util.SafefloatDivide(position.Price*position.Amount+order.OpenPrice*order.Amount,
					position.Amount+order.Amount)
				position.Amount = position.Amount + order.Amount
				ex.longPosition[CurrencyA] = position
				//fmt.Printf("set long position as:%v\n", position)
			} else {
				position := ex.shortPosition[CurrencyA]
				position.Price = util.SafefloatDivide(position.Price*position.Amount+order.OpenPrice*order.Amount,
					position.Amount+order.Amount)
				position.Amount = position.Amount + order.Amount
				ex.shortPosition[CurrencyA] = position
				//fmt.Printf("set short position as:%v\n", position)
			}
			ex.acc.SubAccounts[assetA.StockType] = constant.SubAccount{
				StockType:    assetA.StockType,
				FrozenAmount: assetA.FrozenAmount - costAmount,
				Amount:       assetA.Amount,
				LoanAmount:   0,
			}
		}
	case constant.TradeTypeLongClose, constant.TradeTypeShortClose:
		var position constant.Position
		order.OpenPrice = ex.currData.Close
		if order.TradeType == constant.TradeTypeLongClose {
			position = ex.longPosition[CurrencyA]
		} else {
			position = ex.shortPosition[CurrencyA]
		}
		if order.Status == constant.ORDER_CANCEL {
			position.Amount = position.Amount + order.Amount
			position.FrozenAmount = position.FrozenAmount - order.Amount
		} else {
			//ex.longPosition[CurrencyA] = position
			position.FrozenAmount = position.FrozenAmount - order.Amount
			ex.longPosition[CurrencyA] = position
			costAmount := (order.Amount * ex.BaseExchange.contractRate) / (lever * order.OpenPrice)
			ex.acc.SubAccounts[assetA.StockType] = constant.SubAccount{
				StockType:    assetA.StockType,
				Amount:       assetA.Amount + costAmount + fee,
				FrozenAmount: assetA.FrozenAmount,
				LoanAmount:   0,
			}
		}
		if order.TradeType == constant.TradeTypeLongClose {
			ex.longPosition[CurrencyA] = position
		} else {
			ex.shortPosition[CurrencyA] = position
		}

	}
}

// GetRecords get candlestick data
func (e *ExchangeFutureBack) GetRecords(period, maStr string) ([]constant.Record, error) {
	var size = constant.RecordSize

	vec, err := e.BaseExchange.BackGetOHLCs(e.BaseExchange.start, e.currData.Time, period)
	if err != nil {
		e.logger.Log(constant.ERROR, e.GetStockType(), 0.0, 0.0, "GetRecords() error")
		return nil, err
	}
	if len(vec) > size {
		vec = vec[len(vec)-size:]
	}
	var records []constant.Record
	for _, kline := range vec {
		records = append([]constant.Record{{
			Open:   kline.Open,
			High:   kline.High,
			Low:    kline.Low,
			Close:  kline.Close,
			Volume: kline.Volume,
		}}, records...)
	}
	return records, nil
}