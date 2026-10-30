package market

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Capitalization keeps the provider's company-wide total rather than the
// outstanding shares of just one share class. FXRate is CNY per one USD.
type Capitalization struct {
	Local     float64 `json:"local"`
	USD       float64 `json:"usd,omitempty"`
	Source    string  `json:"source"`
	Estimated bool    `json:"estimated,omitempty"`
	FXRate    float64 `json:"fx_rate,omitempty"`
	FXTime    int64   `json:"fx_time,omitempty"`
}

func positiveFinite(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (q Quote) IsEquity() bool { return q.QuoteType == "EQUITY" }

func (q *Quote) setCapitalization(value, basePrice float64, source string) {
	if !q.IsEquity() || !positiveFinite(value) {
		return
	}
	cap := &Capitalization{Local: value, Source: source}
	if positiveFinite(basePrice) && positiveFinite(q.Price) && !isClose(q.Price, basePrice, 1e-9, 1e-9) {
		cap.Local *= q.Price / basePrice
		cap.Estimated = true
	}
	if !positiveFinite(cap.Local) {
		return
	}
	if q.Currency == "USD" {
		cap.USD = cap.Local
	}
	q.Capitalization = cap
}

func (q *Quote) setYahooCapitalization(raw json.RawMessage, basePrice float64) {
	var value float64
	if json.Unmarshal(raw, &value) == nil {
		q.setCapitalization(value, basePrice, "Yahoo Finance 总市值")
	}
}

type fxQuote struct {
	rate float64
	at   int64
}

func parseCNYRate(body []byte, now time.Time) (fxQuote, error) {
	if len(body) > MaxBody {
		return fxQuote{}, fmt.Errorf("汇率响应超过大小限制")
	}
	var response yahooChartResponse
	if err := json.Unmarshal(body, &response); err != nil || len(response.Chart.Result) != 1 {
		return fxQuote{}, fmt.Errorf("汇率响应无效")
	}
	m := response.Chart.Result[0].Meta
	if m.Symbol != "CNY=X" || m.Currency != "CNY" || m.InstrumentType != "CURRENCY" || m.RegularMarketPrice == nil || !positiveFinite(*m.RegularMarketPrice) {
		return fxQuote{}, fmt.Errorf("汇率标识、方向或数值无效")
	}
	if m.RegularMarketTime <= 0 || m.RegularMarketTime > now.Add(30*time.Second).Unix() || m.RegularMarketTime < now.Add(-72*time.Hour).Unix() {
		return fxQuote{}, fmt.Errorf("汇率行情时间无效或过期")
	}
	return fxQuote{rate: *m.RegularMarketPrice, at: m.RegularMarketTime}, nil
}

func convertCapitalizationUSD(batch map[string]SymbolResult, fx fxQuote) {
	if !positiveFinite(fx.rate) {
		return
	}
	for _, result := range batch {
		q := result.Quote
		if q == nil || !q.IsEquity() || q.Currency != "CNY" || q.Capitalization == nil {
			continue
		}
		cap := *q.Capitalization
		usd := cap.Local / fx.rate
		if !positiveFinite(usd) {
			continue
		}
		cap.USD, cap.FXRate, cap.FXTime, cap.Estimated = usd, fx.rate, fx.at, true
		q.Capitalization = &cap
	}
}

func (q Quote) CapitalizationText() string {
	if !q.IsEquity() {
		return ""
	}
	cap := q.Capitalization
	if cap == nil || !positiveFinite(cap.USD) {
		return "市值 --"
	}
	units := []string{"", "K", "M", "B", "T"}
	value, unit := cap.USD, 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	if math.Round(value*100)/100 >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	approx := ""
	if cap.Estimated {
		approx = "≈"
	}
	return fmt.Sprintf("市值 %s$%.2f%s", approx, value, units[unit])
}

func (q Quote) capitalizationTooltip() string {
	if !q.IsEquity() {
		return ""
	}
	cap := q.Capitalization
	if cap == nil || !positiveFinite(cap.USD) {
		return "\n  美元总市值: 暂无数据（市值或换算汇率不可用）"
	}
	text := fmt.Sprintf("\n  美元总市值: $%.2f USD；来源: %s", cap.USD, cap.Source)
	if cap.Estimated {
		text += "；按最新价或汇率估算"
	}
	if cap.FXRate > 0 {
		text += fmt.Sprintf("；1 USD = %.4f CNY；汇率时间 %s UTC", cap.FXRate, time.Unix(cap.FXTime, 0).UTC().Format("2006-01-02 15:04:05"))
	}
	return text
}
