package commands

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"oksana-vpn-telegram-bot/pkg/api"

	telebot "gopkg.in/telebot.v4"
)

func getVlessKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnLink := kb.Data("Link", "vless|link")
	btnQR := kb.Data("QR-Code", "vless|qr")
	btnToStart := kb.Data("К началу", "to_start")

	kb.Inline(
		kb.Row(btnLink, btnQR),
		kb.Row(btnToStart),
	)

	return kb
}

func getVlessResultKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnBack := kb.Data("Назад", "vless|menu")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnBack, btnToStart))

	return kb
}

func getVlessErrorMessage(apiErr *api.APIError, fallback string) string {
	if apiErr == nil {
		return fallback
	}

	if api.IsMissingUserError(apiErr.StatusCode, apiErr.Message) {
		return missingUserMessage()
	}

	switch apiErr.StatusCode {
	case 403, 404:
		if apiErr.Message != "" {
			return apiErr.Message
		}
	}

	return fallback
}

func ensureVlessAccess(c telebot.Context) (bool, error) {
	client := api.NewClient(c)
	response, err := client.GetVlessConfigs()
	if err == nil {
		return true, nil
	}

	kb := getVlessResultKeyboard()
	if isMissingUserConfigResponse(&response, err) {
		return false, c.Send(missingUserMessage(), kb)
	}
	if response.Message != "" {
		return false, c.Send(response.Message, kb)
	}

	return false, c.Send("Не получилось получить доступ к VLESS. Попробуй чуть позже.", kb)
}

func HandleVlessButton(c telebot.Context) error {
	return HandleVlessCommand(c)
}

func HandleVlessCommand(c telebot.Context) error {
	allowed, err := ensureVlessAccess(c)
	if err != nil || !allowed {
		return err
	}

	return c.Send("Выбери действие для VLESS:", getVlessKeyboard())
}

func buildVlessLinkMessage(link string) string {
	escapedLink := url.QueryEscape(link)
	happLink := fmt.Sprintf("happ://add-subscription?url=%s", escapedLink)
	v2rayTunLink := fmt.Sprintf("v2raytun://install-sub?url=%s", escapedLink)

	return fmt.Sprintf(
		"Вот твоя ссылка 😽\n\n<a href=\"%s\">Добавить подписку в Happ</a>\n\n<a href=\"%s\">Добавить подписку в V2RayTun</a>\n\nИли просто вставьте ссылку в приложении:\n<code>%s</code>",
		html.EscapeString(happLink),
		html.EscapeString(v2rayTunLink),
		html.EscapeString(link),
	)
}

func HandleVlessLinkAction(c telebot.Context) error {
	client := api.NewClient(c)
	kb := getVlessResultKeyboard()
	link, apiErr, err := client.GetVlessSubscriptionLink()
	if err != nil {
		if apiErr != nil {
			fmt.Println("API error:", apiErr.Message)
			return c.Send(getVlessErrorMessage(apiErr, "Не получилось получить VLESS ссылку. Попробуй чуть позже."), kb)
		}

		fmt.Println("Request error:", err)
		return c.Send("Не получилось получить VLESS ссылку. Попробуй чуть позже.", kb)
	}

	return c.Send(buildVlessLinkMessage(link), &telebot.SendOptions{
		ParseMode:   telebot.ModeHTML,
		ReplyMarkup: kb,
	})
}

func HandleVlessQrAction(c telebot.Context) error {
	client := api.NewClient(c)
	kb := getVlessResultKeyboard()
	fileData, apiErr, err := client.GetVlessSubscriptionQRCode()
	if err != nil {
		if apiErr != nil {
			fmt.Println("API error:", apiErr.Message)
			return c.Send(getVlessErrorMessage(apiErr, "Не получилось получить VLESS QR-Code. Попробуй чуть позже."), kb)
		}

		fmt.Println("Request error:", err)
		return c.Send("Не получилось получить VLESS QR-Code. Попробуй чуть позже.", kb)
	}

	photo := &telebot.Photo{
		File: telebot.File{
			FileReader: bytes.NewReader(fileData),
		},
		Caption: "Вот твой QR Code 😽",
	}

	return c.Send(photo, kb)
}
