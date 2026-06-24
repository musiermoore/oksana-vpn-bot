package commands

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"oksana-vpn-telegram-bot/pkg/api"
	"strings"
	"testing"
	"time"

	telebot "gopkg.in/telebot.v4"
)

type fakeContext struct {
	sender       *telebot.User
	chat         *telebot.Chat
	msg          *telebot.Message
	cb           *telebot.Callback
	bot          telebot.API
	sent         []string
	documents    []*telebot.Document
	replyMarkups []*telebot.ReplyMarkup
	parseModes   []telebot.ParseMode
}

func (f *fakeContext) Bot() telebot.API                                { return f.bot }
func (f *fakeContext) Update() telebot.Update                          { return telebot.Update{} }
func (f *fakeContext) Message() *telebot.Message                       { return f.msg }
func (f *fakeContext) Callback() *telebot.Callback                     { return f.cb }
func (f *fakeContext) Query() *telebot.Query                           { return nil }
func (f *fakeContext) InlineResult() *telebot.InlineResult             { return nil }
func (f *fakeContext) ShippingQuery() *telebot.ShippingQuery           { return nil }
func (f *fakeContext) PreCheckoutQuery() *telebot.PreCheckoutQuery     { return nil }
func (f *fakeContext) Payment() *telebot.Payment                       { return nil }
func (f *fakeContext) Poll() *telebot.Poll                             { return nil }
func (f *fakeContext) PollAnswer() *telebot.PollAnswer                 { return nil }
func (f *fakeContext) ChatMember() *telebot.ChatMemberUpdate           { return nil }
func (f *fakeContext) ChatJoinRequest() *telebot.ChatJoinRequest       { return nil }
func (f *fakeContext) Migration() (int64, int64)                       { return 0, 0 }
func (f *fakeContext) Topic() *telebot.Topic                           { return nil }
func (f *fakeContext) Boost() *telebot.BoostUpdated                    { return nil }
func (f *fakeContext) BoostRemoved() *telebot.BoostRemoved             { return nil }
func (f *fakeContext) PurchasedPaidMedia() *telebot.PaidMediaPurchased { return nil }
func (f *fakeContext) Sender() *telebot.User                           { return f.sender }
func (f *fakeContext) Chat() *telebot.Chat                             { return f.chat }
func (f *fakeContext) Recipient() telebot.Recipient                    { return f.sender }
func (f *fakeContext) Text() string                                    { return "" }
func (f *fakeContext) ThreadID() int                                   { return 0 }
func (f *fakeContext) Entities() telebot.Entities                      { return nil }
func (f *fakeContext) Data() string                                    { return "" }
func (f *fakeContext) Args() []string                                  { return nil }
func (f *fakeContext) Send(what interface{}, opts ...interface{}) error {
	for _, opt := range opts {
		switch value := opt.(type) {
		case *telebot.ReplyMarkup:
			f.replyMarkups = append(f.replyMarkups, value)
		case *telebot.SendOptions:
			if value != nil && value.ReplyMarkup != nil {
				f.replyMarkups = append(f.replyMarkups, value.ReplyMarkup)
			}
			if value != nil {
				f.parseModes = append(f.parseModes, value.ParseMode)
			}
		}
	}

	if text, ok := what.(string); ok {
		f.sent = append(f.sent, text)
		return nil
	}

	if document, ok := what.(*telebot.Document); ok {
		f.documents = append(f.documents, document)
		f.sent = append(f.sent, "")
		return nil
	}

	f.sent = append(f.sent, "")
	return nil
}
func (f *fakeContext) SendAlbum(a telebot.Album, opts ...interface{}) error      { return nil }
func (f *fakeContext) Reply(what interface{}, opts ...interface{}) error         { return nil }
func (f *fakeContext) Forward(msg telebot.Editable, opts ...interface{}) error   { return nil }
func (f *fakeContext) ForwardTo(to telebot.Recipient, opts ...interface{}) error { return nil }
func (f *fakeContext) Edit(what interface{}, opts ...interface{}) error          { return nil }
func (f *fakeContext) EditCaption(caption string, opts ...interface{}) error     { return nil }
func (f *fakeContext) EditOrSend(what interface{}, opts ...interface{}) error    { return nil }
func (f *fakeContext) EditOrReply(what interface{}, opts ...interface{}) error   { return nil }
func (f *fakeContext) Delete() error                                             { return nil }
func (f *fakeContext) DeleteAfter(d time.Duration) *time.Timer                   { return nil }
func (f *fakeContext) Notify(action telebot.ChatAction) error                    { return nil }
func (f *fakeContext) Ship(what ...interface{}) error                            { return nil }
func (f *fakeContext) Accept(errorMessage ...string) error                       { return nil }
func (f *fakeContext) Answer(resp *telebot.QueryResponse) error                  { return nil }
func (f *fakeContext) Respond(resp ...*telebot.CallbackResponse) error           { return nil }
func (f *fakeContext) RespondText(text string) error                             { return nil }
func (f *fakeContext) RespondAlert(text string) error                            { return nil }
func (f *fakeContext) Get(key string) interface{}                                { return nil }
func (f *fakeContext) Set(key string, val interface{})                           {}

