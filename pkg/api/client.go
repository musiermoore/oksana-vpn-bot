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

type apiResponse struct {
	Body        []byte
	ContentType string
	StatusCode  int
}

func NewClient(context telebot.Context) *Client {
	return &Client{
		userId:   context.Sender().ID,
		username: context.Sender().Username,
	}
}

func Request(method string, path string, data any) ([]byte, error) {
	resp, err := request(method, path, data, "application/json")
	if err != nil {
		return resp.Body, err
	}

	return resp.Body, nil
}

func request(method string, path string, data any, accept string) (apiResponse, error) {
	baseUrl := os.Getenv("API_URL")
	username := os.Getenv("API_BASIC_AUTH_USER")
	password := os.Getenv("API_BASIC_AUTH_PASSWORD")

	var body io.Reader
	if data != nil {
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return apiResponse{}, fmt.Errorf("failed to encode body: %w", err)
		}
		body = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequest(method, baseUrl+path, body)

	if err != nil {
		return apiResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return apiResponse{}, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse{}, fmt.Errorf("failed to read response body: %w", err)
	}

	result := apiResponse{
		Body:        respBody,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
	}

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
	}

	return result, nil
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

func (c *Client) ApprovePaymentRequest(transactionID string) error {
	_, err := Request("POST", fmt.Sprintf("transactions/%s/approve", transactionID), nil)
	return err
}

func (c *Client) DeclinePaymentRequest(transactionID string) error {
	_, err := Request("DELETE", fmt.Sprintf("transactions/%s/decline", transactionID), nil)
	return err
}

type Config struct {
	ID     int32  `json:"id"`
	UserID int32  `json:"user_id"`
	Name   string `json:"name"`
}

type ConfigResponse struct {
	Configs []Config `json:"configs"`
	Message string   `json:"message"`
	Type    string   `json:"type"`
}

func (c *Client) GetWireGuardConfigs() (ConfigResponse, error) {
	return c.GetConfigs("wireguard")
}

func (c *Client) GetVlessConfigs() (ConfigResponse, error) {
	return c.GetConfigs("vless")
}

func (c *Client) GetConfigs(configType string) (ConfigResponse, error) {
	respBytes, err := Request("GET", fmt.Sprintf("users/%s/%s/configs", c.username, configType), nil)

	// Even if there's an error, try to parse the response body as it might contain error details
	var data ConfigResponse
	if len(respBytes) > 0 {
		if jsonErr := json.Unmarshal(respBytes, &data); jsonErr != nil {
			// If we can't parse JSON and there was a request error, return both
			if err != nil {
				return ConfigResponse{}, err
			}
			return ConfigResponse{}, fmt.Errorf("failed to parse JSON: %w", jsonErr)
		}
	}

	// If the response indicates an error, return it with the data
	if data.Type != "" || data.Message != "" || err != nil {
		if data.Message != "" {
			return data, fmt.Errorf("api returned error: %s", data.Message)
		}
		return data, err
	}

	return data, nil
}

func (c *Client) GetConfigQrCode(configType, config string) ([]byte, *ConfigResponse, error) {
	resp, err := request(
		"GET",
		fmt.Sprintf("users/%s/configs/%s/%s/qr-code", c.username, configType, config),
		nil,
		"image/png, application/json",
	)

	if resp.StatusCode >= 400 || resp.ContentType == "application/json" {
		var data ConfigResponse
		if jsonErr := json.Unmarshal(resp.Body, &data); jsonErr != nil {
			if resp.StatusCode >= 400 {
				return nil, nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(resp.Body))
			}
			return nil, nil, fmt.Errorf("failed to parse JSON: %w", jsonErr)
		}

		if data.Message == "" {
			data.Message = fmt.Sprintf("api error %d", resp.StatusCode)
		}

		return nil, &data, fmt.Errorf("api returned error: %s", data.Message)
	}

	if err != nil {
		return nil, nil, err
	}

	return resp.Body, nil, nil
}

func (c *Client) GetConfigFile(configType, config string) ([]byte, *ConfigResponse, error) {
	// Make the request
	respBytes, err := Request("GET", fmt.Sprintf("users/%s/configs/%s/%s/download", c.username, configType, config), nil)
	if err != nil {
		// The API might return a JSON error body even on non-200 codes
		var errData ConfigResponse
		if jsonErr := json.Unmarshal([]byte(err.Error()), &errData); jsonErr == nil {
			return nil, &errData, fmt.Errorf("api error: %w", err)
		}
		return nil, nil, err
	}

	// Try to detect if the response is JSON (error) or file (success)
	if len(respBytes) > 0 && respBytes[0] == '{' {
		// Looks like JSON
		var data ConfigResponse
		if err := json.Unmarshal(respBytes, &data); err != nil {
			return nil, nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return nil, &data, fmt.Errorf("api returned error: %s", data.Message)
	}

	// Otherwise, assume it's the file bytes
	return respBytes, nil, nil
}

func (c *Client) GetLink(configType, config string) (string, *ConfigResponse, error) {
	// Make the request
	respBytes, err := Request("GET", fmt.Sprintf("users/%s/configs/%s/%s/download", c.username, configType, config), nil)
	if err != nil {
		// The API might return a JSON error body even on non-200 codes
		var errData ConfigResponse
		if jsonErr := json.Unmarshal([]byte(err.Error()), &errData); jsonErr == nil {
			return "", &errData, fmt.Errorf("api error: %w", err)
		}
		return "", nil, err
	}

	link := string(respBytes)

	// Otherwise, assume it's the file bytes
	return link, nil, nil
}

func (c *Client) GetVlessLink() (string, *ConfigResponse, error) {
	// Make the request
	respBytes, err := Request("GET", fmt.Sprintf("users/%s/vless-link", c.username), nil)
	if err != nil {
		// The API might return a JSON error body even on non-200 codes
		var errData ConfigResponse
		if jsonErr := json.Unmarshal([]byte(err.Error()), &errData); jsonErr == nil {
			return "", &errData, fmt.Errorf("api error: %w", err)
		}
		return "", nil, err
	}

	link := string(respBytes)

	// Otherwise, assume it's the file bytes
	return link, nil, nil
}
