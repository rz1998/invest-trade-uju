package logic

import (
	"fmt"
	basic "github.com/rz1998/invest-basic"
	"github.com/rz1998/invest-basic/types/investBasic"
	"github.com/rz1998/invest-trade-basic/types/tradeBasic"
	uju "github.com/rz1998/invest-trade-uju"
	"github.com/rz1998/invest-trade-uju/rpc/stock/types/tradeStock"
	"strconv"
	"time"
)

// TransAcFund 解析账户资金信息
func TransAcFund(resp *tradeStock.AccountInfoResp) *tradeBasic.SAcFund {
	var acFund *tradeBasic.SAcFund
	if resp == nil {
		return acFund
	}
	valBalance, _ := strconv.ParseFloat(resp.TotalAssets, 64)
	valAvailable, _ := strconv.ParseFloat(resp.AvailableFunds, 64)
	valFunds, _ := strconv.ParseFloat(resp.Funds, 64)

	acFund = &tradeBasic.SAcFund{
		ValWithdraw:  valAvailable,
		ValAvailable: valAvailable,
		ValFrozen:    valFunds - valAvailable,
		Margin:       valBalance - valFunds,
		ValBalance:   valBalance,
	}
	return acFund
}

// TransAcPos 解析有据股票持仓
func TransAcPos(resp *tradeStock.PositionInfoResp) *tradeBasic.SAcPos {
	var acPos *tradeBasic.SAcPos
	if resp == nil {
		return acPos
	}
	price, _ := strconv.ParseFloat(resp.LastPrice, 64)
	vol, _ := strconv.ParseFloat(resp.TotalPosition, 64)
	open, _ := strconv.ParseFloat(resp.CostPrice, 64)
	settle, _ := strconv.ParseFloat(resp.ClosePrice, 64)
	canSell, _ := strconv.ParseFloat(resp.CanSell, 64)
	acPos = &tradeBasic.SAcPos{
		UniqueCode:     uju.FromUju2StdUniqueCode(fmt.Sprintf("%s.%s", resp.Stocks, resp.Suffix)),
		TradeDir:       investBasic.LONG,
		PriceOpen:      int64(open * 10000),
		PriceSettle:    int64(settle * 10000),
		VolTotal:       int64(vol),
		VolYd:          int64(canSell),
		VolTd:          int64(vol - canSell),
		VolFrozenTotal: int64(vol - canSell),
		Margin:         price * vol,
	}
	return acPos
}

// TransOrderStatus 解析订单状态信息
func TransOrderStatus(resp *tradeStock.EntrustInfoResp) *tradeBasic.SOrderStatus {
	var orderStatus *tradeBasic.SOrderStatus
	if resp == nil {
		return orderStatus
	}

	vol, _ := strconv.ParseInt(resp.EntrustNum, 10, 64)
	volTraded, _ := strconv.ParseInt(resp.DealNum, 10, 64)
	volCanceled, _ := strconv.ParseInt(resp.CanceledNum, 10, 64)
	timeTrade, err := time.Parse("2006-01-02 15:04:05", resp.Time)
	if err != nil {
		fmt.Printf("TransOrderStatus time err %v\r\n", err)
	}

	orderStatus = &tradeBasic.SOrderStatus{
		Timestamp:         timeTrade.UnixMilli(),
		StatusOrderSubmit: tradeBasic.Accepted,
		VolTraded:         volTraded,
		VolTotal:          vol,
	}

	//status
	traded := false
	canceled := false
	if orderStatus.VolTraded > 0 {
		traded = true
	}
	if volCanceled > 0 {
		canceled = true
	}
	if canceled {
		orderStatus.TimeCancel = orderStatus.Timestamp
		if traded {
			orderStatus.StatusOrder = tradeBasic.PartialCanceled
		} else {
			orderStatus.StatusOrder = tradeBasic.Canceled
		}
	} else {
		if traded {
			if orderStatus.VolTraded == orderStatus.VolTotal {
				orderStatus.StatusOrder = tradeBasic.AllTraded
			} else {
				orderStatus.StatusOrder = tradeBasic.PartialTraded
			}
		} else {
			orderStatus.StatusOrder = tradeBasic.NotTraded
		}
	}
	return orderStatus
}

// TransOrderSysFromOrder 解析订单信息中的系统信息
func TransOrderSysFromOrder(resp *tradeStock.EntrustInfoResp, sourceInfo tradeBasic.ESourceInfo) *tradeBasic.SOrderSys {
	var orderSys *tradeBasic.SOrderSys
	if resp == nil {
		return orderSys
	}
	orderSys = &tradeBasic.SOrderSys{
		OrderRef:     resp.LocalOrderCode,
		IdOrderLocal: resp.EntrustNo,
		SourceInfo:   sourceInfo,
	}
	return orderSys
}

