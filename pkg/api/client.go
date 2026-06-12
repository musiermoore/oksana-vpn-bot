package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"sort"
	"strconv"
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

func (c *Client) userPath(path string) string {
	base := fmt.Sprintf("users/%s", c.userIDString())
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return base
	}

	return base + "/" + path
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

type SaveUserIDRequest struct {
	Telegram string `json:"telegram"`
	Name     string `json:"name"`
}

type RegistrationStatus struct {
	Registered                       bool    `json:"registered"`
	ActiveSubscriptionEndDate        *string `json:"active_subscription_end_date"`
	HasMoneyForNextSubscriptionMonth bool    `json:"has_money_for_next_subscription_month"`
}

func (r *RegistrationStatus) UnmarshalJSON(data []byte) error {
	var payload struct {
		Registered                       bool    `json:"registered"`
		ActiveSubscriptionEndDate        *string `json:"active_subscription_end_date"`
		HasMoneyForNextSubscriptionMonth bool    `json:"has_money_for_next_subscription_month"`
		Attributes                       struct {
			Registered                       bool    `json:"registered"`
			ActiveSubscriptionEndDate        *string `json:"active_subscription_end_date"`
			HasMoneyForNextSubscriptionMonth bool    `json:"has_money_for_next_subscription_month"`
		} `json:"attributes"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	r.Registered = payload.Registered || payload.Attributes.Registered
	if payload.ActiveSubscriptionEndDate != nil {
		r.ActiveSubscriptionEndDate = payload.ActiveSubscriptionEndDate
	} else {
		r.ActiveSubscriptionEndDate = payload.Attributes.ActiveSubscriptionEndDate
	}
	r.HasMoneyForNextSubscriptionMonth = payload.HasMoneyForNextSubscriptionMonth || payload.Attributes.HasMoneyForNextSubscriptionMonth

	return nil
}

func MissingUserMessage() string {
	return "Я не нашла тебя в базе. Нажми кнопку регистрации и попробуй ещё раз."
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

type SubscriptionPackage struct {
	Month           int `json:"month"`
	Price           int `json:"price"`
	DiscountPercent int `json:"discount_percent"`
}

func (c *Client) GetBalance() (Balance, error) {
	respBytes, err := Request("GET", c.userPath("balance"), nil)
	if err != nil {
		return Balance{}, err
	}

	var data struct {
		Balance    float64 `json:"balance"`
		Amount     float64 `json:"amount"`
		Debt       float64 `json:"debt"`
		Attributes struct {
			Balance float64 `json:"balance"`
			Amount  float64 `json:"amount"`
			Debt    float64 `json:"debt"`
		} `json:"attributes"`
	}
	if err := unmarshalAPIData(respBytes, &data); err != nil {
		return Balance{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	amount := data.Amount
	if amount == 0 {
		amount = data.Balance
	}
	if amount == 0 {
		amount = data.Attributes.Amount
	}
	if amount == 0 {
		amount = data.Attributes.Balance
	}

	debt := data.Debt
	if debt == 0 {
		debt = data.Attributes.Debt
	}

	return Balance{
		Amount: amount,
		Debt:   debt,
	}, nil
}

func (c *Client) RegisterUser() error {
	resp, err := request("POST", c.userPath("save-id"), SaveUserIDRequest{
		Telegram: c.username,
		Name:     c.name,
	}, "application/json")
	if err == nil {
		return nil
	}

	// Keep compatibility with older backends during rollout.
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		return err
	}

	_, err = Request("POST", "users/register", RegisterUserRequest{
		Telegram:   c.username,
		TelegramID: c.userIDString(),
		Name:       c.name,
	})

	return err
}

func (c *Client) GetRegistrationStatus() (RegistrationStatus, error) {
	respBytes, err := Request("GET", c.userPath("registration-status"), nil)
	if err != nil {
		return RegistrationStatus{}, err
	}

	var data RegistrationStatus
	if err := unmarshalAPIData(respBytes, &data); err != nil {
		return RegistrationStatus{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return data, nil
}

func (c *Client) GetSubscriptionPackages() ([]SubscriptionPackage, error) {
	respBytes, err := Request("GET", c.userPath("subscription-packages"), nil)
	if err != nil {
		return nil, err
	}

	var packages []SubscriptionPackage
	if err := unmarshalAPIData(respBytes, &packages); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Month < packages[j].Month
	})

	return packages, nil
}

type PaymentResponse struct {
	Status           string `json:"status"`
	Message          string `json:"message"`
	DepositAmount    float64
	TransactionID    int    `json:"transaction_id"`
	InvoiceID        int    `json:"invoice_id"`
	PaymentID        string `json:"payment_id"`
	PaymentStatus    string `json:"payment_status"`
	ConfirmationURL  string `json:"confirmation_url"`
	EndDate          string `json:"end_date"`
	FormattedEndDate string `json:"formatted_end_date"`
}

func (c *Client) SendPaymentRequest(month int, bank string) (PaymentResponse, error) {
	type PaymentBody struct {
		Month int    `json:"month"`
		Bank  string `json:"bank"`
	}

	respBytes, err := Request("POST", c.userPath("transactions"), PaymentBody{
		Month: month,
		Bank:  bank,
	})

	if len(respBytes) == 0 {
		return PaymentResponse{
			Message: "Что-то пошло не так :(",
		}, fmt.Errorf("empty response from API: %w", err)
	}

	response, parseErr := parsePaymentResponse(respBytes)
	if parseErr != nil {
		return PaymentResponse{
			Message: "Что-то пошло не так :(",
		}, fmt.Errorf("failed to parse JSON: %w; raw: %s", parseErr, string(respBytes))
	}

	if response.Message == "" {
		response.Message = "Что-то пошло не так :("
	}

	return response, err
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

func (c *Config) UnmarshalJSON(data []byte) error {
	var payload struct {
		ID         json.RawMessage `json:"id"`
		UserID     json.RawMessage `json:"user_id"`
		Name       string          `json:"name"`
		Attributes struct {
			UserID json.RawMessage `json:"user_id"`
			Name   string          `json:"name"`
		} `json:"attributes"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	id, err := parseOptionalInt32(payload.ID)
	if err != nil {
		return err
	}

	userID, err := parseOptionalInt32(payload.UserID)
	if err != nil {
		return err
	}
	if userID == 0 {
		userID, err = parseOptionalInt32(payload.Attributes.UserID)
		if err != nil {
			return err
		}
	}

	name := payload.Name
	if name == "" {
		name = payload.Attributes.Name
	}

	c.ID = id
	c.UserID = userID
	c.Name = name

	return nil
}

type ConfigResponse struct {
	Configs []Config `json:"configs"`
	Message string   `json:"message"`
	Type    string   `json:"type"`
}

type VlessSubscriptionLinkResponse struct {
	Link             string `json:"link"`
	HappDeepLink     string `json:"happ_deep_link"`
	V2RayTunDeepLink string `json:"v2raytun_deeplink"`
}

func parseConfigAPIError(resp apiResponse, reqErr error) (*ConfigResponse, *APIError, error) {
	data, dataErr := parseConfigResponse(resp.Body)
	if dataErr != nil {
		if reqErr != nil {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    string(resp.Body),
			}, reqErr
		}
		return nil, nil, fmt.Errorf("failed to parse JSON: %w", dataErr)
	}

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		Message:    data.Message,
		Type:       data.Type,
	}
	if apiErr.Message == "" {
		apiErr.Message, _ = parseAPIMessage(resp.Body)
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
	respBytes, err := Request("GET", c.userPath(fmt.Sprintf("configs/%s", configType)), nil)
	if err != nil && isRouteFallbackResponse(err) {
		respBytes, err = Request("GET", fmt.Sprintf("users/%s/%s/configs", c.userIDString(), configType), nil)
	}

	data, parseErr := parseConfigResponse(respBytes)
	if parseErr != nil {
		if err != nil {
			return ConfigResponse{}, err
		}
		return ConfigResponse{}, fmt.Errorf("failed to parse JSON: %w", parseErr)
	}

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
		c.userPath(fmt.Sprintf("configs/%s/%s/qr-code", configType, configID)),
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
		c.userPath(fmt.Sprintf("configs/%s/%s/download", configType, configID)),
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
		data, err := parseConfigResponse(resp.Body)
		if err != nil {
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
		c.userPath(fmt.Sprintf("configs/%s/%s/download", configType, config)),
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

	if isJSONContentType(resp.ContentType) {
		link, linkErr := parseLinkValue(resp.Body)
		if linkErr == nil {
			return link, nil, nil
		}

		data, parseErr := parseConfigResponse(resp.Body)
		if parseErr != nil {
			return "", nil, parseErr
		}
		return "", &data, fmt.Errorf("api returned error: %s", data.Message)
	}

	link := string(resp.Body)

	return link, nil, nil
}

func (c *Client) GetVlessSubscriptionLink() (VlessSubscriptionLinkResponse, *APIError, error) {
	resp, err := request(
		"GET",
		c.userPath("vless/link"),
		nil,
		"text/plain, application/json",
	)
	if err != nil {
		_, apiErr, parseErr := parseConfigAPIError(resp, err)
		if apiErr != nil {
			return VlessSubscriptionLinkResponse{}, apiErr, parseErr
		}
		return VlessSubscriptionLinkResponse{}, nil, err
	}

	if isJSONContentType(resp.ContentType) {
		linkResponse, parseErr := parseVlessSubscriptionLinkResponse(resp.Body)
		if parseErr == nil {
			return linkResponse, nil, nil
		}
		_, apiErr, parseErr := parseConfigAPIError(resp, err)
		return VlessSubscriptionLinkResponse{}, apiErr, parseErr
	}

	return VlessSubscriptionLinkResponse{Link: string(resp.Body)}, nil, nil
}

func (c *Client) GetVlessSubscriptionQRCode() ([]byte, *APIError, error) {
	resp, err := request(
		"GET",
		c.userPath("vless/qr-code"),
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

func isRouteFallbackResponse(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.Contains(message, "api error 404:") || strings.Contains(message, "api error 405:")
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		contentType = mediaType
	}

	return contentType == "application/json"
}

func parseOptionalInt32(raw json.RawMessage) (int32, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}

	var intValue int32
	if err := json.Unmarshal(raw, &intValue); err == nil {
		return intValue, nil
	}

	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		if strings.TrimSpace(stringValue) == "" {
			return 0, nil
		}

		value, convErr := strconv.Atoi(stringValue)
		if convErr != nil {
			return 0, convErr
		}

		return int32(value), nil
	}

	return 0, fmt.Errorf("unsupported numeric value: %s", string(raw))
}

