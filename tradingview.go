package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

type TradingView struct {
	Conn           *websocket.Conn
	Dialer         *websocket.Dialer
	Url            string
	RequestHeader  http.Header
	OnConnected    func(tv TradingView)
	OnConnectError func(err error, tv TradingView)
	OnDisconnected func(err error, tv TradingView)
	OnUpdate       func(state TradingViewQuote)
	OnEvent        func(event *TradingViewEvent)
	IsConnected    bool
	SessionID      string
	Timeout        time.Duration
	sendMutex      *sync.Mutex
	recvMutex      *sync.Mutex
}

func NewTradingView() TradingView {
	return TradingView{
		Url: "wss://data.tradingview.com/socket.io/websocket",
		RequestHeader: http.Header{
			"Origin": []string{
				"https://data.tradingview.com/",
			},
		},
		Dialer:    &websocket.Dialer{},
		Timeout:   0,
		sendMutex: &sync.Mutex{},
		recvMutex: &sync.Mutex{},
	}
}

func (tv *TradingView) Connect() {
	var err error
	var resp *http.Response

	tv.Dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	tv.Conn, resp, err = tv.Dialer.Dial(tv.Url, tv.RequestHeader)

	if err != nil {
		log.Errorf("Error while connecting to TradingView: %v", err)
		if resp != nil {
			log.Errorf("HTTP Response %d status: %s", resp.StatusCode, resp.Status)
		}
		tv.IsConnected = false
		if tv.OnConnectError != nil {
			tv.OnConnectError(err, *tv)
		}
		return
	}

	log.Info("Connected to TradingView")

	tv.SessionID = CreateSessionID("qs_")

	tv.Send("set_data_quality", []interface{}{"low"})
	tv.Send("set_auth_token", []interface{}{"unauthorized_user_token"})
	tv.Send("quote_create_session", []interface{}{tv.SessionID})
	tv.Send("quote_set_fields", []interface{}{tv.SessionID, "listed_exchange",
		"ch", "chp", "rtc", "rch", "rchp", "lp", "is_tradable",
		"short_name", "description", "currency_code", "current_session",
		"status", "type", "update_mode", "fundamentals", "pro_name",
		"original_name",
	})

	tv.IsConnected = true
	if tv.OnConnected != nil {
		tv.OnConnected(*tv)
	}

	defaultCloseHandler := tv.Conn.CloseHandler()
	tv.Conn.SetCloseHandler(func(code int, text string) error {
		result := defaultCloseHandler(code, text)
		log.Info("Disconnected from server", result)
		return result
	})

	go tv.Loop()
}

func (tv *TradingView) Watch(symbol string) {
	if _, ok := watchlist.Watching[symbol]; ok {
		return
	} else {
		watchlist.Watching[symbol] = TradingViewQuote{}
	}

	if tv.IsConnected {
		tv.Send("quote_add_symbols", []interface{}{
			tv.SessionID,
			symbol,
			map[string][]string{
				"flags": {
					"force_permission",
				},
			},
		})
		tv.Send("quote_fast_symbols", []interface{}{
			tv.SessionID,
			symbol,
		})
	}
}

func (tv *TradingView) Loop() {
	for {
		tv.recvMutex.Lock()
		_, message, err := tv.Conn.ReadMessage()
		tv.recvMutex.Unlock()

		if err != nil {
			log.Debug("read:", err)
			slackapi.PostMessage(
				"U2PANET2T",
				slack.MsgOptionBlocks(
					slack.NewSectionBlock(
						slack.NewTextBlockObject(slack.MarkdownType, "Lost connection to TradingView; quitting.", false, false),
						nil,
						nil,
					),
				),
			)
			panic("Lost connection; panic to restart")
		}

		tv.MessageHandler(string(message))
	}
}

func (tv *TradingView) MessageHandler(message string) {
	re := regexp.MustCompile("~m~[0-9]+~m~")
	lines := re.Split(message, -1)

	for i := range lines {
		if lines[i] != "" {
			if matched := re.MatchString(lines[i]); matched {
				tv.SendSigned(lines[i])
				continue
			}

			if err := ParseTradingViewEvent(lines[i]); err != nil {
				log.Errorf("Error parsing incoming message: %v", err)
			}
		}
	}
}

func (tv *TradingView) Send(method string, params []interface{}) {
	data := TradingViewRequest{
		Method: method,
		Params: params,
	}

	message, err := json.Marshal(data)

	if err != nil {
		log.Errorf("Error creating Signed Message: %v", err)
		return
	}

	tv.SendSigned(string(message))
}

func (tv *TradingView) SendSigned(message string) {
	message = fmt.Sprintf("~m~%d~m~%s", len(message), message)

	err := tv.SendRaw(message)
	if err != nil {
		log.Errorf("Error sending message: %v", err)
		return
	}
}

func (tv *TradingView) SendRaw(message string) error {
	tv.sendMutex.Lock()
	err := tv.Conn.WriteMessage(websocket.TextMessage, []byte(message))
	tv.sendMutex.Unlock()
	return err
}

func ParseTradingViewEvent(line string) error {
	var err error

	event := &TradingViewEvent{}
	err = json.Unmarshal([]byte(line), event)
	if err != nil {
		return err
	}

	if event.Type == "" {
		return nil
	}

	event.SessionID = string(event.RawData[0])

	switch event.Type {
	case "qsd":
		if len(event.RawData) != 2 {
			return fmt.Errorf("unrecognized QSD event message format")
		}

		envelope := &TradingViewEventQSDEnvelope{}
		err = json.Unmarshal([]byte(event.RawData[1]), envelope)
		if err != nil {
			return err
		}

		symbol := envelope.Symbol
		re := regexp.MustCompile(`([A-Z]+)$`)
		parsed := re.FindStringSubmatch(symbol)
		if len(parsed) == 2 {
			symbol = parsed[0]
		}

		var qsd TradingViewQuote
		if quote, ok := watchlist.GetCurrent(symbol); ok {
			qsd = quote
		}

		err = json.Unmarshal([]byte(envelope.Data), &qsd)
		if err != nil {
			return fmt.Errorf("error parsing quote data: %v", err)
		}

		log.Infof("QSD line %v", line)

		if qsd.OriginalName != "" {
			if _, ok := watchlist.Watching[qsd.OriginalName]; !ok {
				tradingview.Watch(qsd.OriginalName)
			}
		}

		if qsd.ProName != "" {
			if _, ok := watchlist.Watching[qsd.ProName]; !ok {
				tradingview.Watch(qsd.ProName)
			}
		}

		watchlist.Update(symbol, qsd)
	default:
		log.Infof("Unknown TV payload: %v", line)
		return nil
	}

	return nil
}

func CreateSessionID(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	var runes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, 12)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}

	return prefix + string(b)
}
