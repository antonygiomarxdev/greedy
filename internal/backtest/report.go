package backtest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ReportFormatter interface {
	Format(r *Report) string
}

type textFormatter struct{}

func (f *textFormatter) Format(r *Report) string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════╗\n")
	sb.WriteString("║  GREEDY BACKTEST REPORT                  ║\n")
	sb.WriteString("╠══════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║ Strategy:    %-28s ║\n", r.Strategy))
	sb.WriteString(fmt.Sprintf("║ Symbol:      %-28s ║\n", r.Symbol))
	sb.WriteString(fmt.Sprintf("║ Period:      %-28s ║\n", r.Period))
	sb.WriteString("╠══════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║ Initial:     $%-27.2f ║\n", r.InitialBalance))
	sb.WriteString(fmt.Sprintf("║ Final:       $%-27.2f ║\n", r.FinalBalance))
	sb.WriteString(fmt.Sprintf("║ Return:      $%-21.2f (%5.2f%%)   ║\n", r.TotalReturn, r.TotalReturnPct))
	sb.WriteString("╠══════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║ Total Trades:%4d                        ║\n", r.TotalTrades))
	sb.WriteString(fmt.Sprintf("║ Winners:     %4d  (%5.1f%%)            ║\n", r.WinningTrades, r.WinRate))
	sb.WriteString(fmt.Sprintf("║ Losers:      %4d                        ║\n", r.LosingTrades))
	sb.WriteString("╠══════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║ Max DD:      %7.2f%%                    ║\n", r.MaxDrawdownPct))
	sb.WriteString(fmt.Sprintf("║ Sharpe:      %7.2f                      ║\n", r.SharpeRatio))
	sb.WriteString(fmt.Sprintf("║ Profit Fact: %7.2f                      ║\n", r.ProfitFactor))
	sb.WriteString("╚══════════════════════════════════════════╝\n")
	sb.WriteString(fmt.Sprintf("\nGenerated: %s\n", time.Now().Format(time.RFC3339)))

	return sb.String()
}

type jsonFormatter struct{}

func (f *jsonFormatter) Format(r *Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}

var formatters = map[string]ReportFormatter{
	"text": &textFormatter{},
	"json": &jsonFormatter{},
}

func FormatReport(r *Report, format string) (string, error) {
	f, ok := formatters[format]
	if !ok {
		f = formatters["text"]
	}
	return f.Format(r), nil
}

func (r *Report) FormatText() string { return formatters["text"].Format(r) }
func (r *Report) FormatJSON() string { return formatters["json"].Format(r) }
