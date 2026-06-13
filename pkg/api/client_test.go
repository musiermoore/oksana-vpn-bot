package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func withTestAPI(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	t.Setenv("API_URL", server.URL+"/")
	t.Setenv("API_BASIC_AUTH_USER", "")
	t.Setenv("API_BASIC_AUTH_PASSWORD", "")

	return &Client{
		userID:   123,
		username: "oksana_user",
		name:     "Oksana",
	}
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	defer r.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("decode request body: %v", err)
	}

	return payload
}

func TestRegisterUserUsesGroupedSaveIDRoute(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/save-id" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		payload := decodeBody(t, r)
		if _, exists := payload["telegram_id"]; exists {
			t.Fatalf("telegram_id should not be sent to save-id route")
		}
		if payload["telegram"] != "oksana_user" {
			t.Fatalf("unexpected telegram username: %#v", payload["telegram"])
		}
		if payload["name"] != "Oksana" {
			t.Fatalf("unexpected name: %#v", payload["name"])
		}

		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.RegisterUser(); err != nil {
		t.Fatalf("RegisterUser returned error: %v", err)
	}
}

func TestGetConfigsParsesResourcePayload(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/configs/vless" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "7", "attributes": {"name": "iPhone", "user_id": "123"}},
				{"id": 9, "name": "MacBook", "user_id": 123}
			]
		}`))
	})

	response, err := client.GetConfigs("vless")
	if err != nil {
		t.Fatalf("GetConfigs returned error: %v", err)
	}

	if len(response.Configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(response.Configs))
	}
	if response.Configs[0].ID != 7 || response.Configs[0].Name != "iPhone" || response.Configs[0].UserID != 123 {
		t.Fatalf("unexpected first config: %#v", response.Configs[0])
	}
	if response.Configs[1].ID != 9 || response.Configs[1].Name != "MacBook" || response.Configs[1].UserID != 123 {
		t.Fatalf("unexpected second config: %#v", response.Configs[1])
	}
}

func TestGetBalanceParsesWrappedAmountPayload(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/balance" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"amount":150.5,"debt":25}}`))
	})

	balance, err := client.GetBalance()
	if err != nil {
		t.Fatalf("GetBalance returned error: %v", err)
	}

	if balance.Amount != 150.5 || balance.Debt != 25 {
		t.Fatalf("unexpected balance: %#v", balance)
	}
}

func TestGetRegistrationStatusParsesResourceAttributesPayload(t *testing.T) {
	endDate := "2026-06-01"
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/registration-status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"attributes":{"registered":true,"active_subscription_end_date":"` + endDate + `","has_money_for_next_subscription_month":true}}}`))
	})

	status, err := client.GetRegistrationStatus()
	if err != nil {
		t.Fatalf("GetRegistrationStatus returned error: %v", err)
	}

	if !status.Registered || status.ActiveSubscriptionEndDate == nil || *status.ActiveSubscriptionEndDate != endDate || !status.HasMoneyForNextSubscriptionMonth {
		t.Fatalf("unexpected registration status: %#v", status)
	}
}

func TestSendPaymentRequestUsesMonthPayload(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/transactions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		payload := decodeBody(t, r)
		if _, exists := payload["amount"]; exists {
			t.Fatalf("amount should not be sent in subscription request payload")
		}
		if payload["month"] != float64(6) {
			t.Fatalf("unexpected month payload: %#v", payload["month"])
		}
		if payload["bank"] != "tbank" {
			t.Fatalf("unexpected bank payload: %#v", payload["bank"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"deposit_required","message":"Для активации подписки необходимо оплатить 520 ₽.","deposit_amount":520.0,"transaction_id":1,"invoice_id":456,"payment_id":"uuid","payment_status":"pending","confirmation_url":"https://pay.example/confirm"}`))
	})

	response, err := client.SendPaymentRequest(6, "tbank")
	if err != nil {
		t.Fatalf("SendPaymentRequest returned error: %v", err)
	}

	if response.Status != "deposit_required" || response.DepositAmount != 520 || response.TransactionID != 1 {
		t.Fatalf("unexpected payment response: %#v", response)
	}
	if response.InvoiceID != 456 || response.PaymentID != "uuid" || response.PaymentStatus != "pending" || response.ConfirmationURL != "https://pay.example/confirm" {
		t.Fatalf("unexpected yookassa fields: %#v", response)
	}
}

