package ujuStockClient

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	trade "github.com/rz1998/invest-trade-basic"
	"github.com/rz1998/invest-trade-basic/types/tradeBasic"
	"github.com/rz1998/invest-trade-uju/rpc/stock/internal/logic"
	"github.com/rz1998/invest-trade-uju/rpc/stock/types/tradeStock"
	ws "github.com/sacOO7/gowebsocket"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type ApiTradeUjuStock struct {
	spi        *trade.ISpiTrader
	socket     *ws.Socket
	childId    string
	userId     string
	infoAc     *tradeBasic.PInfoAc
	connecting bool
	connected  bool
	logining   bool
	isLogin    bool
	countOrder int32
}

// Login 登录
func (api *ApiTradeUjuStock) Login(infoAc *tradeBasic.PInfoAc) {
	if infoAc == nil {
		fmt.Println("Login stopped by no infoAc")
		return
	}
	api.countOrder = 0
	api.infoAc = infoAc
	api.childId = infoAc.UserId
	api.connecting = true
	api.connected = false
	api.logining = true
	api.isLogin = false
	// 初始化
	s := ws.New("ws://tradeBasic.stock.uju:18769/ws")

	s.OnConnected = func(socket ws.Socket) {
		if infoAc == nil {
			fmt.Println("OnConnected stopped by no infoAc")
			return
		}
		// 连接成功登录
		fmt.Printf("ApiTradeUjuStock Connected to server %+v\n", *infoAc)
		api.connecting = false
		api.connected = true
		// 密码md5
		digest := md5.New()
		digest.Write([]byte(infoAc.Psw))
		cipherStr := digest.Sum(nil)
		password := hex.EncodeToString(cipherStr)
		// 公司id
		compId := ""
		if strings.Contains(infoAc.BrokerId, "-") {
			// 有公司id
			compId = strings.Split(infoAc.BrokerId, "-")[1]
		}
		// uuid
		u1, err := uuid.NewUUID()
		if err != nil {
			fmt.Printf("uuid error %v\n", err)
		}
		header := tradeStock.Header{
			ChildId: 0,
			Uuid:    u1.String(),
			Type:    tradeStock.Header_LOGIN_REQ,
		}
		req := tradeStock.LoginReqMessage{
			Username:  infoAc.InvestorId,
			Passwd:    password,
			CompanyId: compId,
		}
		msg := tradeStock.WCMessage{
			Header:          &header,
			LoginReqMessage: &req,
		}
		bytes, err := proto.Marshal(&msg)
		if err != nil {
			fmt.Printf("ApiTradeUjuStock OnConnected error %v\n", err)
		}
		socket.SendBinary(bytes)
	}

	s.OnConnectError = func(err error, socket ws.Socket) {
		if infoAc == nil {
			fmt.Println("OnConnectError stopped by no infoAc")
			return
		}
		fmt.Printf("ApiTradeUjuStock Recieved connect error %v %+v\n", err, *infoAc)
		time.Sleep(1 * time.Second)
		api.Login(infoAc)
	}

	s.OnBinaryMessage = func(data []byte, socket ws.Socket) {
		msg := tradeStock.WCMessage{}
		err := proto.Unmarshal(data, &msg)
		if err != nil {
			fmt.Printf("error parsing proto %v\n", err)
		}
		go handleMsg(&msg, api)
	}

	s.OnDisconnected = func(err error, socket ws.Socket) {
		if infoAc == nil {
			fmt.Println("OnDisconnected stopped by no infoAc")
			return
		}
		fmt.Printf("ApiTradeUjuStock Login Disconnected from server %+v\n", *infoAc)
		api.connected = false
		if !api.isLogin && !api.logining {
			//登出后不再重连
			return
		}
		api.Login(infoAc)
	}
	// 连接
	api.socket = &s
	s.Connect()
}

// Logout 登出
func (api *ApiTradeUjuStock) Logout() {
	api.isLogin = false
	api.connected = false
	api.socket.Close()
	api.spi = nil
	api.infoAc = nil
}

