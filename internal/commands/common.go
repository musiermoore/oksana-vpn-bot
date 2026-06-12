package commands

import (
	"oksana-vpn-telegram-bot/pkg/api"
	"strings"
)

func missingUserMessage() string {
	return api.MissingUserMessage()
}

func isMissingUserConfigResponse(response *api.ConfigResponse, err error) bool {
	if err == nil || response == nil {
		return false
	}

	return api.IsMissingUserError(404, response.Message)
}

func callbackData(raw string) string {
	return strings.TrimSpace(strings.TrimPrefix(raw, "\f"))
}
