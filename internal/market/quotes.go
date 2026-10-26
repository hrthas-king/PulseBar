// Package market 解析并校验 VOO / QQQ 基金 24 小时全时段（夜盘、盘前、盘中、盘后）实时行情与状态机。
package market

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const (
	MaxBody       = 8192
	PrimaryHost   = "query1.finance.yahoo.com"
	FallbackHost  = "query2.finance.yahoo.com"
	TimeoutMillis = 1800
)

var (
	Symbols = []string{"VOO", "QQQ"}
	Tickers = map[string]string{
		"VOO": "NYSEArca:VOO",
		"QQQ": "NASDAQ:QQQ",
	}
)

type HTTPError struct {
	StatusCode int
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

type Quote struct {
	Symbol        string  `json:"symbol"`
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"change_percent"`
	ChangeAbs     float64 `json:"change_abs"`
	RegularPrice  float64 `json:"regular_price"`
	UpdateMode    string  `json:"update_mode"`
	Session       string  `json:"session"`
	Description   string  `json:"description"`
	RTTMs         float64 `json:"rtt_ms"`
}

func (q Quote) Previous() float64 {
	return q.Price - q.ChangeAbs
}

func (q Quote) SessionText() string {
	if q.Session != "" {
		return q.Session
	}
	return "24h 全时段实时"
}

func ValidateQuote(q Quote) error {
	if _, ok := Tickers[q.Symbol]; !ok {
		return fmt.Errorf("标的不匹配")
	}
	for _, v := range []float64{q.Price, q.ChangePercent, q.ChangeAbs, q.RTTMs} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("非有限数值")
		}
	}
	prev := q.Previous()
	if math.IsNaN(prev) || math.IsInf(prev, 0) || q.Price <= 0 || prev <= 0 || q.RTTMs < 0 {
		return fmt.Errorf("价格、基准或请求耗时无效")
	}
	calc := q.ChangeAbs / prev * 100.0
	if math.IsNaN(calc) || math.IsInf(calc, 0) || !isClose(calc, q.ChangePercent, 1e-6, 0.025) {
		return fmt.Errorf("最新价、涨跌额与涨跌幅不一致")
	}
	if q.UpdateMode == "" || len(q.UpdateMode) > 80 {
		return fmt.Errorf("更新模式字段无效")
	}
	return nil
}

func isClose(a, b, relTol, absTol float64) bool {
	diff := math.Abs(a - b)
	limit := math.Max(relTol*math.Max(math.Abs(a), math.Abs(b)), absTol)
	return diff <= limit
}

type SymbolResult struct {
	Quote *Quote
	Err   string
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta yahooMeta `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

type yahooMeta struct {
	Currency                   string   `json:"currency"`
	Symbol                     string   `json:"symbol"`
	InstrumentType             string   `json:"instrumentType"`
	ShortName                  string   `json:"shortName"`
	LongName                   string   `json:"longName"`
	GMTOffset                  int64    `json:"gmtoffset"`
	RegularMarketPrice         *float64 `json:"regularMarketPrice"`
	RegularMarketChangePercent *float64 `json:"regularMarketChangePercent"`
	ChartPreviousClose         *float64 `json:"chartPreviousClose"`
	PreviousClose              *float64 `json:"previousClose"`
	FulldayPrice               *float64 `json:"fulldayPrice"`
	FulldayChange              *float64 `json:"fulldayChange"`
	FulldayChangePercent       *float64 `json:"fulldayChangePercent"`
}

// DetectSession 根据美东时间（利用接口返回的 gmtoffset 或 UTC-4/-5）判断当前处于夜盘、盘前、盘中、盘后或周末休市。
func DetectSession(now time.Time, gmtOffsetSeconds int64) string {
	if gmtOffsetSeconds == 0 {
		gmtOffsetSeconds = -14400
	}
	et := now.UTC().Add(time.Duration(gmtOffsetSeconds) * time.Second)
	wd := et.Weekday()
	mins := et.Hour()*60 + et.Minute()

	// 周六全天、周五 20:00 后、周日 20:00 前为周末休市
	if wd == time.Saturday || (wd == time.Friday && mins >= 20*60) || (wd == time.Sunday && mins < 20*60) {
		return "周末休市"
	}
	switch {
	case mins >= 4*60 && mins < 9*60+30:
		return "盘前 (Pre-Market)"
	case mins >= 9*60+30 && mins < 16*60:
		return "盘中 (Regular)"
	case mins >= 16*60 && mins < 20*60:
		return "盘后 (Post-Market)"
	default:
		return "夜盘 (Overnight)"
	}
}

// ParseYahooChart 解析单标的 24h 全时段行情（优先采用覆盖夜盘/盘前/盘中/盘后的 fulldayPrice 实时流）。
func ParseYahooChart(expectedSymbol string, body []byte, rttMs float64, now time.Time) (Quote, error) {
	if len(body) > MaxBody {
		return Quote{}, fmt.Errorf("响应超过 %d 字节上限", MaxBody)
	}
	var resp yahooChartResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return Quote{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if len(resp.Chart.Result) != 1 {
		return Quote{}, fmt.Errorf("缺少或重复行情结果")
	}
	m := resp.Chart.Result[0].Meta
	if m.Symbol != expectedSymbol || m.Currency != "USD" || m.InstrumentType != "ETF" {
		return Quote{}, fmt.Errorf("标的、类型或币种不匹配 (%s/%s/%s)", m.Symbol, m.InstrumentType, m.Currency)
	}
	desc := m.LongName
	if desc == "" {
		desc = m.ShortName
	}
	if desc == "" {
		desc = expectedSymbol + " ETF"
	}
	var regPrice float64
	if m.RegularMarketPrice != nil {
		regPrice = *m.RegularMarketPrice
	}

	var price, changeAbs, changePct float64
	mode := "24h_fullday_realtime"
	if m.FulldayPrice != nil && m.FulldayChange != nil && m.FulldayChangePercent != nil && *m.FulldayPrice > 0 {
		price = *m.FulldayPrice
		changeAbs = *m.FulldayChange
		changePct = *m.FulldayChangePercent
	} else if m.RegularMarketPrice != nil && *m.RegularMarketPrice > 0 {
		price = *m.RegularMarketPrice
		prev := 0.0
		if m.ChartPreviousClose != nil && *m.ChartPreviousClose > 0 {
			prev = *m.ChartPreviousClose
		} else if m.PreviousClose != nil && *m.PreviousClose > 0 {
			prev = *m.PreviousClose
		}
		if prev <= 0 {
			return Quote{}, fmt.Errorf("缺少有效昨收基准价")
		}
		changeAbs = price - prev
		if m.RegularMarketChangePercent != nil {
			changePct = *m.RegularMarketChangePercent
		} else {
			changePct = changeAbs / prev * 100.0
		}
		mode = "regular_realtime"
	} else {
		return Quote{}, fmt.Errorf("缺少有效价格字段")
	}

	q := Quote{
		Symbol:        expectedSymbol,
		Price:         price,
		ChangePercent: changePct,
		ChangeAbs:     changeAbs,
		RegularPrice:  regPrice,
		UpdateMode:    mode,
		Session:       DetectSession(now, m.GMTOffset),
		Description:   desc,
		RTTMs:         rttMs,
	}
	if err := ValidateQuote(q); err != nil {
		return Quote{}, err
	}
	return q, nil
}

type View struct {
	Symbol      string   `json:"symbol"`
	Quote       *Quote   `json:"quote,omitempty"`
	Healthy     bool     `json:"healthy"`
	Reason      string   `json:"reason,omitempty"`
	ReceivedAge *float64 `json:"received_age_seconds,omitempty"`
}

func (v View) Tooltip() string {
	status := "实时数据正常"
	if !v.Healthy {
		status = "数据异常 — " + v.Reason
	}
	session := "等待数据"
	if v.Quote != nil {
		session = v.Quote.SessionText()
	}
	fetched := "尚未成功读取"
	if v.ReceivedAge != nil {
		fetched = fmt.Sprintf("%.1fs 前更新", *v.ReceivedAge)
	}
	if v.Quote != nil {
		return fmt.Sprintf("%s: %+.2f%% (最新 %.2f, 涨跌 %+.2f) | %s\n  %s (%s)；时段: %s；%s",
			v.Symbol, v.Quote.ChangePercent, v.Quote.Price, v.Quote.ChangeAbs, status,
			v.Quote.Description, Tickers[v.Symbol], session, fetched)
	}
	return fmt.Sprintf("%s: %s\n  %s；%s；%s",
		v.Symbol, status, Tickers[v.Symbol], session, fetched)
}

type feedState struct {
	quote      *Quote
	receivedAt time.Time
	err        string
}

type Tracker struct {
	feeds map[string]*feedState
}

func NewTracker() *Tracker {
	t := &Tracker{feeds: make(map[string]*feedState, len(Symbols))}
	t.Reset()
	return t
}

func (t *Tracker) Reset() {
	for _, sym := range Symbols {
		t.feeds[sym] = &feedState{err: "等待 VOO/QQQ 实时行情"}
	}
}

func (t *Tracker) ApplyBatch(batch map[string]SymbolResult, now time.Time) {
	for _, sym := range Symbols {
		res, ok := batch[sym]
		if !ok {
			t.Fail(sym, "缺少标的响应")
			continue
		}
		if res.Quote != nil {
			q := *res.Quote
			st := t.feeds[sym]
			st.quote = &q
			st.receivedAt = now
			st.err = ""
		} else {
			t.Fail(sym, res.Err)
		}
	}
}

func (t *Tracker) Fail(sym, reason string) {
	if st, ok := t.feeds[sym]; ok {
		if len(reason) > 180 {
			reason = reason[:180]
		}
		st.err = reason
	}
}

func (t *Tracker) FailAll(reason string) {
	for _, sym := range Symbols {
		t.Fail(sym, reason)
	}
}

func (t *Tracker) Views(now time.Time, intervalSeconds int) []View {
	if intervalSeconds < 2 {
		intervalSeconds = 2
	}
	maxAge := math.Max(float64(2*intervalSeconds)+float64(TimeoutMillis)/1000.0, 6.0)
	out := make([]View, 0, len(Symbols))
	for _, sym := range Symbols {
		st := t.feeds[sym]
		var agePtr *float64
		reason := st.err
		if st.quote != nil && !st.receivedAt.IsZero() {
			age := math.Max(0, now.Sub(st.receivedAt).Seconds())
			agePtr = &age
			if reason == "" && age > maxAge {
				reason = "超过预期刷新时间，数据源失联"
			}
		}
		var qCopy *Quote
		if st.quote != nil {
			c := *st.quote
			qCopy = &c
		}
		out = append(out, View{
			Symbol:      sym,
			Quote:       qCopy,
			Healthy:     qCopy != nil && reason == "",
			Reason:      reason,
			ReceivedAge: agePtr,
		})
	}
	return out
}
