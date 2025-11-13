package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"gopkg.in/telebot.v4"
)

type Client struct {
	userId   int64
	username string
}

func NewClient(context telebot.Context) *Client {
	return &Client{
		userId:   context.Sender().ID,
		username: context.Sender().Username,
	}
}

func Request(method string, path string, data any) ([]byte, error) {
	baseUrl := os.Getenv("API_URL")
	username := os.Getenv("API_BASIC_AUTH_USER")
	password := os.Getenv("API_BASIC_AUTH_PASSWORD")

	var body io.Reader
	if data != nil {
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode body: %w", err)
		}
		body = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequest(method, baseUrl+path, body)

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

type Balance struct {
	Amount float64
	Debt   float64
}

func (c *Client) GetBalance() (Balance, error) {
	respBytes, err := Request("GET", fmt.Sprintf("users/%s/balance", c.username), nil)
	if err != nil {
		return Balance{}, err
	}

	// Parse JSON response
	var data struct {
		Balance float64 `json:"balance"`
		Debt    float64 `json:"debt"`
	}
	if err := json.Unmarshal(respBytes, &data); err != nil {
		return Balance{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return Balance{
		Amount: data.Balance,
		Debt:   data.Debt,
	}, nil
}

type PaymentResponse struct {
	Status  bool
	Message string
}

func (c *Client) SendPaymentRequest(amount float32) (PaymentResponse, error) {
	type PaymentBody struct {
		Amount float32 `json:"amount"`
		Bank   string  `json:"bank"`
	}

	respBytes, err := Request("POST", fmt.Sprintf("users/%s/transactions", c.username), PaymentBody{
		Amount: amount,
		Bank:   "tbank",
	})

	var data struct {
		Message string `json:"message"`
	}

	if len(respBytes) == 0 {
		return PaymentResponse{
			Message: "Что-то пошло не так :(",
		}, fmt.Errorf("empty response from API: %w", err)
	}

	if jsonErr := json.Unmarshal(respBytes, &data); jsonErr != nil {
		return PaymentResponse{
			Message: "Что-то пошло не так :(",
		}, fmt.Errorf("failed to parse JSON: %w; raw: %s", jsonErr, string(respBytes))
	}

	if data.Message == "" {
		data.Message = "Что-то пошло не так :("
	}

	return PaymentResponse{
		Status:  err == nil,
		Message: data.Message,
	}, nil
}
