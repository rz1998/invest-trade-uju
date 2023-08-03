package uju

import (
	"fmt"
	basic "github.com/rz1998/invest-basic"
	"github.com/rz1998/invest-basic/types/investBasic"
)

func FromStd2UjuUniqueCode(uniqueCode string) string {
	code, exchangeCD := basic.GetSecInfo(uniqueCode)
	if exchangeCD == "" {
		return uniqueCode
	}
	ujuCode := code + "."
	switch exchangeCD {
	case investBasic.SSE:
		ujuCode += "SH"
	case investBasic.SZSE:
		ujuCode += "SZ"
	default:
		fmt.Printf("FromStd2UjuUniqueCode unhandled exchangeCD %s\n", exchangeCD)
	}
	return ujuCode
}

func FromUju2StdUniqueCode(ujuCode string) string {
	code, exchangeCDUju := basic.GetSecInfo(ujuCode)
	var exchangeCD investBasic.ExchangeCD
	switch string(exchangeCDUju) {
	case "SH":
		exchangeCD = investBasic.SSE
	case "SZ":
		exchangeCD = investBasic.SZSE
	default:
		fmt.Printf("FromUju2StdUniqueCode unhandled exchangeCD %s\n", exchangeCDUju)
	}
	return fmt.Sprintf("%s.%s", code, exchangeCD)
}
func From2UjuExchangeCD(exchangeCD investBasic.ExchangeCD) string {
	switch exchangeCD {
	case investBasic.SSE:
		return "SH"
	case investBasic.SZSE:
		return "SZ"
	default:
		fmt.Printf("From2UjuExchangeCD unhandled exchangeCD %v\n", exchangeCD)
	}
	return ""
}