// ReqOrder 报单请求
func (api *ApiTradeUjuStock) ReqOrder(reqOrder *tradeBasic.PReqOrder) {
	if api.infoAc == nil {
		fmt.Println("ReqOrder stopped by no infoAc")
		return
	}
	if reqOrder == nil {
		fmt.Printf("reqOrder stopped by no reqOrder %+v\n", *api.infoAc)
		return
	}
	// 处理断线
	timesRetry := 300
	for api.connecting && api.logining && timesRetry > 0 {
		fmt.Printf("reqOrder waiting for connect %+v\n", *api.infoAc)
		time.Sleep(1 * time.Second)
		timesRetry--
	}
	if api.connecting && api.logining && !api.connected {
		fmt.Printf("reqOrder stopped by not connected %+v\n", *api.infoAc)
		return
	}
	fmt.Printf("reqOrder %+v\n", *reqOrder)
	reqOrder.OrderRef = fmt.Sprintf("%s_%d_%d",
		api.infoAc.UserId,
		time.Now().UnixMilli(),
		atomic.AddInt32(&api.countOrder, 1))
	// 生成msg
	childId, _ := strconv.ParseInt(api.childId, 10, 32)
	// uuid
	u1, err := uuid.NewUUID()
	if err != nil {
		fmt.Printf("uuid error %v\n", err)
	}
	header := &tradeStock.Header{
		ChildId: int32(childId),
		Uuid:    u1.String(),
		Type:    tradeStock.Header_SERVICE_REQ,
	}
	reqMessage := &tradeStock.ReqMessage{
		Type:           tradeStock.ReqMessage_ORDER,
		UserId:         api.userId,
		LocalOrderCode: reqOrder.OrderRef,
		OrderReq:       logic.FromReqOrder(reqOrder),
	}
	msg := tradeStock.WCMessage{
		Header:     header,
		ReqMessage: reqMessage,
	}
	bytes, err := proto.Marshal(&msg)
	if err != nil {
		fmt.Printf("ReqOrder error %v\n", err)
	}
	api.socket.SendBinary(bytes)
	fmt.Printf("ReqOrder sending %v\n", msg)
}

// ReqOrderBatch 批量报单请求
func (api *ApiTradeUjuStock) ReqOrderBatch(reqOrders []*tradeBasic.PReqOrder) {
	if api.infoAc == nil {
		fmt.Println("ReqOrderBatch stopped by no infoAc")
		return
	}
	if reqOrders == nil || len(reqOrders) == 0 {
		fmt.Printf("ReqOrderBatch stopped by no reqOrder %+v\n", *api.infoAc)
		return
	}
	// 处理断线
	timesRetry := 300
	for api.connecting && api.logining && timesRetry > 0 {
		fmt.Printf("ReqOrderBatch waiting for connect %+v\n", *api.infoAc)
		time.Sleep(1 * time.Second)
		timesRetry--
	}
	if api.connecting && api.logining && !api.connected {
		fmt.Printf("ReqOrderBatch stopped by not connected %+v\n", *api.infoAc)
		return
	}
	fmt.Printf("reqOrders %v %+v\n", reqOrders, *api.infoAc)
	// 生成msg
	childId, _ := strconv.ParseInt(api.childId, 10, 32)
	// uuid
	u1, err := uuid.NewUUID()
	if err != nil {
		fmt.Printf("uuid error %v\n", err)
	}
	header := &tradeStock.Header{
		ChildId: int32(childId),
		Uuid:    u1.String(),
		Type:    tradeStock.Header_SERVICE_REQ,
	}
	batchOrder := make([]*tradeStock.OrderReq, len(reqOrders))
	for i, reqOrder := range reqOrders {
		reqOrder.OrderRef = fmt.Sprintf("%s_%d_%d",
			api.infoAc.UserId,
			time.Now().UnixMilli(),
			atomic.AddInt32(&api.countOrder, 1))
		batchOrder[i] = logic.FromReqOrder(reqOrder)
	}
	reqMessage := &tradeStock.ReqMessage{
		Type:       tradeStock.ReqMessage_BATCH_ORDER,
		UserId:     api.userId,
		BatchOrder: batchOrder,
	}
	msg := tradeStock.WCMessage{
		Header:     header,
		ReqMessage: reqMessage,
	}
	bytes, err := proto.Marshal(&msg)
	if err != nil {
		fmt.Printf("ReqOrderBatch error %v %+v\n", err, *api.infoAc)
	}
	api.socket.SendBinary(bytes)
	fmt.Printf("ReqOrderBatch sending %v %+v\n", msg, *api.infoAc)
}

