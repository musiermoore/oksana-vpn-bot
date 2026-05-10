package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"gopkg.in/telebot.v4"
)

type Client struct {
	userID   int64
	username string
	name     string
}

func (c *Client) userIDString() string {
	return fmt.Sprintf("%d", c.userID)
}

type APIError struct {
	StatusCode int
	Message    string
	Type       string
}

type apiResponse struct {
	Body        []byte
	ContentType string
	StatusCode  int
}

func NewClient(context telebot.Context) *Client {
	name := context.Sender().FirstName
	if name == "" {
		name = context.Sender().Username
	}
	if name == "" && context.Chat() != nil {
		name = context.Chat().Title
	}

	return &Client{
		userID:   context.Sender().ID,
		username: context.Sender().Username,
		name:     name,
	}
}

func Request(method string, path string, data any) ([]byte, error) {
	resp, err := request(method, path, data, "application/json")
	if err != nil {
		return resp.Body, err
	}

	return resp.Body, nil
}

type RegisterUserRequest struct {
	Telegram   string `json:"telegram"`
	TelegramID string `json:"telegram_id"`
	Name       string `json:"name"`
}

func MissingUserMessage() string {
	return "Я не нашла тебя в базе. Используй /register и попробуй ещё раз."
}

func IsMissingUserError(statusCode int, message string) bool {
	if statusCode != http.StatusNotFound {
		return false
	}

	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return true
	}

	directMarkers := []string{
		"user not found",
		"not found in db",
		"пользователь не найден",
	}

	for _, marker := range directMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}

	notFoundMarkers := []string{"not found", "не найден"}
	userMarkers := []string{"user", "telegram", "username", "пользоват", "телеграм"}

	hasNotFoundMarker := false
	for _, marker := range notFoundMarkers {
		if strings.Contains(normalized, marker) {
			hasNotFoundMarker = true
			break
		}
	}

	if !hasNotFoundMarker {
		return false
	}

	for _, marker := range userMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}

	return false
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
	respBytes, err := Request("GET", fmt.Sprintf("users/%s/balance", c.userIDString()), nil)
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

func (c *Client) RegisterUser() error {
	_, err := Request("POST", "users/register", RegisterUserRequest{
		Telegram:   c.username,
		TelegramID: c.userIDString(),
		Name:       c.name,
	})

	return err
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

	respBytes, err := Request("POST", fmt.Sprintf("users/%s/transactions", c.userIDString()), PaymentBody{
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

func parseConfigAPIError(resp apiResponse, reqErr error) (*ConfigResponse, *APIError, error) {
	var data ConfigResponse
	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &data); err != nil {
			if reqErr != nil {
				return nil, &APIError{
					StatusCode: resp.StatusCode,
					Message:    string(resp.Body),
				}, reqErr
			}
			return nil, nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		Message:    data.Message,
		Type:       data.Type,
	}
	if apiErr.Message == "" {
		apiErr.Message = string(resp.Body)
	}

	if reqErr == nil {
		reqErr = fmt.Errorf("api error %d: %s", resp.StatusCode, apiErr.Message)
	}

	return &data, apiErr, reqErr
}

func (c *Client) GetWireGuardConfigs() (ConfigResponse, error) {
	return c.GetConfigs("wireguard")
}

func (c *Client) GetVlessConfigs() (ConfigResponse, error) {
	return c.GetConfigs("vless")
}

func (c *Client) GetConfigs(configType string) (ConfigResponse, error) {
	respBytes, err := Request("GET", fmt.Sprintf("users/%s/%s/configs", c.userIDString(), configType), nil)

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

func (c *Client) GetConfigQrCode(configType, configID string) ([]byte, *ConfigResponse, error) {
	resp, err := request(
		"GET",
		fmt.Sprintf("users/%s/configs/%s/%s/qr-code", c.userIDString(), configType, configID),
		nil,
		"image/png, application/json",
	)

	contentType, _, parseErr := mime.ParseMediaType(resp.ContentType)
	if parseErr != nil {
		contentType = resp.ContentType
	}

	if resp.StatusCode >= 400 || contentType == "application/json" {
		data, _, parseErr := parseConfigAPIError(resp, err)
		return nil, data, parseErr
	}

	if err != nil {
		return nil, nil, err
	}

	return resp.Body, nil, nil
}

func (c *Client) GetConfigFile(configType, configID string) ([]byte, *ConfigResponse, error) {
	resp, err := request(
		"GET",
		fmt.Sprintf("users/%s/configs/%s/%s/download", c.userIDString(), configType, configID),
		nil,
		"text/plain, application/json",
	)
	if err != nil {
		data, _, parseErr := parseConfigAPIError(resp, err)
		if data != nil {
			return nil, data, parseErr
		}
		return nil, nil, err
	}

	// Try to detect if the response is JSON (error) or file (success)
	if len(resp.Body) > 0 && resp.Body[0] == '{' {
		// Looks like JSON
		var data ConfigResponse
		if err := json.Unmarshal(resp.Body, &data); err != nil {
			return nil, nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return nil, &data, fmt.Errorf("api returned error: %s", data.Message)
	}

	// Otherwise, assume it's the file bytes
	return resp.Body, nil, nil
}

func (c *Client) GetLink(configType, config string) (string, *ConfigResponse, error) {
	resp, err := request(
		"GET",
		fmt.Sprintf("users/%s/configs/%s/%s/download", c.userIDString(), configType, config),
		nil,
		"text/plain, application/json",
	)
	if err != nil {
		data, _, parseErr := parseConfigAPIError(resp, err)
		if data != nil {
			return "", data, parseErr
		}
		return "", nil, err
	}

	link := string(resp.Body)

	return link, nil, nil
}

func (c *Client) GetVlessSubscriptionLink() (string, *APIError, error) {
	resp, err := request(
		"GET",
		fmt.Sprintf("users/%s/vless/link", c.userIDString()),
		nil,
		"text/plain, application/json",
	)
	if err != nil {
		_, apiErr, parseErr := parseConfigAPIError(resp, err)
		if apiErr != nil {
			return "", apiErr, parseErr
		}
		return "", nil, err
	}

	return string(resp.Body), nil, nil
}

func (c *Client) GetVlessSubscriptionQRCode() ([]byte, *APIError, error) {
	resp, err := request(
		"GET",
		fmt.Sprintf("users/%s/vless/qr-code", c.userIDString()),
		nil,
		"image/png, application/json",
	)

	contentType, _, parseErr := mime.ParseMediaType(resp.ContentType)
	if parseErr != nil {
		contentType = resp.ContentType
	}

	if resp.StatusCode >= 400 || contentType == "application/json" {
		_, apiErr, parseErr := parseConfigAPIError(resp, err)
		return nil, apiErr, parseErr
	}

	if err != nil {
		return nil, nil, err
	}

	return resp.Body, nil, nil
}