type fakeBotAPI struct {
	telebot.API
	ctx           *fakeContext
	nextMessageID int
}

func (f *fakeBotAPI) Send(to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error) {
	for _, opt := range opts {
		switch value := opt.(type) {
		case *telebot.ReplyMarkup:
			f.ctx.replyMarkups = append(f.ctx.replyMarkups, value)
		case *telebot.SendOptions:
			if value != nil && value.ReplyMarkup != nil {
				f.ctx.replyMarkups = append(f.ctx.replyMarkups, value.ReplyMarkup)
			}
			if value != nil {
				f.ctx.parseModes = append(f.ctx.parseModes, value.ParseMode)
			}
		}
	}

	if text, ok := what.(string); ok {
		f.ctx.sent = append(f.ctx.sent, text)
	}

	chat := f.ctx.chat
	if recipientChat, ok := to.(*telebot.Chat); ok {
		chat = recipientChat
	}

	if chat == nil {
		chat = &telebot.Chat{}
	}

	return &telebot.Message{
		ID:   f.nextMessageID,
		Chat: chat,
	}, nil
}

func decodeRequestBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	defer r.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("decode request body: %v", err)
	}

	return payload
}

func newTestContext() *fakeContext {
	ctx := &fakeContext{
		sender: &telebot.User{ID: 777, Username: "oksana", FirstName: "Oksana"},
		chat:   &telebot.Chat{ID: 777},
	}

	ctx.bot = &fakeBotAPI{
		ctx:           ctx,
		nextMessageID: 999,
	}

	return ctx
}

func setupAPIEnv(t *testing.T, server *httptest.Server) {
	t.Helper()

	t.Setenv("API_URL", server.URL+"/")
	t.Setenv("API_BASIC_AUTH_USER", "")
	t.Setenv("API_BASIC_AUTH_PASSWORD", "")
}