// ReqOrderAction 订单操作请求
func (api *ApiTradeUjuStock) ReqOrderAction(reqOrderAction *tradeBasic.PReqOrderAction) {
	if api.infoAc == nil {
		fmt.Println("ReqOrderAction stopped by no infoAc")
		return
	}
	if reqOrderAction == nil {
		fmt.Printf("ReqOrderAction stopped by no reqOrder %+v\n", *api.infoAc)
		return
	}
	// 处理断线
	timesRetry := 300
	for api.connecting && api.logining && timesRetry > 0 {
		fmt.Printf("ReqOrderAction waiting for connect %+v\n", *api.infoAc)
		time.Sleep(1 * time.Second)
		timesRetry--
	}
	if api.connecting && api.logining && !api.connected {
		fmt.Printf("ReqOrderAction stopped by not connected %+v\n", *api.infoAc)
		return
	}
	fmt.Printf("ReqOrderAction %+v %+v\n", *reqOrderAction, *api.infoAc)
	// 生成msg
	childId, _ := strconv.ParseInt(api.childId, 10, 32)
	// uuid
	u1, err := uuid.NewUUID()
	if err != nil {
		fmt.Printf("uuid error %v\n", err)
	}
	header := &tradeStock.Header{
		ChildId: int32(childId),
		Uuid:    u1.String(),
		Type:    tradeStock.Header_SERVICE_REQ,
	}
	reqMessage := &tradeStock.ReqMessage{
		Type:           tradeStock.ReqMessage_CANCELORDER,
		LocalOrderCode: reqOrderAction.OrderSys.OrderRef,
	}
	msg := tradeStock.WCMessage{
		Header:     header,
		ReqMessage: reqMessage,
	}
	bytes, err := proto.Marshal(&msg)
	if err != nil {
		fmt.Printf("ReqOrderAction error %v %+v\n", err, *api.infoAc)
	}
	api.socket.SendBinary(bytes)
	fmt.Printf("ReqOrderAction sending %v %+v\n", msg, *api.infoAc)
}

// QryAcFund 查询资金
func (api *ApiTradeUjuStock) QryAcFund() {
	if api.infoAc == nil {
		fmt.Println("QryAcFund stopped by no infoAc")
		return
	}
	// 处理断线
	timesRetry := 300
	for api.connecting && api.logining && timesRetry > 0 {
		fmt.Printf("QryAcFund waiting for connect %+v\n", *api.infoAc)
		time.Sleep(1 * time.Second)
		timesRetry--
	}
	if api.connecting && api.logining && !api.connected {
		fmt.Printf("QryAcFund stopped by not connected %+v\n", *api.infoAc)
		return
	}
	fmt.Printf("QryAcFund %+v\n", *api.infoAc)
	// 生成msg
	childId, _ := strconv.ParseInt(api.childId, 10, 32)
	// uuid
	u1, err := uuid.NewUUID()
	if err != nil {
		fmt.Printf("uuid error %v\n", err)
	}
	header := &tradeStock.Header{
		ChildId: int32(childId),
		Uuid:    u1.String(),
		Type:    tradeStock.Header_SERVICE_REQ,
	}
	reqMessage := &tradeStock.ReqMessage{
		Type:   tradeStock.ReqMessage_ACOUNT_INFO_QRY,
		UserId: api.userId,
	}
	msg := tradeStock.WCMessage{
		Header:     header,
		ReqMessage: reqMessage,
	}
	bytes, err := proto.Marshal(&msg)
	if err != nil {
		fmt.Printf("QryAcFund error %v %+v\n", err, *api.infoAc)
	}
	api.socket.SendBinary(bytes)
	fmt.Printf("QryAcFund sending %v %+v\n", msg, *api.infoAc)
}

