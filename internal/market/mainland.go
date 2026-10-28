package market

import "time"

func resolveMainlandSession(now time.Time, tradeTime int64) (SessionState, string) {
	zone := time.FixedZone("CST", 8*3600)
	local := now.In(zone)
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		return StateClosed, "周末休市"
	}
	m := local.Hour()*60 + local.Minute()
	switch {
	case m >= 9*60+15 && m < 9*60+30:
		return StatePreMarket, "集合竞价 (Pre-Market)"
	case (m >= 9*60+30 && m < 11*60+30) || (m >= 13*60 && m < 15*60):
		trade := time.Unix(tradeTime, 0).In(zone)
		if tradeTime > 0 && (trade.Year() != local.Year() || trade.YearDay() != local.YearDay()) {
			return StateClosed, "休市或行情日期未更新"
		}
		return StateRegular, "盘中 (Regular)"
	case m >= 11*60+30 && m < 13*60:
		return StateClosed, "午间休市 (Lunch Break)"
	default:
		return StateClosed, "休市 (Closed)"
	}
}