func TestShowStartMenuRegistersMissingUser(t *testing.T) {
	var calls []string
	welcomeText := "Привет из админки"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)

		switch len(calls) {
		case 1:
			if r.URL.Path != "/users/777/registration-status" {
				t.Fatalf("unexpected first path: %s", r.URL.Path)
			}
			http.Error(w, `{"message":"user not found"}`, http.StatusNotFound)
		case 2:
			if r.URL.Path != "/users/777/save-id" || r.Method != http.MethodPost {
				t.Fatalf("unexpected second request: %s %s", r.Method, r.URL.Path)
			}
			payload := decodeRequestBody(t, r)
			if payload["telegram"] != "oksana" || payload["name"] != "Oksana" {
				t.Fatalf("unexpected payload: %#v", payload)
			}
			w.WriteHeader(http.StatusNoContent)
		case 3:
			if r.URL.Path != "/users/777/registration-status" {
				t.Fatalf("unexpected third path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"registered":true,"has_money_for_next_subscription_month":true,"welcome_text":"` + welcomeText + `"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	if err := showStartMenu(ctx); err != nil {
		t.Fatalf("showStartMenu returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if ctx.sent[0] != welcomeText {
		t.Fatalf("unexpected start message: %q", ctx.sent[0])
	}
	if len(ctx.parseModes) != 1 || ctx.parseModes[0] != telebot.ModeHTML {
		t.Fatalf("unexpected parse modes: %#v", ctx.parseModes)
	}
}

func TestShowStartMenuForRegisteredUserSkipsRegistration(t *testing.T) {
	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/users/777/registration-status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"registered":true,"has_money_for_next_subscription_month":true}}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	if err := showStartMenu(ctx); err != nil {
		t.Fatalf("showStartMenu returned error: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 API call, got %d", calls)
	}
	if len(ctx.sent) != 1 || ctx.sent[0] != "👋 Добро пожаловать в <b>OksanaVPN</b>!\n\nВыберите нужное действие в меню ниже и начните пользоваться VPN уже через пару минут 🚀" {
		t.Fatalf("unexpected messages: %#v", ctx.sent)
	}
	if len(ctx.parseModes) != 1 || ctx.parseModes[0] != telebot.ModeHTML {
		t.Fatalf("unexpected parse modes: %#v", ctx.parseModes)
	}
}

func TestShowStartMenuFallsBackToDefaultWelcomeText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/777/registration-status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"registered":true,"has_money_for_next_subscription_month":true,"welcome_text":"   "}}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	if err := showStartMenu(ctx); err != nil {
		t.Fatalf("showStartMenu returned error: %v", err)
	}

	if len(ctx.sent) != 1 || ctx.sent[0] != "👋 Добро пожаловать в <b>OksanaVPN</b>!\n\nВыберите нужное действие в меню ниже и начните пользоваться VPN уже через пару минут 🚀" {
		t.Fatalf("unexpected messages: %#v", ctx.sent)
	}
	if len(ctx.parseModes) != 1 || ctx.parseModes[0] != telebot.ModeHTML {
		t.Fatalf("unexpected parse modes: %#v", ctx.parseModes)
	}
}

func TestHandleHelpCommandShowsHelpSectionsMenu(t *testing.T) {
	ctx := newTestContext()

	if err := HandleHelpCommand(ctx); err != nil {
		t.Fatalf("HandleHelpCommand returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if ctx.sent[0] != "Выберите раздел помощи:" {
		t.Fatalf("unexpected help message: %q", ctx.sent[0])
	}
}

func TestGetHelpMenuKeyboardContainsExpectedButtons(t *testing.T) {
	kb := getHelpMenuKeyboard()

	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}

	if kb.InlineKeyboard[0][0].Text != "WG" || kb.InlineKeyboard[0][0].Unique != "help|wg" {
		t.Fatalf("unexpected WG button: %#v", kb.InlineKeyboard[0][0])
	}
	if kb.InlineKeyboard[0][1].Text != "VLESS" || kb.InlineKeyboard[0][1].Unique != "help|vless" {
		t.Fatalf("unexpected VLESS button: %#v", kb.InlineKeyboard[0][1])
	}
	if kb.InlineKeyboard[1][0].Text != "Клиенты" || kb.InlineKeyboard[1][0].Unique != "help|clients" {
		t.Fatalf("unexpected clients button: %#v", kb.InlineKeyboard[1][0])
	}
}

func TestGetHelpWGDetailsKeyboardContainsClientsShortcut(t *testing.T) {
	kb := getHelpWGDetailsKeyboard()

	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].Text != "WG клиенты" || kb.InlineKeyboard[0][0].Unique != "help|clients|wg" {
		t.Fatalf("unexpected WG clients shortcut: %#v", kb.InlineKeyboard[0][0])
	}
}