// QryAcLiability 查询负债
func (api *ApiTradeUjuStock) QryAcLiability() {

}

// QryAcPos 查询持仓
func (api *ApiTradeUjuStock) QryAcPos() {
	if api.infoAc == nil {
		fmt.Println("QryAcPos stopped by no infoAc")
		return
	}
	// 处理断线
	timesRetry := 300
	for api.connecting && api.logining && timesRetry > 0 {
		fmt.Printf("QryAcPos waiting for connect %+v\n", *api.infoAc)
		time.Sleep(1 * time.Second)
		timesRetry--
	}
	if api.connecting && api.logining && !api.connected {
		fmt.Printf("QryAcPos stopped by not connected %+v\n", *api.infoAc)
		return
	}
	fmt.Printf("QryAcPos %+v\n", *api.infoAc)
	// 生成msg
	childId, _ := strconv.ParseInt(api.childId, 10, 32)
	// uuid
	u1, err := uuid.NewUUID()
	if err != nil {
		fmt.Printf("uuid error %v\n", err)
	}
	header := &tradeStock.Header{
		ChildId: int32(childId),
		Uuid:    u1.String(),
		Type:    tradeStock.Header_SERVICE_REQ,
	}
	reqMessage := &tradeStock.ReqMessage{
		Type:   tradeStock.ReqMessage_POSITION_QRY,
		UserId: api.userId,
	}
	msg := tradeStock.WCMessage{
		Header:     header,
		ReqMessage: reqMessage,
	}
	bytes, err := proto.Marshal(&msg)
	if err != nil {
		fmt.Printf("QryAcPos error %v\n", err)
	}
	api.socket.SendBinary(bytes)
	fmt.Printf("QryAcPos sending %v %+v\n", msg, *api.infoAc)
}

// QryOrder 查询委托
func (api *ApiTradeUjuStock) QryOrder(orderSys *tradeBasic.SOrderSys) {
	if api.infoAc == nil {
		fmt.Println("QryOrder stopped by no infoAc")
		return
	}
	// 处理断线
	timesRetry := 300
	for api.connecting && api.logining && timesRetry > 0 {
		fmt.Printf("QryOrder waiting for connect %+v\n", *api.infoAc)
		time.Sleep(1 * time.Second)
		timesRetry--
	}
	if api.connecting && api.logining && !api.connected {
		fmt.Printf("QryOrder stopped by not connected %+v\n", *api.infoAc)
		return
	}
	if orderSys != nil {
		fmt.Printf("QryOrder %+v %+v\n", *orderSys, *api.infoAc)
	} else {
		fmt.Printf("QryOrder %+v\n", *api.infoAc)
	}
	// 生成msg
	childId, _ := strconv.ParseInt(api.childId, 10, 32)
	// uuid
	u1, err := uuid.NewUUID()
	if err != nil {
		fmt.Printf("uuid error %v\n", err)
	}
	header := &tradeStock.Header{
		ChildId: int32(childId),
		Uuid:    u1.String(),
		Type:    tradeStock.Header_SERVICE_REQ,
	}
	reqMessage := &tradeStock.ReqMessage{
		Type:      tradeStock.ReqMessage_ENTRUST_QRY,
		UserId:    api.userId,
		BeginTime: time.Now().Format("2006-01-02"),
	}
	msg := tradeStock.WCMessage{
		Header:     header,
		ReqMessage: reqMessage,
	}
	bytes, err := proto.Marshal(&msg)
	if err != nil {
		fmt.Printf("QryOrder error %v %+v\n", err, *api.infoAc)
	}
	api.socket.SendBinary(bytes)
	fmt.Printf("QryOrder sending %v %+v\n", msg, *api.infoAc)
}

