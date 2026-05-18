package commands

import (
	"oksana-vpn-telegram-bot/pkg/api"
)

var waitingForAmount = make(map[int64]bool)

func missingUserMessage() string {
	return api.MissingUserMessage()
}

func isMissingUserConfigResponse(response *api.ConfigResponse, err error) bool {
	if err == nil || response == nil {
		return false
	}

	return api.IsMissingUserError(404, response.Message)
}