func TestSendPaymentRequestParsesActivatedResponse(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"activated","message":"Подписка активирована до 18.11.2026.","end_date":"2026-11-18","formatted_end_date":"18.11.2026"}`))
	})

	response, err := client.SendPaymentRequest(3, "tbank")
	if err != nil {
		t.Fatalf("SendPaymentRequest returned error: %v", err)
	}

	if response.Status != "activated" {
		t.Fatalf("unexpected status: %#v", response)
	}
	if response.FormattedEndDate != "18.11.2026" || response.EndDate != "2026-11-18" {
		t.Fatalf("unexpected dates: %#v", response)
	}
}

func TestGetSubscriptionPackagesParsesPricingPayload(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/subscription-packages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"month": 12, "price": 3360, "discount_percent": 30},
				{"month": 1, "price": 400, "discount_percent": 0},
				{"month": 6, "price": 1920, "discount_percent": 20},
				{"month": 3, "price": 1080, "discount_percent": 10}
			]
		}`))
	})

	packages, err := client.GetSubscriptionPackages()
	if err != nil {
		t.Fatalf("GetSubscriptionPackages returned error: %v", err)
	}

	if len(packages) != 4 {
		t.Fatalf("expected 4 packages, got %d", len(packages))
	}
	if packages[0].Month != 1 || packages[0].Price != 400 || packages[0].DiscountPercent != 0 {
		t.Fatalf("unexpected first package: %#v", packages[0])
	}
	if packages[3].Month != 12 || packages[3].Price != 3360 || packages[3].DiscountPercent != 30 {
		t.Fatalf("unexpected last package: %#v", packages[3])
	}
}

func TestGetVlessSubscriptionLinkParsesJSONLinkPayload(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/vless/link" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"link":"vless://subscription","happ_deep_link":"happ://deep","v2raytun_deeplink":"v2raytun://deep"}}`))
	})

	linkResponse, apiErr, err := client.GetVlessSubscriptionLink()
	if err != nil {
		t.Fatalf("GetVlessSubscriptionLink returned error: %v", err)
	}
	if apiErr != nil {
		t.Fatalf("expected nil API error, got: %#v", apiErr)
	}
	if linkResponse.Link != "vless://subscription" {
		t.Fatalf("unexpected link: %#v", linkResponse)
	}
	if linkResponse.HappDeepLink != "happ://deep" {
		t.Fatalf("unexpected Happ deep link: %#v", linkResponse)
	}
	if linkResponse.V2RayTunDeepLink != "v2raytun://deep" {
		t.Fatalf("unexpected V2RayTun deep link: %#v", linkResponse)
	}
}

func TestGetVlessSubscriptionLinkFallsBackToPlainStringResponse(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/vless/link" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(`vless://subscription`))
	})

	linkResponse, apiErr, err := client.GetVlessSubscriptionLink()
	if err != nil {
		t.Fatalf("GetVlessSubscriptionLink returned error: %v", err)
	}
	if apiErr != nil {
		t.Fatalf("expected nil API error, got: %#v", apiErr)
	}
	if linkResponse.Link != "vless://subscription" {
		t.Fatalf("unexpected link response: %#v", linkResponse)
	}
	if linkResponse.HappDeepLink != "" || linkResponse.V2RayTunDeepLink != "" {
		t.Fatalf("expected empty deep links for plain string response, got %#v", linkResponse)
	}
}

func TestGetConfigsFallsBackToLegacyRoute(t *testing.T) {
	var seen []string
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)

		switch r.URL.Path {
		case "/users/123/configs/wireguard":
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
		case "/users/123/wireguard/configs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"configs":[{"id":1,"user_id":123,"name":"legacy"}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	response, err := client.GetConfigs("wireguard")
	if err != nil {
		t.Fatalf("GetConfigs returned error: %v", err)
	}
	if len(response.Configs) != 1 || response.Configs[0].Name != "legacy" {
		t.Fatalf("unexpected configs response: %#v", response)
	}

	joined := strings.Join(seen, ",")
	if !strings.Contains(joined, "/users/123/configs/wireguard") || !strings.Contains(joined, "/users/123/wireguard/configs") {
		t.Fatalf("expected both new and legacy routes to be tried, saw %s", joined)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
