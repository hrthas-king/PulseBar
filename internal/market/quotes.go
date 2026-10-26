// Package market 解析并校验 VOO / QQQ 基金 24 小时全时段（夜盘 BOATS、盘前、盘中、盘后）实时行情与状态机。
package market

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	MaxBody       = 16384
	PrimaryHost   = "query1.finance.yahoo.com"
	FallbackHost  = "query2.finance.yahoo.com"
	CookieHost    = "fc.yahoo.com"
	TimeoutMillis = 2200
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
	TradeTime     int64   `json:"trade_time,omitempty"`
	QuoteSource   string  `json:"quote_source,omitempty"`
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

type yahooQuoteBatchResponse struct {
	QuoteResponse struct {
		Result []yahooQuoteItem `json:"result"`
	} `json:"quoteResponse"`
}

type yahooQuoteItem struct {
	Symbol                       string   `json:"symbol"`
	Currency                     string   `json:"currency"`
	QuoteType                    string   `json:"quoteType"`
	ShortName                    string   `json:"shortName"`
	LongName                     string   `json:"longName"`
	MarketState                  string   `json:"marketState"`
	QuoteSourceName              string   `json:"quoteSourceName"`
	GMTOffsetMilliseconds        int64    `json:"gmtOffSetMilliseconds"`
	RegularMarketPrice           *float64 `json:"regularMarketPrice"`
	RegularMarketChange          *float64 `json:"regularMarketChange"`
	RegularMarketChangePercent   *float64 `json:"regularMarketChangePercent"`
	RegularMarketTime            *int64   `json:"regularMarketTime"`
	RegularMarketPreviousClose   *float64 `json:"regularMarketPreviousClose"`
	PreMarketPrice               *float64 `json:"preMarketPrice"`
	PreMarketChange              *float64 `json:"preMarketChange"`
	PreMarketChangePercent       *float64 `json:"preMarketChangePercent"`
	PreMarketTime                *int64   `json:"preMarketTime"`
	PostMarketPrice              *float64 `json:"postMarketPrice"`
	PostMarketChange             *float64 `json:"postMarketChange"`
	PostMarketChangePercent      *float64 `json:"postMarketChangePercent"`
	PostMarketTime               *int64   `json:"postMarketTime"`
	OvernightMarketPrice         *float64 `json:"overnightMarketPrice"`
	OvernightMarketChange        *float64 `json:"overnightMarketChange"`
	OvernightMarketChangePercent *float64 `json:"overnightMarketChangePercent"`
	OvernightMarketTime          *int64   `json:"overnightMarketTime"`
	FulldayPrice                 *float64 `json:"fulldayPrice"`
	FulldayChange                *float64 `json:"fulldayChange"`
	FulldayChangePercent         *float64 `json:"fulldayChangePercent"`
}

type sessionCandidate struct {
	price     float64
	changeAbs float64
	changePct float64
	tradeTime int64
	session   string
	mode      string
	valid     bool
}

func makeCandidate(price, chg, pct *float64, ts *int64, session, mode string) sessionCandidate {
	if price == nil || chg == nil || pct == nil || *price <= 0 {
		return sessionCandidate{}
	}
	var t int64
	if ts != nil {
		t = *ts
	}
	return sessionCandidate{
		price:     *price,
		changeAbs: *chg,
		changePct: *pct,
		tradeTime: t,
		session:   session,
		mode:      mode,
		valid:     true,
	}
}

// ParseYahooQuoteBatch 解析 /v7/finance/quote?overnightPrice=true 返回的 24h 全时段（夜盘 BOATS / 盘前 / 盘中 / 盘后）实时行情。
func ParseYahooQuoteBatch(body []byte, rttMs float64, now time.Time) (map[string]SymbolResult, error) {
	if len(body) > MaxBody {
		return nil, fmt.Errorf("响应超过 %d 字节上限", MaxBody)
	}
	var resp yahooQuoteBatchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if len(resp.QuoteResponse.Result) == 0 {
		return nil, fmt.Errorf("缺少行情结果")
	}

	out := make(map[string]SymbolResult, len(Symbols))
	var anySuccess bool
	for _, item := range resp.QuoteResponse.Result {
		if _, ok := Tickers[item.Symbol]; !ok {
			continue
		}
		q, err := parseYahooQuoteItem(item, rttMs, now)
		if err != nil {
			out[item.Symbol] = SymbolResult{Err: err.Error()}
		} else {
			anySuccess = true
			qCopy := q
			out[item.Symbol] = SymbolResult{Quote: &qCopy}
		}
	}
	for _, sym := range Symbols {
		if _, ok := out[sym]; !ok {
			out[sym] = SymbolResult{Err: "缺少标的响应"}
		}
	}
	if !anySuccess {
		return out, fmt.Errorf("全部标的解析失败")
	}
	return out, nil
}