// QryTrade 查询成交
func (api *ApiTradeUjuStock) QryTrade() {
	if api.infoAc == nil {
		fmt.Println("QryTrade stopped by no infoAc")
		return
	}
	// 处理断线
	timesRetry := 300
	for api.connecting && api.logining && timesRetry > 0 {
		fmt.Printf("QryTrade waiting for connect %+v\n", *api.infoAc)
		time.Sleep(1 * time.Second)
		timesRetry--
	}
	if api.connecting && api.logining && !api.connected {
		fmt.Printf("QryTrade stopped by not connected %+v\n", *api.infoAc)
		return
	}
	fmt.Printf("QryTrade %+v\n", *api.infoAc)
	// 生成msg
	childId, _ := strconv.ParseInt(api.childId, 10, 32)
	// uuid
	u1, err := uuid.NewUUID()
	if err != nil {
		fmt.Printf("uuid error %v\n", err)
	}
	header := &tradeStock.Header{
		ChildId: int32(childId),
		Uuid:    u1.String(),
		Type:    tradeStock.Header_SERVICE_REQ,
	}
	reqMessage := &tradeStock.ReqMessage{
		Type:      tradeStock.ReqMessage_DEAL_QRY,
		UserId:    api.userId,
		BeginTime: time.Now().Format("2006-01-02"),
	}
	msg := tradeStock.WCMessage{
		Header:     header,
		ReqMessage: reqMessage,
	}
	bytes, err := proto.Marshal(&msg)
	if err != nil {
		fmt.Printf("QryTrade error %v %+v\n", err, *api.infoAc)
	}
	api.socket.SendBinary(bytes)
	fmt.Printf("QryTrade sending %v %+v\n", msg, *api.infoAc)
}

// SetSpi 设置回报监听
func (api *ApiTradeUjuStock) SetSpi(spi *trade.ISpiTrader) {
	api.spi = spi
}

// GetSpi 获取回报监听
func (api *ApiTradeUjuStock) GetSpi() *trade.ISpiTrader {
	return api.spi
}

// GetInfoSession 获取会话信息
func (api *ApiTradeUjuStock) GetInfoSession() *tradeBasic.SInfoSessionTrader {
	return nil
}

func (api *ApiTradeUjuStock) GenerateUniqueOrder(orderSys *tradeBasic.SOrderSys) string {
	if orderSys == nil {
		return ""
	}
	return orderSys.OrderRef
}
func sendHeartBeat(api *ApiTradeUjuStock) {
	if api.infoAc == nil {
		fmt.Println("QryTrade stopped by no infoAc")
		return
	}
	for api.isLogin {
		// 生成msg
		childId, _ := strconv.ParseInt(api.childId, 10, 32)
		// uuid
		u1, err := uuid.NewUUID()
		if err != nil {
			fmt.Printf("uuid error %v\n", err)
		}
		header := tradeStock.Header{
			ChildId: int32(childId),
			Uuid:    u1.String(),
			Type:    tradeStock.Header_HEARTBEAT_REQ,
		}
		msg := tradeStock.WCMessage{
			Header: &header,
		}
		bytes, err := proto.Marshal(&msg)
		if err != nil {
			fmt.Printf("ApiTradeUjuStock sendHeartBeat error %v %+v\n", err, *api.infoAc)
		}
		if api.socket != nil {
			api.socket.SendBinary(bytes)
		}
		time.Sleep(4 * time.Second)
	}
}