// TransReqOrder 解析订单信息中的报单数据
func TransReqOrder(resp *tradeStock.EntrustInfoResp) *tradeBasic.PReqOrder {
	var reqOrder *tradeBasic.PReqOrder
	if resp == nil {
		return reqOrder
	}

	price, _ := strconv.ParseFloat(resp.EntrustPrice, 64)
	vol, _ := strconv.ParseInt(resp.EntrustNum, 10, 64)
	timestamp, err := time.Parse("2006-01-02 15:04:05", resp.Time)
	if err != nil {
		fmt.Printf("TransReqOrder time err %v\r\n", err)
	}

	reqOrder = &tradeBasic.PReqOrder{
		Timestamp:  timestamp.UnixMilli(),
		UniqueCode: uju.FromUju2StdUniqueCode(fmt.Sprintf("%s.%s", resp.EntrustCode, resp.Exchange)),
		Price:      int64(price * 10000),
		Vol:        vol,
	}

	switch resp.EntrustType {
	case tradeStock.EntrustInfoResp_buy:
		reqOrder.Dir = investBasic.LONG
		reqOrder.FlagOffset = tradeBasic.Open
	case tradeStock.EntrustInfoResp_sell:
		reqOrder.Dir = investBasic.SHORT
		reqOrder.FlagOffset = tradeBasic.Close
	}
	return reqOrder
}

// TransOrderInfo 解析订单信息
func TransOrderInfo(resp *tradeStock.EntrustInfoResp, sourceInfo tradeBasic.ESourceInfo) *tradeBasic.SOrderInfo {
	var orderInfo *tradeBasic.SOrderInfo
	if resp == nil {
		return orderInfo
	}

	orderInfo = &tradeBasic.SOrderInfo{
		OrderSys:    TransOrderSysFromOrder(resp, sourceInfo),
		ReqOrder:    TransReqOrder(resp),
		OrderStatus: TransOrderStatus(resp),
	}
	return orderInfo
}

// TransOrderSysFromQry 从成交信息中解析系统信息
func TransOrderSysFromQry(resp *tradeStock.DealInfoResp, sourceInfo tradeBasic.ESourceInfo) *tradeBasic.SOrderSys {
	var orderSys *tradeBasic.SOrderSys
	if resp == nil {
		return orderSys
	}

	orderSys = &tradeBasic.SOrderSys{
		OrderRef:     resp.LocalOrderCode,
		IdOrderLocal: resp.EntrustNo,
		SourceInfo:   sourceInfo,
	}

	return orderSys
}

// TransTradeStatusFromQry 从成交信息中解析成交状态
func TransTradeStatusFromQry(resp *tradeStock.DealInfoResp, sourceInfo tradeBasic.ESourceInfo) *tradeBasic.STradeStatus {
	var tradeStatus *tradeBasic.STradeStatus
	if resp == nil {
		return tradeStatus
	}
	timestamp, err := time.Parse("2006-01-02 15:04:05", resp.Time)
	if err != nil {
		fmt.Printf("TransTradeStatusFromQry time err %v\r\n", err)
	}
	price, _ := strconv.ParseFloat(resp.AverageDealPrice, 64)
	val, _ := strconv.ParseFloat(resp.Amount, 64)
	vol, _ := strconv.ParseInt(resp.DealNum, 10, 64)

	tradeStatus = &tradeBasic.STradeStatus{
		Timestamp:   timestamp.UnixMilli(),
		IdTrade:     resp.DealCode,
		TypeTrade:   tradeBasic.Common,
		SourcePrice: tradeBasic.LastPrice,
		Price:       int64(price * 10000),
		Vol:         vol,
		Val:         val,
		Margin:      val,
		TradeSource: sourceInfo,
	}
	return tradeStatus
}

// TransTradeInfoFromQry 成交查询转成交回报
func TransTradeInfoFromQry(resp *tradeStock.DealInfoResp, sourceInfo tradeBasic.ESourceInfo) *tradeBasic.STradeInfo {
	var tradeInfo *tradeBasic.STradeInfo
	if resp == nil {
		return tradeInfo
	}
	timestamp, err := time.Parse("2006-01-02 15:04:05", resp.Time)
	if err != nil {
		fmt.Printf("TransTradeInfoFromQry time err %v\r\n", err)
	}

	tradeInfo = &tradeBasic.STradeInfo{
		Date:        timestamp.Format("2006-01-02"),
		OrderSys:    TransOrderSysFromQry(resp, sourceInfo),
		TradeStatus: TransTradeStatusFromQry(resp, sourceInfo),
		ReqOrder:    &tradeBasic.PReqOrder{},
	}

	return tradeInfo
}

