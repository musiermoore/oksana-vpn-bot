package commands

import (
	"bytes"
	"fmt"
	"oksana-vpn-telegram-bot/pkg/api"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

func getConfigsKeyboard(c telebot.Context, configType string) (*telebot.ReplyMarkup, *api.ConfigResponse, error) {
	client := api.NewClient(c)
	response, err := client.GetConfigs(configType)

	kb := &telebot.ReplyMarkup{}
	var inline []telebot.Row

	if err == nil {
		for _, config := range response.Configs {
			row := kb.Data(config.Name, "config|"+configType+"|"+strconv.Itoa(int(config.ID)))
			inline = append(inline, kb.Row(row))
		}
	} else if response.Type == "debt" {
		btnSubscription := kb.Data("Подписка", "menu_subscription")
		inline = append(inline, kb.Row(btnSubscription))
	}

	btnToStart := kb.Data("К началу", "to_start")
	inline = append(inline, kb.Row(btnToStart))
	kb.Inline(inline...)

	return kb, &response, err
}

func getConfigMessage(response *api.ConfigResponse, err error, message string) string {
	if isMissingUserConfigResponse(response, err) {
		return missingUserMessage()
	}
	if err != nil && response.Message != "" {
		return response.Message
	}
	if len(response.Configs) == 0 {
		return "Конфиги не найдены"
	}

	return message
}

func sendConfigList(c telebot.Context, configType string) error {
	kb, response, err := getConfigsKeyboard(c, configType)
	return c.Send(getConfigMessage(response, err, "Выбери конфиг:"), kb)
}

func HandleWireguardConfigsCommand(c telebot.Context) error {
	return sendConfigList(c, "wireguard")
}

func HandleWireguardConfigsButton(c telebot.Context) error {
	return sendConfigList(c, "wireguard")
}

func HandleVlessConfigsCommand(c telebot.Context) error {
	return sendConfigList(c, "vless")
}

func HandleVlessConfigsButton(c telebot.Context) error {
	return sendConfigList(c, "vless")
}

func HandleChoosingConfig(c telebot.Context) error {
	config := callbackData(c.Callback().Data)
	parts := strings.Split(config, "|")

	configType := parts[1]
	configID := parts[2]
	configName := lookupConfigName(c, configType, configID)

	kb := &telebot.ReplyMarkup{}
	btnQR := kb.Data("QR Code", "action_config_qr|"+configType+"|"+configID)

	fmt.Println("HandleChoosing: " + config)
	var actionButtons []telebot.Btn
	if configType == "vless" {
		btnLink := kb.Data("Получить ссылку", "action_config_link|"+configType+"|"+configID)
		actionButtons = []telebot.Btn{btnQR, btnLink}
	} else {
		btnDownload := kb.Data("Файл", "action_config_file|"+configType+"|"+configID)
		actionButtons = []telebot.Btn{btnQR, btnDownload}
	}

	btnWireguardConfigs := kb.Data("WireGuard Конфиги", "to_wireguard_configs")
	btnVless := kb.Data("VLESS", "to_vless")

	kb.Inline(
		kb.Row(actionButtons...),
		kb.Row(btnWireguardConfigs),
		kb.Row(btnVless),
	)

	return c.Send("Выбери действие для конфига "+configName, kb)
}

func lookupConfigName(c telebot.Context, configType, configID string) string {
	client := api.NewClient(c)
	response, err := client.GetConfigs(configType)
	if err != nil {
		return "ID " + configID
	}

	for _, config := range response.Configs {
		if strconv.Itoa(int(config.ID)) == configID {
			return config.Name
		}
	}

	return "ID " + configID
}

func prepareConfigData(c telebot.Context) []string {
	return strings.Split(callbackData(c.Callback().Data), "|")
}

func getConfigType(c telebot.Context) string {
	parts := prepareConfigData(c)
	if len(parts) < 2 {
		return ""
	}

	return parts[1]
}

func getConfigID(c telebot.Context) string {
	parts := prepareConfigData(c)
	if len(parts) < 3 {
		return ""
	}

	return parts[2]
}

func getActionConfigKeyboard(configType string) *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	backAction := "to_wireguard_configs"
	if configType == "vless" {
		backAction = "to_vless"
	}

	btnPrev := kb.Data("Конфиги", backAction)
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnPrev, btnToStart))

	return kb
}