func parseOptionalFloat64(raw json.RawMessage) (float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}

	var floatValue float64
	if err := json.Unmarshal(raw, &floatValue); err == nil {
		return floatValue, nil
	}

	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		if strings.TrimSpace(stringValue) == "" {
			return 0, nil
		}

		value, convErr := strconv.ParseFloat(stringValue, 64)
		if convErr != nil {
			return 0, convErr
		}

		return value, nil
	}

	return 0, fmt.Errorf("unsupported float value: %s", string(raw))
}

func unmarshalAPIData(body []byte, target any) error {
	if len(body) == 0 {
		return io.EOF
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		return json.Unmarshal(envelope.Data, target)
	}

	return json.Unmarshal(body, target)
}

func parseAPIMessage(body []byte) (string, error) {
	var envelope struct {
		Message string              `json:"message"`
		Errors  map[string][]string `json:"errors"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", err
	}

	if envelope.Message != "" {
		return envelope.Message, nil
	}

	if envelope.Data.Message != "" {
		return envelope.Data.Message, nil
	}

	for _, messages := range envelope.Errors {
		if len(messages) > 0 && messages[0] != "" {
			return messages[0], nil
		}
	}

	return "", nil
}

func parsePaymentResponse(body []byte) (PaymentResponse, error) {
	if len(body) == 0 {
		return PaymentResponse{}, nil
	}

	var response struct {
		Status           string          `json:"status"`
		Message          string          `json:"message"`
		DepositAmount    json.RawMessage `json:"deposit_amount"`
		TransactionID    int             `json:"transaction_id"`
		InvoiceID        int             `json:"invoice_id"`
		PaymentID        string          `json:"payment_id"`
		PaymentStatus    string          `json:"payment_status"`
		ConfirmationURL  string          `json:"confirmation_url"`
		EndDate          string          `json:"end_date"`
		FormattedEndDate string          `json:"formatted_end_date"`
	}
	if err := unmarshalAPIData(body, &response); err != nil {
		return PaymentResponse{}, err
	}

	depositAmount, err := parseOptionalFloat64(response.DepositAmount)
	if err != nil {
		return PaymentResponse{}, err
	}

	result := PaymentResponse{
		Status:           response.Status,
		Message:          response.Message,
		DepositAmount:    depositAmount,
		TransactionID:    response.TransactionID,
		InvoiceID:        response.InvoiceID,
		PaymentID:        response.PaymentID,
		PaymentStatus:    response.PaymentStatus,
		ConfirmationURL:  response.ConfirmationURL,
		EndDate:          response.EndDate,
		FormattedEndDate: response.FormattedEndDate,
	}

	if result.Message == "" {
		message, err := parseAPIMessage(body)
		if err == nil {
			result.Message = message
		}
	}

	return result, nil
}

func parseConfigResponse(body []byte) (ConfigResponse, error) {
	if len(body) == 0 {
		return ConfigResponse{}, nil
	}

	var response ConfigResponse
	if err := json.Unmarshal(body, &response); err == nil && (len(response.Configs) > 0 || response.Message != "" || response.Type != "") {
		return response, nil
	}

	var envelope struct {
		Data    json.RawMessage     `json:"data"`
		Message string              `json:"message"`
		Type    string              `json:"type"`
		Errors  map[string][]string `json:"errors"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return ConfigResponse{}, err
	}

	response.Message = envelope.Message
	response.Type = envelope.Type
	if response.Message == "" {
		for _, messages := range envelope.Errors {
			if len(messages) > 0 && messages[0] != "" {
				response.Message = messages[0]
				break
			}
		}
	}

	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return response, nil
	}

	if err := json.Unmarshal(envelope.Data, &response.Configs); err == nil {
		return response, nil
	}

	var nested struct {
		Configs []Config `json:"configs"`
	}
	if err := json.Unmarshal(envelope.Data, &nested); err == nil {
		response.Configs = nested.Configs
		return response, nil
	}

	return response, fmt.Errorf("unsupported configs payload: %s", string(envelope.Data))
}

func parseLinkValue(body []byte) (string, error) {
	var envelope struct {
		Data struct {
			Link string `json:"link"`
			URL  string `json:"url"`
		} `json:"data"`
		Link string `json:"link"`
		URL  string `json:"url"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", err
	}

	switch {
	case envelope.Data.Link != "":
		return envelope.Data.Link, nil
	case envelope.Data.URL != "":
		return envelope.Data.URL, nil
	case envelope.Link != "":
		return envelope.Link, nil
	case envelope.URL != "":
		return envelope.URL, nil
	default:
		return "", fmt.Errorf("link value not found")
	}
}

func parseVlessSubscriptionLinkResponse(body []byte) (VlessSubscriptionLinkResponse, error) {
	link, err := parseLinkValue(body)
	if err != nil {
		return VlessSubscriptionLinkResponse{}, err
	}

	var response VlessSubscriptionLinkResponse
	if err := unmarshalAPIData(body, &response); err != nil {
		return VlessSubscriptionLinkResponse{}, err
	}

	if response.Link == "" {
		response.Link = link
	}

	return response, nil
}
