// Package market 解析并校验全球多市场标的（美股 ETF/正股、A股、港股、亚太、欧洲指数）实时行情与五态时段状态机。
package market

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	MaxBody       = 32768
	PrimaryHost   = "query1.finance.yahoo.com"
	FallbackHost  = "query2.finance.yahoo.com"
	CookieHost    = "fc.yahoo.com"
	TimeoutMillis = 1500
)

const (
	GroupVOOQQQ      = "voo_qqq"
	GroupSOXXSMH     = "soxx_smh"
	GroupTop2US      = "top2_us"
	GroupCNIndices   = "cn_indices"
	GroupHKIndices   = "hk_indices"
	GroupAsiaIndices = "asia_indices"
	GroupEUIndices   = "eu_indices"
	DefaultGroup     = GroupVOOQQQ
)

// SessionState 表示五态交易时段状态信号。
type SessionState string

const (
	StatePreMarket  SessionState = "pre_market"  // ☀️ 早盘 / 盘前 (Pre-Market)
	StateRegular    SessionState = "regular"     // 🟢 盘中 (Regular Trading)
	StatePostMarket SessionState = "post_market" // 🔵 盘后 (Post-Market)
	StateOvernight  SessionState = "overnight"   // 🌙 夜盘 (Overnight)
	StateClosed     SessionState = "closed"      // ⚪ 休市 / 周末 (Closed / Weekend)
)

// Badge 返回用于文本诊断与悬停提示的时段状态符号。
func (s SessionState) Badge() string {
	switch s {
	case StatePreMarket:
		return "☀"
	case StateRegular:
		return "🟢"
	case StatePostMarket:
		return "🔵"
	case StateOvernight:
		return "🌙"
	default:
		return "⚪"
	}
}

type GroupSpec struct {
	ID        string
	MenuLabel string
	Symbols   []string
}

var Groups = []GroupSpec{
	{ID: GroupVOOQQQ, MenuLabel: "标普 / 纳指 (VOO / QQQ)", Symbols: []string{"VOO", "QQQ"}},
	{ID: GroupSOXXSMH, MenuLabel: "半导体 ETF (SOXX / SMH)", Symbols: []string{"SOXX", "SMH"}},
	{ID: GroupTop2US, MenuLabel: "美股市值前二 (NVDA / MSFT)", Symbols: []string{"NVDA", "MSFT"}},
	{ID: GroupCNIndices, MenuLabel: "A股指数 (上证 / 创指)", Symbols: []string{"000001.SS", "399006.SZ"}},
	{ID: GroupHKIndices, MenuLabel: "港股指数 (恒指 / 恒科)", Symbols: []string{"^HSI", "^HSTECH"}},
	{ID: GroupAsiaIndices, MenuLabel: "日韩指数 (日经225 / 韩国综合)", Symbols: []string{"^N225", "^KS11"}},
	{ID: GroupEUIndices, MenuLabel: "欧洲指数 (法国CAC / 德国DAX)", Symbols: []string{"^FCHI", "^GDAXI"}},
}

var (
	groupMu     sync.RWMutex
	activeGroup = Groups[0]
	Symbols     = []string{"VOO", "QQQ"}

	Aliases = map[string]string{
		"VOO":       "VOO",
		"QQQ":       "QQQ",
		"SOXX":      "SOXX",
		"SMH":       "SMH",
		"NVDA":      "NVDA",
		"MSFT":      "MSFT",
		"000001.SS": "上证",
		"399006.SZ": "创指",
		"^HSI":      "恒指",
		"^HSTECH":   "恒科",
		"^N225":     "日经",
		"^KS11":     "韩指",
		"^FCHI":     "CAC40",
		"^GDAXI":    "DAX",
	}

	Tickers = map[string]string{
		"VOO":       "NYSEArca:VOO",
		"QQQ":       "NASDAQ:QQQ",
		"SOXX":      "NASDAQ:SOXX",
		"SMH":       "NASDAQ:SMH",
		"NVDA":      "NASDAQ:NVDA",
		"MSFT":      "NASDAQ:MSFT",
		"000001.SS": "SSE:000001",
		"399006.SZ": "SZSE:399006",
		"^HSI":      "HKEX:HSI",
		"^HSTECH":   "HKEX:HSTECH",
		"^N225":     "OSE:N225",
		"^KS11":     "KRX:KS11",
		"^FCHI":     "Euronext:CAC40",
		"^GDAXI":    "XETRA:DAX",
	}

	allowedCurrencies = map[string]bool{
		"USD": true,
		"CNY": true,
		"HKD": true,
		"JPY": true,
		"EUR": true,
		"KRW": true,
		"":    true,
	}

	allowedQuoteTypes = map[string]bool{
		"ETF":    true,
		"EQUITY": true,
		"INDEX":  true,
		"":       true,
	}
)

