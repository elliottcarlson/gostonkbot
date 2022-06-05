package main

var watchlist *WatchList

type WatchList struct {
	Watching      map[string]TradingViewQuote
	Notifications []*WatchListNotifications
}

type WatchListNotifications struct {
	Symbol string
	Action func(TradingViewQuote) bool
}

func InitWatchList() {
	watchlist = &WatchList{
		Watching: make(map[string]TradingViewQuote),
	}
}

func (w *WatchList) GetQuote(symbol string, callback func(TradingViewQuote) (shouldDelete bool)) {
	if _, ok := w.Watching[symbol]; ok {
		callback(w.Watching[symbol])
		return
	}

	w.OnUpdate(symbol, callback)
}

func (w *WatchList) GetCurrent(symbol string) (quote TradingViewQuote, ok bool) {
	if _, ok := w.Watching[symbol]; ok {
		return w.Watching[symbol], true
	}

	return TradingViewQuote{}, false
}

func (w *WatchList) OnUpdate(symbol string, callback func(TradingViewQuote) (shouldDelete bool)) {
	if _, ok := w.Watching[symbol]; !ok {
		tradingview.Watch(symbol)
	}

	notification := &WatchListNotifications{
		Symbol: symbol,
		Action: callback,
	}

	w.Notifications = append(w.Notifications, notification)
}

func (w *WatchList) Update(symbol string, quote TradingViewQuote) {
	w.Watching[symbol] = quote

	temp := w.Notifications[:0]
	for i := range w.Notifications {
		notification := w.Notifications[i]

		if notification.Symbol == symbol {
			shouldDelete := notification.Action(quote)

			if shouldDelete {
				continue
			}
		}

		temp = append(temp, notification)
	}

	w.Notifications = temp
}