func handleMsg(wcMessage *tradeStock.WCMessage, api *ApiTradeUjuStock) {
	if api.childId != "" &&
		wcMessage.Header != nil &&
		wcMessage.Header.ChildId != 0 &&
		api.childId != fmt.Sprintf("%d", wcMessage.Header.ChildId) {
		// 有子账户信息且子账户不对，直接返回
		return
	}
	// log内容
	if wcMessage.Header != nil &&
		(wcMessage.Header.Type == tradeStock.Header_HEARTBEAT_RESP ||
			wcMessage.Header.Type == tradeStock.Header_LOGIN_RESP) {

	} else {
		fmt.Printf("Recieved msg %+v\n", wcMessage)
	}
	// spi有问题不返回
	if api == nil {
		fmt.Println("stopped by no a....")
		return
	}
	if api.spi == nil {
		fmt.Println("stopped by no spi....")
		return
	}
	switch wcMessage.Header.Type {
	case tradeStock.Header_LOGIN_RESP:
		//登录请求响应
		//userId
		api.userId = wcMessage.LoginRespMessage.UserId
		api.logining = false
		api.isLogin = true
		// 发送心跳
		go sendHeartBeat(api)
	case tradeStock.Header_SERVICE_RESP:
		//服务请求响应
		//处理正确信息
		msgType := tradeStock.ReqMessage_ReqMsgType(wcMessage.RespMessage.ReqMsgType)
		switch msgType {
		//处理正确信息
		case tradeStock.ReqMessage_ACOUNT_INFO_QRY:
			//查询账户资金信息返回数据
			resps := wcMessage.RespMessage.AccountInfoResp
			if resps != nil && len(resps) > 0 {
				//查找指定子账号
				hasChildId := false
				for _, resp := range resps {
					if resp.ChildId == api.childId {
						(*api.spi).OnRtnAcFund(logic.TransAcFund(resp))
						hasChildId = true
						break
					}
				}
				//没找到子账号返回空数据
				if !hasChildId {
					(*api.spi).OnRtnAcFund(nil)
				}
			}
		case tradeStock.ReqMessage_POSITION_QRY:
			//查询持仓返回数据
			resps := wcMessage.RespMessage.PositionInfoResp
			if resps != nil && len(resps) > 0 {
				for _, resp := range resps {
					acPos := logic.TransAcPos(resp)
					if acPos != nil {
						(*api.spi).OnRtnAcPos(acPos, false)
					}
				}
			}
			(*api.spi).OnRtnAcPos(nil, true)
		case tradeStock.ReqMessage_ENTRUST_QRY:
			//查询委托返回数据
			resps := wcMessage.RespMessage.EtrustInfoResp
			if resps != nil && len(resps) > 0 {
				for _, resp := range resps {
					orderInfo := logic.TransOrderInfo(resp, tradeBasic.QUERY)
					if orderInfo != nil {
						(*api.spi).OnRtnOrder(orderInfo, false)
					}
				}
			}
			(*api.spi).OnRtnOrder(nil, true)
		case tradeStock.ReqMessage_DEAL_QRY:
			//查询成交返回数据
			resps := wcMessage.RespMessage.DealInfoResp
			if resps != nil && len(resps) > 0 {
				for _, resp := range resps {
					tradeInfo := logic.TransTradeInfoFromQry(resp, tradeBasic.QUERY)
					if tradeInfo != nil {
						(*api.spi).OnRtnTrade(tradeInfo, false)
					}
				}
			}
			(*api.spi).OnRtnTrade(nil, true)
		case tradeStock.ReqMessage_Push_Order:
			//委托状态回报
			resp := wcMessage.RespMessage.PushEntrustInfoResp
			if resp != nil {
				orderInfo := logic.TransOrderInfo(resp, tradeBasic.RETURN)
				(*api.spi).OnRtnOrder(orderInfo, true)
			}
		case tradeStock.ReqMessage_DEAL_RESP:
			//成交回报
			resp := wcMessage.RespMessage.DealResp
			if resp != nil {
				tradeInfo := logic.TransTradeInfoFromRtn(resp)
				(*api.spi).OnRtnTrade(tradeInfo, true)
			}
		case tradeStock.ReqMessage_ORDER, tradeStock.ReqMessage_CANCELORDER, tradeStock.ReqMessage_Push_Accountinfo,
			tradeStock.ReqMessage_POSITION_CHANGE, tradeStock.ReqMessage_BATCH_ORDER:
			//懒得处理了
		}
	}
}
