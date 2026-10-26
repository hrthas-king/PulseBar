// Package market 解析并校验 TradingView 的 ES / NQ 期货主力合约行情与状态机。
package market

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	MaxBody       = 2047
	ScannerHost   = "scanner.tradingview.com"
	ScannerPath   = "/futures/scan"
	TimeoutMillis = 1800
)

var (
	Symbols = []string{"ES", "NQ"}
	Tickers = map[string]string{
		"ES": "CME_MINI:ES1!",
		"NQ": "CME_MINI:NQ1!",
	}
	RequestPayload = []byte(`{"symbols":{"tickers":["CME_MINI:ES1!","CME_MINI:NQ1!"],"query":{"types":[]}},"columns":["name","description","type","close","change","change_abs","currency","update_mode"]}`)
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
	UpdateMode    string  `json:"update_mode"`
	Description   string  `json:"description"`
	RTTMs         float64 `json:"rtt_ms"`
}

func (q Quote) Previous() float64 {
	return q.Price - q.ChangeAbs
}

func (q Quote) DelaySeconds() (int, bool) {
	if q.UpdateMode == "streaming" {
		return 0, true
	}
	if strings.HasPrefix(q.UpdateMode, "delayed_streaming_") {
		secStr := strings.TrimPrefix(q.UpdateMode, "delayed_streaming_")
		if secStr != "" {
			if sec, err := strconv.Atoi(secStr); err == nil && sec >= 0 {
				return sec, true
			}
		}
	}
	return 0, false
}

func (q Quote) DelayText() string {
	sec, ok := q.DelaySeconds()
	if !ok {
		return "延时未知"
	}
	if sec == 0 {
		return "源标记 streaming（不作实时保证）"
	}
	if sec%60 == 0 {
		return fmt.Sprintf("延时 %d 分钟", sec/60)
	}
	return fmt.Sprintf("延时 %.1f 分钟", float64(sec)/60.0)
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
	if math.IsNaN(calc) || math.IsInf(calc, 0) || !isClose(calc, q.ChangePercent, 1e-6, 0.02) {
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

type tvResponse struct {
	Data []tvRow `json:"data"`
}

type tvRow struct {
	S string            `json:"s"`
	D []json.RawMessage `json:"d"`
}

func parseStrictFloat(raw json.RawMessage) (float64, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "true" || s == "false" || strings.HasPrefix(s, `"`) {
		return 0, fmt.Errorf("不是有效数字")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("数字超出范围")
	}
	return v, nil
}

func parseStrictString(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return "", fmt.Errorf("字符串为空")
	}
	return s, nil
}

// ParseTradingView 独立校验 ES 与 NQ；单个标的异常不影响另一个有效标的。
func ParseTradingView(body []byte, rttMs float64) (map[string]SymbolResult, error) {
	if len(body) > MaxBody {
		return nil, fmt.Errorf("行情响应超过 %d 字节上限", MaxBody)
	}
	var resp tvResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("TradingView JSON 解析失败: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("TradingView 行情列表格式无效")
	}
	results := make(map[string]SymbolResult, len(Symbols))
	for _, sym := range Symbols {
		ticker := Tickers[sym]
		var matches []tvRow
		for _, r := range resp.Data {
			if r.S == ticker {
				matches = append(matches, r)
			}
		}
		if len(matches) != 1 {
			results[sym] = SymbolResult{Err: "TradingView 数据异常: 缺少或重复标的"}
			continue
		}
		fields := matches[0].D
		if len(fields) != 8 {
			results[sym] = SymbolResult{Err: "TradingView 数据异常: 字段数量不匹配"}
			continue
		}
		name, err1 := parseStrictString(fields[0])
		desc, err2 := parseStrictString(fields[1])
		kind, err3 := parseStrictString(fields[2])
		price, err4 := parseStrictFloat(fields[3])
		pct, err5 := parseStrictFloat(fields[4])
		changeAbs, err6 := parseStrictFloat(fields[5])
		currency, err7 := parseStrictString(fields[6])
		mode, err8 := parseStrictString(fields[7])
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil || err7 != nil || err8 != nil {
			results[sym] = SymbolResult{Err: "TradingView 数据异常: 字段类型无效"}
			continue
		}
		expectedName := strings.SplitN(ticker, ":", 2)[1]
		if name != expectedName || kind != "futures" || currency != "USD" {
			results[sym] = SymbolResult{Err: "TradingView 数据异常: 合约、类型或币种不匹配"}
			continue
		}
		if desc == "" || len(desc) > 200 {
			results[sym] = SymbolResult{Err: "TradingView 数据异常: 合约名称无效"}
			continue
		}
		q := Quote{
			Symbol:        sym,
			Price:         price,
			ChangePercent: pct,
			ChangeAbs:     changeAbs,
			UpdateMode:    mode,
			Description:   desc,
			RTTMs:         rttMs,
		}
		if err := ValidateQuote(q); err != nil {
			results[sym] = SymbolResult{Err: "TradingView 数据异常: " + err.Error()}
			continue
		}
		results[sym] = SymbolResult{Quote: &q}
	}
	return results, nil
}

type View struct {
	Symbol      string   `json:"symbol"`
	Quote       *Quote   `json:"quote,omitempty"`
	Healthy     bool     `json:"healthy"`
	Reason      string   `json:"reason,omitempty"`
	ReceivedAge *float64 `json:"received_age_seconds,omitempty"`
}

func (v View) Tooltip() string {
	status := "数据正常（允许延时）"
	if !v.Healthy {
		status = "数据异常 — " + v.Reason
	}
	delay := "等待数据"
	if v.Quote != nil {
		delay = v.Quote.DelayText()
	}
	fetched := "尚未成功读取"
	if v.ReceivedAge != nil {
		fetched = fmt.Sprintf("%.1fs 前成功读取", *v.ReceivedAge)
	}
	if v.Quote != nil {
		return fmt.Sprintf("%s: %+.2f%% (最新 %.2f, 涨跌 %+.2f) | %s\n  TradingView %s；%s；%s",
			v.Symbol, v.Quote.ChangePercent, v.Quote.Price, v.Quote.ChangeAbs, status,
			Tickers[v.Symbol], delay, fetched)
	}
	return fmt.Sprintf("%s: %s\n  TradingView %s；%s；%s",
		v.Symbol, status, Tickers[v.Symbol], delay, fetched)
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
		t.feeds[sym] = &feedState{err: "等待 TradingView 行情"}
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