func ValidGroup(id string) bool {
	for _, g := range Groups {
		if g.ID == id {
			return true
		}
	}
	return false
}

func GroupByID(id string) GroupSpec {
	for _, g := range Groups {
		if g.ID == id {
			return g
		}
	}
	return Groups[0]
}

func SetActiveGroup(id string) GroupSpec {
	g := GroupByID(id)
	groupMu.Lock()
	activeGroup = g
	Symbols = append([]string(nil), g.Symbols...)
	groupMu.Unlock()
	return g
}

func ActiveGroup() GroupSpec {
	groupMu.RLock()
	g := activeGroup
	groupMu.RUnlock()
	return g
}

func ActiveSymbols() []string {
	groupMu.RLock()
	out := append([]string(nil), activeGroup.Symbols...)
	groupMu.RUnlock()
	return out
}

func AliasFor(symbol string) string {
	if a, ok := Aliases[symbol]; ok && a != "" {
		return a
	}
	return symbol
}

func normalizeSymbol(sym string) string {
	s := strings.TrimSpace(sym)
	if strings.EqualFold(s, "HSTECH.HK") {
		return "^HSTECH"
	}
	return s
}

type HTTPError struct {
	StatusCode int
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

type Quote struct {
	Symbol        string       `json:"symbol"`
	Alias         string       `json:"alias"`
	Price         float64      `json:"price"`
	ChangePercent float64      `json:"change_percent"`
	ChangeAbs     float64      `json:"change_abs"`
	RegularPrice  float64      `json:"regular_price"`
	Currency      string       `json:"currency,omitempty"`
	QuoteType     string       `json:"quote_type,omitempty"`
	MarketState   string       `json:"market_state,omitempty"`
	State         SessionState `json:"session_state"`
	TradeTime     int64        `json:"trade_time,omitempty"`
	QuoteSource   string       `json:"quote_source,omitempty"`
	UpdateMode    string       `json:"update_mode"`
	Session       string       `json:"session"`
	Description   string       `json:"description"`
	RTTMs         float64      `json:"rtt_ms"`
}

func (q Quote) Previous() float64 {
	return q.Price - q.ChangeAbs
}

func (q Quote) SessionText() string {
	if q.Session != "" {
		return q.Session
	}
	return "实时行情"
}

func (q Quote) DisplayName() string {
	if q.Alias != "" {
		return q.Alias
	}
	return AliasFor(q.Symbol)
}

func ValidateQuote(q Quote) error {
	if _, ok := Tickers[q.Symbol]; !ok {
		return fmt.Errorf("标的不匹配 (%s)", q.Symbol)
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
	if math.IsNaN(calc) || math.IsInf(calc, 0) || !isClose(calc, q.ChangePercent, 1e-5, 0.05) {
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
	state     SessionState
	session   string
	mode      string
	valid     bool
}

func makeCandidate(price, chg, pct *float64, ts *int64, state SessionState, session, mode string) sessionCandidate {
	if price == nil || chg == nil || pct == nil || *price <= 0 {
		return sessionCandidate{}
	}
	var t int64
	if ts != nil {
		t = *ts
	}
	p := *price
	c := *chg
	cp := *pct
	if prev := p - c; prev > 0 {
		calc := c / prev * 100.0
		if !math.IsNaN(calc) && !math.IsInf(calc, 0) && !isClose(calc, cp, 1e-5, 0.05) && math.Abs(calc-cp) < 0.35 {
			cp = calc
		}
	}
	return sessionCandidate{
		price:     p,
		changeAbs: c,
		changePct: cp,
		tradeTime: t,
		state:     state,
		session:   session,
		mode:      mode,
		valid:     true,
	}
}

// ResolveSessionState 根据 Yahoo API 返回的 marketState、资产类型与交易所本地时钟判定五态时段状态。
// 注意：Yahoo 在当地午夜 00:00 后会将尚未进入盘前的休市状态标记为 PREPRE，在盘后结束后标记为 POSTPOST，
// 且对无盘后交易的欧洲/亚太现金指数（INDEX）标记为 POST/PREPRE，需结合交易所本地时间过滤误报。
func ResolveSessionState(marketState string, now time.Time, gmtOffsetSeconds int64) (SessionState, string) {
	return ResolveSessionStateForType(marketState, "", now, gmtOffsetSeconds)
}

func ResolveSessionStateForType(marketState, quoteType string, now time.Time, gmtOffsetSeconds int64) (SessionState, string) {
	raw := strings.ToUpper(strings.TrimSpace(marketState))
	qt := strings.ToUpper(strings.TrimSpace(quoteType))
	isIndex := qt == "INDEX"

	switch raw {
	case "REGULAR":
		return StateRegular, "盘中 (Regular)"
	case "OVERNIGHT":
		return StateOvernight, "夜盘 (Overnight)"
	case "CLOSED", "POSTPOST":
		return StateClosed, "休市 (Closed)"
	case "POST":
		if !isIndex && gmtOffsetSeconds <= 0 {
			return StatePostMarket, "盘后 (Post-Market)"
		}
		return StateClosed, "休市 (Closed)"
	case "PRE":
		if !isIndex && gmtOffsetSeconds <= 0 {
			return StatePreMarket, "盘前 (Pre-Market)"
		}
		if isPreOpenWindow(now, gmtOffsetSeconds) {
			return StatePreMarket, "盘前 (Pre-Market)"
		}
		return StateClosed, "休市 (Closed)"
	case "PREPRE":
		// PREPRE 表示“盘前尚未开始”（如亚太午夜 00:00–09:00 休市期，或美股 00:00–04:00 夜盘期）
		if !isIndex && gmtOffsetSeconds <= 0 {
			return inferSessionByClock(now, gmtOffsetSeconds)
		}
		if isPreOpenWindow(now, gmtOffsetSeconds) {
			return StatePreMarket, "盘前 (Pre-Market)"
		}
		return StateClosed, "休市 (Closed)"
	}
	return inferSessionByClock(now, gmtOffsetSeconds)
}

func isPreOpenWindow(now time.Time, gmtOffsetSeconds int64) bool {
	local := now.UTC().Add(time.Duration(gmtOffsetSeconds) * time.Second)
	wd := local.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	mins := local.Hour()*60 + local.Minute()
	if gmtOffsetSeconds == 8*3600 {
		// A股 / 港股盘前集合竞价窗口: 09:00 - 09:30
		return mins >= 9*60 && mins < 9*60+30
	}
	// 日韩 (UTC+9) 与欧洲 (UTC+1/+2) 盘前窗口: 08:30 - 09:00
	return mins >= 8*60+30 && mins < 9*60
}

func inferSessionByClock(now time.Time, gmtOffsetSeconds int64) (SessionState, string) {
	if gmtOffsetSeconds == 0 {
		gmtOffsetSeconds = -14400
	}
	local := now.UTC().Add(time.Duration(gmtOffsetSeconds) * time.Second)
	wd := local.Weekday()
	mins := local.Hour()*60 + local.Minute()

	// 美区时区（UTC-4 / UTC-5）具备 24×5 夜盘/盘前/盘中/盘后四段结构
	if gmtOffsetSeconds < 0 {
		if wd == time.Saturday || (wd == time.Friday && mins >= 20*60) || (wd == time.Sunday && mins < 20*60) {
			return StateClosed, "周末休市"
		}
		switch {
		case mins >= 4*60 && mins < 9*60+30:
			return StatePreMarket, "盘前 (Pre-Market)"
		case mins >= 9*60+30 && mins < 16*60:
			return StateRegular, "盘中 (Regular)"
		case mins >= 16*60 && mins < 20*60:
			return StatePostMarket, "盘后 (Post-Market)"
		default:
			return StateOvernight, "夜盘 (Overnight)"
		}
	}

	// 亚太及欧洲指数时区（UTC+1 ~ UTC+9）：仅在盘前集合竞价窗口显示盘前太阳，其余非交易时间一律为休市灰点
	if wd == time.Saturday || wd == time.Sunday {
		return StateClosed, "周末休市"
	}
	if isPreOpenWindow(now, gmtOffsetSeconds) {
		return StatePreMarket, "盘前 (Pre-Market)"
	}
	if gmtOffsetSeconds == 8*3600 {
		if mins >= 9*60+30 && mins < 16*60 {
			return StateRegular, "盘中 (Regular)"
		}
		return StateClosed, "休市 (Closed)"
	}
	if mins >= 9*60 && mins < 17*60+35 {
		return StateRegular, "盘中 (Regular)"
	}
	return StateClosed, "休市 (Closed)"
}

// DetectSession 根据交易所时区偏移返回当前时段文本描述。
func DetectSession(now time.Time, gmtOffsetSeconds int64) string {
	_, label := inferSessionByClock(now, gmtOffsetSeconds)
	return label
}

// ParseYahooQuoteBatch 解析 /v7/finance/quote 返回的多市场行情批次。
func ParseYahooQuoteBatch(body []byte, rttMs float64, now time.Time) (map[string]SymbolResult, error) {
	return ParseYahooQuoteBatchForSymbols(ActiveSymbols(), body, rttMs, now)
}

// ParseYahooQuoteBatchForSymbols 解析指定标的列表的 /v7/finance/quote 行情批次。
func ParseYahooQuoteBatchForSymbols(expectedSymbols []string, body []byte, rttMs float64, now time.Time) (map[string]SymbolResult, error) {
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

	out := make(map[string]SymbolResult, len(expectedSymbols))
	var anySuccess bool
	for _, item := range resp.QuoteResponse.Result {
		item.Symbol = normalizeSymbol(item.Symbol)
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
	for _, sym := range expectedSymbols {
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
	cur := strings.ToUpper(strings.TrimSpace(item.Currency))
	qt := strings.ToUpper(strings.TrimSpace(item.QuoteType))
	if !allowedCurrencies[cur] || !allowedQuoteTypes[qt] {
		return Quote{}, fmt.Errorf("不支持的类型或币种 (%s/%s)", item.QuoteType, item.Currency)
	}
	desc := strings.TrimSpace(item.LongName)
	if desc == "" {
		desc = strings.TrimSpace(item.ShortName)
	}
	if desc == "" {
		desc = AliasFor(item.Symbol)
	}
	var regPrice float64
	if item.RegularMarketPrice != nil {
		regPrice = *item.RegularMarketPrice
	}

	reg := makeCandidate(item.RegularMarketPrice, item.RegularMarketChange, item.RegularMarketChangePercent, item.RegularMarketTime, StateRegular, "盘中 (Regular)", "regular_realtime")
	post := makeCandidate(item.PostMarketPrice, item.PostMarketChange, item.PostMarketChangePercent, item.PostMarketTime, StatePostMarket, "盘后 (Post-Market)", "post_market_realtime")
	ovn := makeCandidate(item.OvernightMarketPrice, item.OvernightMarketChange, item.OvernightMarketChangePercent, item.OvernightMarketTime, StateOvernight, "夜盘 (Overnight BOATS)", "overnight_boats_realtime")
	pre := makeCandidate(item.PreMarketPrice, item.PreMarketChange, item.PreMarketChangePercent, item.PreMarketTime, StatePreMarket, "盘前 (Pre-Market)", "pre_market_realtime")

	rawState := strings.ToUpper(strings.TrimSpace(item.MarketState))
	var chosen sessionCandidate

	if rawState == "REGULAR" && reg.valid {
		chosen = reg
	} else {
		for _, c := range []sessionCandidate{ovn, pre, post, reg} {
			if !c.valid {
				continue
			}
			if !chosen.valid || (c.tradeTime > 0 && c.tradeTime > chosen.tradeTime) {
				chosen = c
			}
		}
		if chosen.tradeTime == 0 {
			switch {
			case rawState == "OVERNIGHT" && ovn.valid:
				chosen = ovn
			case strings.HasPrefix(rawState, "PRE") && pre.valid:
				chosen = pre
			case strings.HasPrefix(rawState, "POST") && post.valid:
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
	resolvedState, resolvedLabel := ResolveSessionStateForType(item.MarketState, qt, now, gmtSec)
	if strings.TrimSpace(item.MarketState) == "" && chosen.state != "" {
		resolvedState = chosen.state
		resolvedLabel = chosen.session
	} else if resolvedState == StateOvernight && chosen.mode == "overnight_boats_realtime" {
		resolvedLabel = "夜盘 (Overnight BOATS)"
	}

	src := strings.TrimSpace(item.QuoteSourceName)
	if src == "" {
		src = "Yahoo Finance"
	}

	q := Quote{
		Symbol:        item.Symbol,
		Alias:         AliasFor(item.Symbol),
		Price:         chosen.price,
		ChangePercent: chosen.changePct,
		ChangeAbs:     chosen.changeAbs,
		RegularPrice:  regPrice,
		Currency:      cur,
		QuoteType:     qt,
		MarketState:   rawState,
		State:         resolvedState,
		TradeTime:     chosen.tradeTime,
		QuoteSource:   src,
		UpdateMode:    chosen.mode,
		Session:       resolvedLabel,
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
	MarketState                string   `json:"marketState"`
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

// ParseYahooChart 解析单标的 /v8/finance/chart 备用接口行情，兼容 ETF、正股与全球指数。
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
	sym := normalizeSymbol(m.Symbol)
	cur := strings.ToUpper(strings.TrimSpace(m.Currency))
	inst := strings.ToUpper(strings.TrimSpace(m.InstrumentType))
	if sym != expectedSymbol || !allowedCurrencies[cur] || !allowedQuoteTypes[inst] {
		return Quote{}, fmt.Errorf("标的、类型或币种不匹配 (%s/%s/%s)", m.Symbol, m.InstrumentType, m.Currency)
	}
	desc := strings.TrimSpace(m.LongName)
	if desc == "" {
		desc = strings.TrimSpace(m.ShortName)
	}
	if desc == "" {
		desc = AliasFor(expectedSymbol)
	}
	var regPrice float64
	if m.RegularMarketPrice != nil {
		regPrice = *m.RegularMarketPrice
	}
	state, session := ResolveSessionStateForType(m.MarketState, inst, now, m.GMTOffset)

	var price, changeAbs, changePct float64
	mode := "24h_fullday_realtime"
	if m.FulldayPrice != nil && m.FulldayChange != nil && m.FulldayChangePercent != nil && *m.FulldayPrice > 0 {
		price = *m.FulldayPrice
		changeAbs = *m.FulldayChange
		changePct = *m.FulldayChangePercent
		if state != StateRegular && regPrice > 0 && m.ChartPreviousClose != nil && *m.ChartPreviousClose > 0 {
			rawPrev := price - changeAbs
			rawCalc := changeAbs / rawPrev * 100.0
			if isClose(rawCalc, changePct, 1e-5, 0.05) && math.Abs(*m.ChartPreviousClose-regPrice) > 1e-4 && math.Abs(rawPrev-*m.ChartPreviousClose) < 0.05 {
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
		Alias:         AliasFor(expectedSymbol),
		Price:         price,
		ChangePercent: changePct,
		ChangeAbs:     changeAbs,
		RegularPrice:  regPrice,
		Currency:      cur,
		QuoteType:     inst,
		MarketState:   strings.ToUpper(strings.TrimSpace(m.MarketState)),
		State:         state,
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
	Symbol      string       `json:"symbol"`
	Alias       string       `json:"alias"`
	State       SessionState `json:"session_state"`
	Quote       *Quote       `json:"quote,omitempty"`
	Healthy     bool         `json:"healthy"`
	Reason      string       `json:"reason,omitempty"`
	ReceivedAge *float64     `json:"received_age_seconds,omitempty"`
}

func (v View) DisplayName() string {
	if v.Alias != "" {
		return v.Alias
	}
	if v.Quote != nil && v.Quote.Alias != "" {
		return v.Quote.Alias
	}
	return AliasFor(v.Symbol)
}

func (v View) EffectiveState() SessionState {
	if v.Quote != nil && v.Quote.State != "" {
		return v.Quote.State
	}
	if v.State != "" {
		return v.State
	}
	return StateClosed
}

func (v View) Tooltip() string {
	status := "实时数据正常"
	if !v.Healthy {
		status = "数据异常 — " + v.Reason
	}
	session := "等待数据"
	stateBadge := v.EffectiveState().Badge()
	if v.Quote != nil {
		session = v.Quote.SessionText()
	}
	fetched := "尚未成功读取"
	if v.ReceivedAge != nil {
		fetched = fmt.Sprintf("%.1fs 前轮询", *v.ReceivedAge)
	}
	name := v.DisplayName()
	if name != v.Symbol {
		name = fmt.Sprintf("%s (%s)", name, v.Symbol)
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
		return fmt.Sprintf("%s %s: %.2f (%+.2f%%, 涨跌 %+.2f, 常规收盘 %.2f) | %s\n  %s (%s)；时段: %s %s%s%s；%s",
			stateBadge, name, v.Quote.Price, v.Quote.ChangePercent, v.Quote.ChangeAbs, v.Quote.RegularPrice, status,
			v.Quote.Description, Tickers[v.Symbol], stateBadge, session, srcInfo, tradeInfo, fetched)
	}
	return fmt.Sprintf("%s %s: %s\n  %s；%s；%s",
		stateBadge, name, status, Tickers[v.Symbol], session, fetched)
}

type feedState struct {
	quote      *Quote
	receivedAt time.Time
	err        string
	failures   int
}

type Tracker struct {
	groupID string
	symbols []string
	feeds   map[string]*feedState
}

func NewTracker() *Tracker {
	g := ActiveGroup()
	t := &Tracker{
		groupID: g.ID,
		symbols: append([]string(nil), g.Symbols...),
		feeds:   make(map[string]*feedState, len(g.Symbols)),
	}
	t.Reset()
	return t
}

func (t *Tracker) SetGroup(groupID string) {
	g := GroupByID(groupID)
	t.groupID = g.ID
	t.symbols = append([]string(nil), g.Symbols...)
	t.feeds = make(map[string]*feedState, len(g.Symbols))
	t.Reset()
}

func (t *Tracker) Reset() {
	if len(t.symbols) == 0 {
		t.symbols = ActiveSymbols()
	}
	t.feeds = make(map[string]*feedState, len(t.symbols))
	for _, sym := range t.symbols {
		t.feeds[sym] = &feedState{err: "等待 " + AliasFor(sym) + " 实时行情"}
	}
}

func (t *Tracker) ApplyBatch(batch map[string]SymbolResult, now time.Time) {
	for _, sym := range t.symbols {
		res, ok := batch[sym]
		if !ok {
			t.Fail(sym, "缺少标的响应")
			continue
		}
		if res.Quote != nil {
			q := *res.Quote
			if q.Alias == "" {
				q.Alias = AliasFor(sym)
			}
			st := t.feeds[sym]
			if st == nil {
				st = &feedState{}
				t.feeds[sym] = st
			}
			st.quote = &q
			st.receivedAt = now
			st.err = ""
			st.failures = 0
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
		st.failures++
	}
}

func (t *Tracker) FailAll(reason string) {
	for _, sym := range t.symbols {
		t.Fail(sym, reason)
	}
}

func (t *Tracker) Views(now time.Time, intervalSeconds int) []View {
	if intervalSeconds < 2 {
		intervalSeconds = 2
	}
	maxAge := math.Max(float64(6*intervalSeconds)+float64(TimeoutMillis)/1000.0, 30.0)
	out := make([]View, 0, len(t.symbols))
	for _, sym := range t.symbols {
		st := t.feeds[sym]
		if st == nil {
			st = &feedState{err: "等待 " + AliasFor(sym) + " 实时行情"}
		}
		var agePtr *float64
		reason := st.err
		healthy := false
		state := StateClosed
		if st.quote != nil {
			if st.quote.State != "" {
				state = st.quote.State
			} else {
				state = StateRegular
			}
		}
		if st.quote != nil && !st.receivedAt.IsZero() {
			age := math.Max(0, now.Sub(st.receivedAt).Seconds())
			agePtr = &age
			if age > maxAge {
				if reason == "" {
					reason = "超过预期刷新时间，数据源失联"
				}
				healthy = false
			} else if st.failures < 3 {
				healthy = true
				reason = ""
			}
		}
		var qCopy *Quote
		if st.quote != nil {
			c := *st.quote
			qCopy = &c
		}
		out = append(out, View{
			Symbol:      sym,
			Alias:       AliasFor(sym),
			State:       state,
			Quote:       qCopy,
			Healthy:     healthy,
			Reason:      reason,
			ReceivedAge: agePtr,
		})
	}
	return out
}