func TestGetHelpVLESSDetailsKeyboardContainsClientsShortcut(t *testing.T) {
	kb := getHelpVLESSDetailsKeyboard()

	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].Text != "VLESS клиенты" || kb.InlineKeyboard[0][0].Unique != "help|clients|vless" {
		t.Fatalf("unexpected VLESS clients shortcut: %#v", kb.InlineKeyboard[0][0])
	}
}

func TestGetHelpClientsMenuKeyboardContainsExpectedButtons(t *testing.T) {
	kb := getHelpClientsMenuKeyboard()

	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}

	if kb.InlineKeyboard[0][0].Text != "WG клиенты" || kb.InlineKeyboard[0][0].Unique != "help|clients|wg" {
		t.Fatalf("unexpected WG clients button: %#v", kb.InlineKeyboard[0][0])
	}
	if kb.InlineKeyboard[1][0].Text != "VLESS клиенты" || kb.InlineKeyboard[1][0].Unique != "help|clients|vless" {
		t.Fatalf("unexpected VLESS clients button: %#v", kb.InlineKeyboard[1][0])
	}
}

func TestGetHelpWGClientsKeyboardContainsLinks(t *testing.T) {
	kb := getHelpWGClientsKeyboard()

	if len(kb.InlineKeyboard) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(kb.InlineKeyboard))
	}

	if kb.InlineKeyboard[0][0].URL != "https://apps.apple.com/us/app/amneziavpn/id1600529900" {
		t.Fatalf("unexpected Amnezia iOS link: %#v", kb.InlineKeyboard[0][0])
	}
	if kb.InlineKeyboard[0][1].URL != "https://play.google.com/store/apps/details?id=org.amnezia.awg" {
		t.Fatalf("unexpected Amnezia Android link: %#v", kb.InlineKeyboard[0][1])
	}
	if kb.InlineKeyboard[1][0].URL != "https://amnezia.org/ru/downloads" {
		t.Fatalf("unexpected AmneziaWG PC link: %#v", kb.InlineKeyboard[0][1])
	}
	if kb.InlineKeyboard[2][0].URL != "https://apps.apple.com/us/app/wireguard/id1441195209" {
		t.Fatalf("unexpected WireGuard iOS link: %#v", kb.InlineKeyboard[2][0])
	}
	if kb.InlineKeyboard[2][1].URL != "https://play.google.com/store/apps/details?id=com.wireguard.android&hl=ru" {
		t.Fatalf("unexpected WireGuard Android link: %#v", kb.InlineKeyboard[2][1])
	}
	if kb.InlineKeyboard[3][0].URL != "https://www.wireguard.com/" {
		t.Fatalf("unexpected WireGuard site link: %#v", kb.InlineKeyboard[3][0])
	}
}

func TestGetHelpVLESSClientsKeyboardContainsLinks(t *testing.T) {
	kb := getHelpVLESSClientsKeyboard()

	if len(kb.InlineKeyboard) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(kb.InlineKeyboard))
	}

	if kb.InlineKeyboard[0][0].URL != "https://apps.apple.com/us/app/v2raytun/id6476628951" {
		t.Fatalf("unexpected v2raytun iOS link: %#v", kb.InlineKeyboard[0][0])
	}
	if kb.InlineKeyboard[0][1].URL != "https://play.google.com/store/apps/details?id=com.v2raytun.android" {
		t.Fatalf("unexpected v2raytun Android link: %#v", kb.InlineKeyboard[0][1])
	}
	if kb.InlineKeyboard[1][0].URL != "https://v2raytun.com/#download" {
		t.Fatalf("unexpected v2raytun site link: %#v", kb.InlineKeyboard[1][0])
	}
	if kb.InlineKeyboard[2][0].URL != "https://play.google.com/store/apps/details?id=com.happproxy" {
		t.Fatalf("unexpected Happ Android link: %#v", kb.InlineKeyboard[2][0])
	}
	if kb.InlineKeyboard[2][1].URL != "https://apps.apple.com/us/app/happ-proxy-utility/id6504287215" {
		t.Fatalf("unexpected Happ iOS link: %#v", kb.InlineKeyboard[2][1])
	}
	if kb.InlineKeyboard[3][0].URL != "https://www.happ.su/main/ru" {
		t.Fatalf("unexpected Happ site link: %#v", kb.InlineKeyboard[3][0])
	}
}