func parseYahooQuoteItem(item yahooQuoteItem, rttMs float64, now time.Time) (Quote, error) {
	if item.Currency != "USD" || item.QuoteType != "ETF" {
		return Quote{}, fmt.Errorf("类型或币种不匹配 (%s/%s)", item.QuoteType, item.Currency)
	}
	desc := strings.TrimSpace(item.LongName)
	if desc == "" {
		desc = strings.TrimSpace(item.ShortName)
	}
	if desc == "" {
		desc = item.Symbol + " ETF"
	}
	var regPrice float64
	if item.RegularMarketPrice != nil {
		regPrice = *item.RegularMarketPrice
	}

	reg := makeCandidate(item.RegularMarketPrice, item.RegularMarketChange, item.RegularMarketChangePercent, item.RegularMarketTime, "盘中 (Regular)", "regular_realtime")
	post := makeCandidate(item.PostMarketPrice, item.PostMarketChange, item.PostMarketChangePercent, item.PostMarketTime, "盘后 (Post-Market)", "post_market_realtime")
	ovn := makeCandidate(item.OvernightMarketPrice, item.OvernightMarketChange, item.OvernightMarketChangePercent, item.OvernightMarketTime, "夜盘 (Overnight BOATS)", "overnight_boats_realtime")
	pre := makeCandidate(item.PreMarketPrice, item.PreMarketChange, item.PreMarketChangePercent, item.PreMarketTime, "盘前 (Pre-Market)", "pre_market_realtime")

	state := strings.ToUpper(strings.TrimSpace(item.MarketState))
	var chosen sessionCandidate

	if state == "REGULAR" && reg.valid {
		chosen = reg
	} else {
		// 按真实成交时间戳选择最新活跃时段，确保夜盘->盘前->盘中->盘后切换无缝且绝不回退到旧一日数据
		for _, c := range []sessionCandidate{ovn, pre, post, reg} {
			if !c.valid {
				continue
			}
			if !chosen.valid || (c.tradeTime > 0 && c.tradeTime > chosen.tradeTime) {
				chosen = c
			}
		}
		// 若无时间戳字段则按 marketState 选择
		if chosen.tradeTime == 0 {
			switch {
			case state == "OVERNIGHT" && ovn.valid:
				chosen = ovn
			case strings.HasPrefix(state, "PRE") && pre.valid:
				chosen = pre
			case strings.HasPrefix(state, "POST") && post.valid:
				chosen = post
			case reg.valid:
				chosen = reg
			}
		}
	}

	if !chosen.valid {
		return Quote{}, fmt.Errorf("缺少有效价格字段")
	}

	gmtSec := item.GMTOffsetMilliseconds / 1000
	clockSession := DetectSession(now, gmtSec)
	if clockSession == "周末休市" && (state == "CLOSED" || strings.HasPrefix(state, "POST")) {
		chosen.session = "周末休市 (收盘)"
	}

	src := strings.TrimSpace(item.QuoteSourceName)
	if src == "" {
		src = "Yahoo Finance Real-Time"
	}

	q := Quote{
		Symbol:        item.Symbol,
		Price:         chosen.price,
		ChangePercent: chosen.changePct,
		ChangeAbs:     chosen.changeAbs,
		RegularPrice:  regPrice,
		TradeTime:     chosen.tradeTime,
		QuoteSource:   src,
		UpdateMode:    chosen.mode,
		Session:       chosen.session,
		Description:   desc,
		RTTMs:         rttMs,
	}
	if err := ValidateQuote(q); err != nil {
		return Quote{}, err
	}
	return q, nil
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
	RegularMarketTime          int64    `json:"regularMarketTime"`
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

// ParseYahooChart 解析单标的 /v8/finance/chart 备用接口行情，并在非盘中时段自动修正相对于最近常规收盘价的涨跌幅。
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
	session := DetectSession(now, m.GMTOffset)

	var price, changeAbs, changePct float64
	mode := "24h_fullday_realtime"
	if m.FulldayPrice != nil && m.FulldayChange != nil && m.FulldayChangePercent != nil && *m.FulldayPrice > 0 {
		price = *m.FulldayPrice
		changeAbs = *m.FulldayChange
		changePct = *m.FulldayChangePercent
		// 若处于夜盘/盘前/盘后，且 fulldayChange 仍以两日前 chartPreviousClose 为基准，则先校验原始字段一致性，再修正为相对于最新常规收盘价 regPrice 的当日扩展时段涨跌幅
		if session != "盘中 (Regular)" && regPrice > 0 && m.ChartPreviousClose != nil && *m.ChartPreviousClose > 0 {
			rawPrev := price - changeAbs
			rawCalc := changeAbs / rawPrev * 100.0
			if isClose(rawCalc, changePct, 1e-6, 0.025) && math.Abs(*m.ChartPreviousClose-regPrice) > 1e-4 && math.Abs(rawPrev-*m.ChartPreviousClose) < 0.05 {
				changeAbs = price - regPrice
				changePct = changeAbs / regPrice * 100.0
			}
		}
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
		TradeTime:     m.RegularMarketTime,
		QuoteSource:   "Yahoo Chart Fallback",
		UpdateMode:    mode,
		Session:       session,
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
		fetched = fmt.Sprintf("%.1fs 前轮询", *v.ReceivedAge)
	}
	if v.Quote != nil {
		tradeInfo := ""
		if v.Quote.TradeTime > 0 {
			tradeAge := time.Now().Unix() - v.Quote.TradeTime
			if tradeAge < 0 {
				tradeAge = 0
			}
			if tradeAge < 120 {
				tradeInfo = fmt.Sprintf("；最近成交 %ds 前", tradeAge)
			} else if tradeAge < 7200 {
				tradeInfo = fmt.Sprintf("；最近成交 %dm%ds 前", tradeAge/60, tradeAge%60)
			}
		}
		srcInfo := ""
		if v.Quote.QuoteSource != "" {
			srcInfo = "；数据源: " + v.Quote.QuoteSource
		}
		return fmt.Sprintf("%s: %.2f (%+.2f%%, 涨跌 %+.2f, 常规收盘 %.2f) | %s\n  %s (%s)；时段: %s%s%s；%s",
			v.Symbol, v.Quote.Price, v.Quote.ChangePercent, v.Quote.ChangeAbs, v.Quote.RegularPrice, status,
			v.Quote.Description, Tickers[v.Symbol], session, srcInfo, tradeInfo, fetched)
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
