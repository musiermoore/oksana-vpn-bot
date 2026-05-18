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

func TestGetVlessSubscriptionLinkParsesJSONLinkPayload(t *testing.T) {
	client := withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123/vless/link" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"link":"vless://subscription"}}`))
	})

	link, apiErr, err := client.GetVlessSubscriptionLink()
	if err != nil {
		t.Fatalf("GetVlessSubscriptionLink returned error: %v", err)
	}
	if apiErr != nil {
		t.Fatalf("expected nil API error, got: %#v", apiErr)
	}
	if link != "vless://subscription" {
		t.Fatalf("unexpected link: %s", link)
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