func TestGetConfigsKeyboardUsesCompactCallbackData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/777/configs/wireguard" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"configs":[{"id":571,"name":"musiermoore-rossiia-qkayasstnmdlzued"}]}}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	kb, _, err := getConfigsKeyboard(ctx, "wireguard")
	if err != nil {
		t.Fatalf("getConfigsKeyboard returned error: %v", err)
	}

	if got := kb.InlineKeyboard[0][0].Unique; got != "config|wireguard|571" {
		t.Fatalf("unexpected callback data: %q", got)
	}
}

func TestHandleChoosingConfigBuildsCompactActionButtonsAndResolvesName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/777/configs/wireguard" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"configs":[{"id":571,"name":"musiermoore-rossiia-qkayasstnmdlzued"}]}}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	ctx.cb = &telebot.Callback{Data: "\fconfig|wireguard|571"}

	if err := HandleChoosingConfig(ctx); err != nil {
		t.Fatalf("HandleChoosingConfig returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if got := ctx.sent[0]; got != "Выбери действие для конфига musiermoore-rossiia-qkayasstnmdlzued" {
		t.Fatalf("unexpected message: %q", got)
	}
	if len(ctx.replyMarkups) != 1 {
		t.Fatalf("expected 1 reply markup, got %d", len(ctx.replyMarkups))
	}
	if got := ctx.replyMarkups[0].InlineKeyboard[0][0].Unique; got != "action_config_qr|wireguard|571" {
		t.Fatalf("unexpected QR callback data: %q", got)
	}
	if got := ctx.replyMarkups[0].InlineKeyboard[0][1].Unique; got != "action_config_file|wireguard|571" {
		t.Fatalf("unexpected file callback data: %q", got)
	}
}

func TestHandleDownloadConfigUsesServerFileNameFromContentDisposition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/777/configs/wireguard":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"configs":[{"id":571,"name":"My iPhone"}]}}`))
		case "/users/777/configs/wireguard/571/download":
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Disposition", `attachment; filename="server-name.conf"`)
			_, _ = w.Write([]byte("config-data"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	ctx.cb = &telebot.Callback{Data: "\faction_config_file|wireguard|571"}

	if err := HandleDownloadConfig(ctx); err != nil {
		t.Fatalf("HandleDownloadConfig returned error: %v", err)
	}

	if len(ctx.documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(ctx.documents))
	}
	if ctx.documents[0].FileName != "server-name.conf" {
		t.Fatalf("unexpected document filename: %q", ctx.documents[0].FileName)
	}
}

func TestHandleDownloadConfigFallsBackToGeneratedFileName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/777/configs/wireguard":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"configs":[{"id":571,"name":"My iPhone #1"}]}}`))
		case "/users/777/configs/wireguard/571/download":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("config-data"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	ctx.cb = &telebot.Callback{Data: "\faction_config_file|wireguard|571"}

	if err := HandleDownloadConfig(ctx); err != nil {
		t.Fatalf("HandleDownloadConfig returned error: %v", err)
	}

	if len(ctx.documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(ctx.documents))
	}
	if ctx.documents[0].FileName != "MyiPhone1.conf" {
		t.Fatalf("unexpected fallback filename: %q", ctx.documents[0].FileName)
	}
}

