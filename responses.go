package main

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/slack-go/slack"
	TVScanner "sublim.nl/stonkbot/tvscanner"
)

var scanner = TVScanner.New()

func StockQuoteBlock(quote TradingViewQuote) slack.MsgOption {
	emoji := ":green_up:"
	live_emoji := ":green_up:"
	if quote.Change < 0 {
		emoji = ":red_down:"
	}
	if quote.LiveChange < 0 {
		live_emoji = ":red_down:"
	}

	fields := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("%s (%s:%s)", quote.FullName, quote.Symbol, quote.Exchange), false, false),
		),
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("$%.4f      %s %+.2f (%+.2f%%)", quote.LastPrice, emoji, quote.Change, quote.ChangePercentage), true, false),
		),
	}

	switch quote.CurrentSession {
	case "pre_market":
		fields = append(fields, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("Pre-Market: $%.4f %s %+.2f (%+.2f%%)", quote.LivePrice, live_emoji, quote.LiveChange, quote.LiveChangePercentage), true, false),
			nil, nil,
		))
	case "post_market":
		fields = append(fields, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("Post-Market: $%.4f %s %+.2f (%+.2f%%)", quote.LivePrice, live_emoji, quote.LiveChange, quote.LiveChangePercentage), true, false),
			nil, nil,
		))
	}

	footer_items := []string{
		`<https://www.tradingview.com/chart/?symbol={{.Exchange}}%3A{{.Symbol}}|:tradingview:>`,
		`<https://finance.yahoo.com/quote/{{.Symbol}}|:yahoofinance:>`,
		`<https://www.google.com/finance/quote/{{.Symbol}}:{{.Exchange}}|:googlefinance:>`,
	}

	if analysis, err := scanner.GetAnalysis("america", quote.Exchange, quote.Symbol, "1h"); err == nil {
		recommendation := strings.Title(strings.ToLower(strings.ReplaceAll(analysis.Recommend.Summary, "_", " ")))
		footer_items = append(footer_items, fmt.Sprintf("_1 Day Technical Analysis: *%s* (Buy: %d, Neutral: %d, Sell: %d)_", recommendation, analysis.BuyCount, analysis.NeutralCount, analysis.SellCount))
	}

	t := template.Must(template.New("").Parse(strings.Join(footer_items[:], " | ")))
	footer := new(bytes.Buffer)
	t.Execute(footer, quote)

	fields = append(fields, slack.NewContextBlock("context", slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("%s", footer), false, false)))

	return slack.MsgOptionBlocks(fields...)
}
