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
	sender *telebot.User
	chat   *telebot.Chat
	sent   []string
}

func (f *fakeContext) Bot() telebot.API                                { return nil }
func (f *fakeContext) Update() telebot.Update                          { return telebot.Update{} }
func (f *fakeContext) Message() *telebot.Message                       { return nil }
func (f *fakeContext) Callback() *telebot.Callback                     { return nil }
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
	if text, ok := what.(string); ok {
		f.sent = append(f.sent, text)
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
	return &fakeContext{
		sender: &telebot.User{ID: 777, Username: "oksana", FirstName: "Oksana"},
		chat:   &telebot.Chat{ID: 777},
	}
}

func setupAPIEnv(t *testing.T, server *httptest.Server) {
	t.Helper()

	t.Setenv("API_URL", server.URL+"/")
	t.Setenv("API_BASIC_AUTH_USER", "")
	t.Setenv("API_BASIC_AUTH_PASSWORD", "")
}

func TestShowStartMenuRegistersMissingUser(t *testing.T) {
	var calls []string

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
			_, _ = w.Write([]byte(`{"data":{"registered":true,"has_money_for_next_subscription_month":true}}`))
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
	if ctx.sent[0] != "Привет! Выбери команду:" {
		t.Fatalf("unexpected start message: %q", ctx.sent[0])
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
	if len(ctx.sent) != 1 || ctx.sent[0] != "Привет! Выбери команду:" {
		t.Fatalf("unexpected messages: %#v", ctx.sent)
	}
}

func TestHandleBalanceWithWrappedAPIResponses(t *testing.T) {
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

	if err := HandleBalance(ctx); err != nil {
		t.Fatalf("HandleBalance returned error: %v", err)
	}

	if len(ctx.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(ctx.sent))
	}

	message := ctx.sent[0]
	if !strings.Contains(message, "150\\.50") {
		t.Fatalf("expected escaped balance in message, got %q", message)
	}
	if !strings.Contains(message, "2026\\-06\\-01") {
		t.Fatalf("expected escaped end date in message, got %q", message)
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