func TestHelpTextsLoadedFromFiles(t *testing.T) {
	if !strings.Contains(getHelpWGMessage(), "Настройка WG") {
		t.Fatalf("unexpected WG help text: %q", getHelpWGMessage())
	}
	if !strings.Contains(getHelpVLESSMessage(), "Настройка VLESS") {
		t.Fatalf("unexpected VLESS help text: %q", getHelpVLESSMessage())
	}
}

func TestHandleSubscriptionWithWrappedAPIResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/777/balance":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"amount":150.5,"debt":20}}`))
		case "/users/777/registration-status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"registered":true,"active_subscription_end_date":"2026-06-01","has_money_for_next_subscription_month":false}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()

	if err := HandleSubscription(ctx); err != nil {
		t.Fatalf("HandleSubscription returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}

	message := ctx.sent[0]
	if !strings.Contains(message, "150\\.50") {
		t.Fatalf("expected escaped balance in message, got %q", message)
	}
	if !strings.Contains(message, "01\\.06\\.2026") {
		t.Fatalf("expected escaped end date in message, got %q", message)
	}
}

func TestGetSubscriptionPackageKeyboardContainsExpectedButtons(t *testing.T) {
	kb := getSubscriptionPackageKeyboard([]api.SubscriptionPackage{
		{Month: 1, Price: 400, DiscountPercent: 0},
		{Month: 3, Price: 1080, DiscountPercent: 10},
		{Month: 6, Price: 1920, DiscountPercent: 20},
		{Month: 12, Price: 3360, DiscountPercent: 30},
	})

	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].Text != "1 месяц - 400 ₽" || kb.InlineKeyboard[0][0].Unique != "submit_payment_request|1" {
		t.Fatalf("unexpected first package button: %#v", kb.InlineKeyboard[0][0])
	}
	if kb.InlineKeyboard[1][1].Text != "12 месяцев - 3360 ₽" || kb.InlineKeyboard[1][1].Unique != "submit_payment_request|12" {
		t.Fatalf("unexpected last package button: %#v", kb.InlineKeyboard[1][1])
	}
}

func TestHandleSendPaymentRequestShowsBackendPrices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/777/subscription-packages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"month": 1, "price": 400, "discount_percent": 0},
				{"month": 3, "price": 1080, "discount_percent": 10},
				{"month": 6, "price": 1920, "discount_percent": 20},
				{"month": 12, "price": 3360, "discount_percent": 30}
			]
		}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	if err := HandleSendPaymentRequest(ctx); err != nil {
		t.Fatalf("HandleSendPaymentRequest returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if !strings.Contains(ctx.sent[0], "1 месяц - 400 ₽") {
		t.Fatalf("expected exact package price in message, got %q", ctx.sent[0])
	}
	if !strings.Contains(ctx.sent[0], "3 месяца - 1080 ₽ (скидка 10%)") {
		t.Fatalf("expected discount percent in message, got %q", ctx.sent[0])
	}
	if !strings.Contains(ctx.sent[0], "12 месяцев - 3360 ₽") {
		t.Fatalf("expected full package list in message, got %q", ctx.sent[0])
	}
}

func TestHandleChooseSubscriptionPackageRejectsInvalidMonth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/777/subscription-packages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"month": 1, "price": 400, "discount_percent": 0},
				{"month": 3, "price": 1080, "discount_percent": 10},
				{"month": 6, "price": 1920, "discount_percent": 20},
				{"month": 12, "price": 3360, "discount_percent": 30}
			]
		}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	ctx.cb = &telebot.Callback{Data: "choose_subscription_package|2"}

	if err := HandleChooseSubscriptionPackage(ctx); err != nil {
		t.Fatalf("HandleChooseSubscriptionPackage returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if !strings.Contains(ctx.sent[0], "Не удалось найти выбранный вариант подписки") {
		t.Fatalf("unexpected message: %q", ctx.sent[0])
	}
}