func HandleActionConfig(c telebot.Context) error {
	configType := getConfigType(c)
	configName := lookupConfigName(c, configType, getConfigID(c))
	kb := getActionConfigKeyboard(configType)

	fmt.Println("HandleActionConfig: " + callbackData(c.Callback().Data))
	switch {
	case strings.HasPrefix(callbackData(c.Callback().Data), "action_config_qr|"):
		return HandleQrCodeConfig(c)
	case strings.HasPrefix(callbackData(c.Callback().Data), "action_config_file|"):
		return HandleDownloadConfig(c)
	case strings.HasPrefix(callbackData(c.Callback().Data), "action_config_link|"):
		return HandleGetLinkConfig(c)
	case strings.HasPrefix(callbackData(c.Callback().Data), "action_config_both|"):
		return c.Send("QR Code и файл для "+configName, kb)
	default:
		return c.Send("Непредвиденная ошибка.", kb)
	}
}

func sanitizeConfigFileName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return re.ReplaceAllString(name, "")
}

func resolveDownloadFileName(serverFileName, configName string) string {
	serverFileName = strings.TrimSpace(serverFileName)
	if serverFileName != "" {
		baseName := filepath.Base(serverFileName)
		if baseName != "." && baseName != string(filepath.Separator) && baseName != "" {
			return baseName
		}
	}

	return sanitizeConfigFileName(configName) + ".conf"
}

func HandleDownloadConfig(c telebot.Context) error {
	client := api.NewClient(c)
	configType := getConfigType(c)
	configID := getConfigID(c)
	configName := lookupConfigName(c, configType, configID)
	kb := getActionConfigKeyboard(configType)

	fileData, fileName, apiError, err := client.GetConfigFile(configType, configID)
	if err != nil {
		if apiError != nil {
			fmt.Println("API error:", apiError.Message)
			if isMissingUserConfigResponse(apiError, err) {
				return c.Send(missingUserMessage(), kb)
			}
		} else {
			fmt.Println("Request error:", err)
		}
		return c.Send("Произошла ошибка при запросе файла.")
	}

	doc := &telebot.Document{
		File: telebot.File{
			FileReader: bytes.NewReader(fileData),
		},
		FileName: resolveDownloadFileName(fileName, configName),
		Caption:  "Вот твой конфиг 😽",
	}

	return c.Send(doc, kb)
}

func HandleQrCodeConfig(c telebot.Context) error {
	client := api.NewClient(c)
	configType := getConfigType(c)
	configID := getConfigID(c)
	kb := getActionConfigKeyboard(configType)

	fileData, apiError, err := client.GetConfigQrCode(configType, configID)
	if err != nil {
		if apiError != nil {
			fmt.Println("API error:", apiError.Message)
			if isMissingUserConfigResponse(apiError, err) {
				return c.Send(missingUserMessage(), kb)
			}
		} else {
			fmt.Println("Request error:", err)
		}
		return c.Send("Произошла ошибка при запросе файла.")
	}

	photo := &telebot.Photo{
		File: telebot.File{
			FileReader: bytes.NewReader(fileData),
		},
		Caption: "Вот твой QR Code 😽",
	}

	return c.Send(photo, kb)
}

func HandleGetLinkConfig(c telebot.Context) error {
	client := api.NewClient(c)
	configType := getConfigType(c)
	configID := getConfigID(c)
	kb := getActionConfigKeyboard(configType)

	link, apiError, err := client.GetLink(configType, configID)
	if err != nil {
		if apiError != nil {
			fmt.Println("API error:", apiError.Message)
			if isMissingUserConfigResponse(apiError, err) {
				return c.Send(missingUserMessage(), kb)
			}
			return c.Send(apiError.Message, kb)
		}

		fmt.Println("Request error:", err)
		return c.Send("Произошла ошибка при запросе ссылки.")
	}

	return c.Send(fmt.Sprintf("Вот твоя ссылка 😽\n\n`%s`", link), &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdown,
		ReplyMarkup: kb,
	})
}