// TransTradeStatusFromRtn 从成交回报解析成交状态
func TransTradeStatusFromRtn(resp *tradeStock.DealResp) *tradeBasic.STradeStatus {
	var tradeStatus *tradeBasic.STradeStatus
	if resp == nil {
		return tradeStatus
	}
	fees, _ := strconv.ParseFloat(resp.TotalCost, 64)
	margin, _ := strconv.ParseFloat(resp.DealAmount, 64)
	price, _ := strconv.ParseFloat(resp.AverageDealPrice, 64)
	val, _ := strconv.ParseFloat(resp.TotalDealAmount, 64)
	vol, _ := strconv.ParseInt(resp.DealNum, 10, 64)

	layout := ""
	if resp.TrdTime != "" {
		if len(resp.TrdTime) == 5 {
			layout = "20060102 30405"
		} else if len(resp.TrdTime) == 6 {
			layout = "20060102 150405"
		}
	}
	timestamp, err := time.Parse(layout, fmt.Sprintf("%s %s", resp.TrdDate, resp.TrdTime))
	if err != nil {
		fmt.Printf("TransTradeStatusFromRtn time err %v\r\n", err)
	}

	tradeStatus = &tradeBasic.STradeStatus{
		Timestamp:   timestamp.UnixMilli(),
		IdTrade:     resp.DealNo,
		TypeTrade:   tradeBasic.Common,
		SourcePrice: tradeBasic.LastPrice,
		Price:       int64(price * 10000),
		Vol:         vol,
		Val:         val,
		Margin:      margin,
		Fees:        fees,
		TradeSource: tradeBasic.RETURN,
	}

	return tradeStatus
}

// TransOrderSysFromRtn 从成交回报中解析系统信息
func TransOrderSysFromRtn(resp *tradeStock.DealResp) *tradeBasic.SOrderSys {
	var orderSys *tradeBasic.SOrderSys
	if resp == nil {
		return orderSys
	}
	orderSys = &tradeBasic.SOrderSys{
		OrderRef:     resp.LocalOrderCode,
		IdOrderLocal: resp.EntrustNo,
		SourceInfo:   tradeBasic.RETURN,
	}
	return orderSys
}

// TransTradeInfoFromRtn 解析成交回报
func TransTradeInfoFromRtn(resp *tradeStock.DealResp) *tradeBasic.STradeInfo {
	var tradeInfo *tradeBasic.STradeInfo
	if resp == nil {
		return tradeInfo
	}
	timestamp, err := time.Parse("20060102", resp.TrdDate)
	if err != nil {
		fmt.Printf("TransTradeInfoFromRtn time err %v\r\n", err)
	}

	tradeInfo = &tradeBasic.STradeInfo{
		Date:        timestamp.Format("2006-01-02"),
		OrderSys:    TransOrderSysFromRtn(resp),
		TradeStatus: TransTradeStatusFromRtn(resp),
	}

	return tradeInfo
}

func FromReqOrder(reqOrder *tradeBasic.PReqOrder) *tradeStock.OrderReq {
	var orderReq *tradeStock.OrderReq
	if reqOrder == nil {
		return orderReq
	}
	stockCode := ""
	var exchange investBasic.ExchangeCD
	if reqOrder.UniqueCode != "" {
		stockCode, exchange = basic.GetSecInfo(reqOrder.UniqueCode)
	}
	strPrice := ""
	if reqOrder.Price%100 == 0 {
		strPrice = fmt.Sprintf("%.2f", float64(reqOrder.Price)/10000.0)
	} else {
		strPrice = fmt.Sprintf("%.3f", float64(reqOrder.Price)/10000.0)
	}
	orderReq = &tradeStock.OrderReq{
		StockCode:      stockCode,
		Exchange:       uju.From2UjuExchangeCD(exchange),
		Num:            fmt.Sprintf("%d", reqOrder.Vol),
		Price:          strPrice,
		LocalOrderCode: reqOrder.OrderRef,
		PassiveOrderInfo: &tradeStock.PassiveOrderInfo{
			OrderFlag: tradeStock.PassiveOrderInfo_normal,
		},
	}

	switch reqOrder.Dir {
	case investBasic.LONG:
		orderReq.OrderType = tradeStock.OrderReq_buy
	case investBasic.SHORT:
		orderReq.OrderType = tradeStock.OrderReq_sell
	}
	return orderReq
}