func TestHandleSubmitPaymentRequestActivatedMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/777/transactions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		payload := decodeRequestBody(t, r)
		if payload["month"] != float64(3) {
			t.Fatalf("unexpected month payload: %#v", payload["month"])
		}
		if payload["bank"] != "tbank" {
			t.Fatalf("unexpected bank payload: %#v", payload["bank"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"activated","message":"Подписка активирована до 18.11.2026.","end_date":"2026-11-18","formatted_end_date":"18.11.2026"}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	ctx.cb = &telebot.Callback{Data: "submit_payment_request|3"}

	if err := HandleSubmitPaymentRequest(ctx); err != nil {
		t.Fatalf("HandleSubmitPaymentRequest returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if !strings.Contains(ctx.sent[0], "Подписка активирована до 18.11.2026.") {
		t.Fatalf("unexpected activation message: %q", ctx.sent[0])
	}
	if !strings.Contains(ctx.sent[0], "Ничего дополнительно оплачивать не нужно.") {
		t.Fatalf("expected no-payment hint, got %q", ctx.sent[0])
	}
}

func TestHandleSubmitPaymentRequestDepositRequiredMessage(t *testing.T) {
	var calls []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)

		switch len(calls) {
		case 1:
			if r.URL.Path != "/users/777/transactions" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"deposit_required","message":"Для активации подписки необходимо оплатить 520 ₽.","deposit_amount":520.0,"transaction_id":1,"invoice_id":456,"payment_id":"uuid","payment_status":"pending","confirmation_url":"https://pay.example/confirm"}`))
		case 2:
			if r.URL.Path != "/users/777/transactions/1/telegram-message" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodPatch {
				t.Fatalf("unexpected method: %s", r.Method)
			}

			payload := decodeRequestBody(t, r)
			if payload["telegram_chat_id"] != float64(777) {
				t.Fatalf("unexpected telegram chat id: %#v", payload["telegram_chat_id"])
			}
			if payload["telegram_message_id"] != float64(999) {
				t.Fatalf("unexpected telegram message id: %#v", payload["telegram_message_id"])
			}

			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	ctx.cb = &telebot.Callback{Data: "submit_payment_request|6"}

	if err := HandleSubmitPaymentRequest(ctx); err != nil {
		t.Fatalf("HandleSubmitPaymentRequest returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if !strings.Contains(ctx.sent[0], "Для активации подписки необходимо оплатить 520 ₽.") {
		t.Fatalf("unexpected deposit-required message: %q", ctx.sent[0])
	}
	if !strings.Contains(ctx.sent[0], "Чтобы перейти к оплате нажмите на кнопку \"Перейти к оплате картой / СБП\".") {
		t.Fatalf("expected payment button hint, got %q", ctx.sent[0])
	}
	if !strings.Contains(ctx.sent[0], "После успешной оплаты подписка активируется автоматически.") {
		t.Fatalf("expected auto-activation explanation, got %q", ctx.sent[0])
	}
	if len(ctx.replyMarkups) != 1 {
		t.Fatalf("expected reply markup, got %d", len(ctx.replyMarkups))
	}
	if len(ctx.replyMarkups[0].InlineKeyboard) == 0 || len(ctx.replyMarkups[0].InlineKeyboard[0]) == 0 {
		t.Fatalf("expected payment button, got %#v", ctx.replyMarkups[0])
	}
	if ctx.replyMarkups[0].InlineKeyboard[0][0].Text != "Перейти к оплате картой / СБП" {
		t.Fatalf("unexpected payment button text: %#v", ctx.replyMarkups[0].InlineKeyboard[0][0])
	}
	if ctx.replyMarkups[0].InlineKeyboard[0][0].URL != "https://pay.example/confirm" {
		t.Fatalf("unexpected payment button url: %#v", ctx.replyMarkups[0].InlineKeyboard[0][0])
	}
}

func TestBuildSubscriptionPurchaseMessageUsesFormattedEndDateFallback(t *testing.T) {
	message := buildSubscriptionPurchaseMessage(api.PaymentResponse{
		Status:           "activated",
		FormattedEndDate: "18.11.2026",
	})

	if !strings.Contains(message, "18.11.2026") {
		t.Fatalf("expected formatted end date in message, got %q", message)
	}
}

func TestBuildSubscriptionPurchaseMessageFormatsDepositAmount(t *testing.T) {
	message := buildSubscriptionPurchaseMessage(api.PaymentResponse{
		Status:        "deposit_required",
		DepositAmount: 520,
	})

	if !strings.Contains(message, "оплатить 520 ₽.") {
		t.Fatalf("expected formatted deposit amount in message, got %q", message)
	}
	if !strings.Contains(message, "Чтобы перейти к оплате нажмите на кнопку \"Перейти к оплате картой / СБП\".") {
		t.Fatalf("expected payment button hint in message, got %q", message)
	}
}

func TestHandleVlessCommandChecksAccessBeforeShowingMenu(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/777/configs/vless" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Недостаточно средств","type":"debt"}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	if err := HandleVlessCommand(ctx); err != nil {
		t.Fatalf("HandleVlessCommand returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if ctx.sent[0] != "Недостаточно средств" {
		t.Fatalf("unexpected message: %q", ctx.sent[0])
	}
}

func TestHandleVlessCommandShowsMenuWhenAccessIsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/777/configs/vless" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	setupAPIEnv(t, server)

	ctx := newTestContext()
	if err := HandleVlessCommand(ctx); err != nil {
		t.Fatalf("HandleVlessCommand returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}
	if ctx.sent[0] != "Выбери действие для VLESS:" {
		t.Fatalf("unexpected message: %q", ctx.sent[0])
	}
}

func TestBuildVlessLinkMessageUsesHappDeepLinkWhenPresent(t *testing.T) {
	linkResponse := api.VlessSubscriptionLinkResponse{
		Link:         "https://domain.com/connect?tg=encrypted%3Dvalue&i=second%3Dvalue",
		HappDeepLink: "happ://add-subscription?url=https%3A%2F%2Fbackend.example%2Fwrapped",
	}
	message := buildVlessLinkMessage(linkResponse)

	if !strings.Contains(message, "href=\""+html.EscapeString(linkResponse.HappDeepLink)+"\"") {
		t.Fatalf("expected Happ link in message, got %q", message)
	}
	if !strings.Contains(message, "<code>"+html.EscapeString(linkResponse.Link)+"</code>") {
		t.Fatalf("expected original link in code block, got %q", message)
	}
}

func TestBuildVlessLinkMessageUsesV2RayTunDeepLinkWhenPresent(t *testing.T) {
	linkResponse := api.VlessSubscriptionLinkResponse{
		Link:             "https://domain.com/connect?tg=encrypted%3Dvalue&i=second%3Dvalue",
		V2RayTunDeepLink: "v2raytun://install-sub?url=https%3A%2F%2Fbackend.example%2Fwrapped",
	}
	message := buildVlessLinkMessage(linkResponse)

	if !strings.Contains(message, "href=\""+html.EscapeString(linkResponse.V2RayTunDeepLink)+"\"") {
		t.Fatalf("expected V2RayTun link in message, got %q", message)
	}
}

func TestBuildVlessLinkMessageFallsBackToBaseLinkWhenDeepLinksMissing(t *testing.T) {
	linkResponse := api.VlessSubscriptionLinkResponse{
		Link: "https://domain.com/connect?tg=encrypted%3Dvalue&i=second%3Dvalue",
	}
	message := buildVlessLinkMessage(linkResponse)

	if !strings.Contains(message, "href=\""+html.EscapeString(linkResponse.Link)+"\"") {
		t.Fatalf("expected base link fallback in message, got %q", message)
	}
}
