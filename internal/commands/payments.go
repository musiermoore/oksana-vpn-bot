package commands

import (
	"fmt"
	"math"
	"oksana-vpn-telegram-bot/pkg/api"
	"oksana-vpn-telegram-bot/pkg/utils"
	"strconv"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

func HandleSubscription(c telebot.Context) error {
	kb := &telebot.ReplyMarkup{}

	btnPayment := kb.Data("Купить подписку", "send_payment_request")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnPayment, btnToStart))

	client := api.NewClient(c)
	balance, err := client.GetBalance()
	if err != nil {
		if api.IsMissingUserError(404, err.Error()) {
			return c.Send(missingUserMessage())
		}
		return c.Send("Произошла ошибка. Попробуй чуть позже.")
	}

	status, err := client.GetRegistrationStatus()
	if err != nil {
		return c.Send("Произошла ошибка. Попробуй чуть позже.")
	}

	subscriptionString := `*Подписка*`

	if balance.Amount > 0 {
		subscriptionString += fmt.Sprintf(`
*Баланс:* %.2f
`, balance.Amount)
	}

	if balance.Debt > 0 {
		subscriptionString += fmt.Sprintf(`
*Долг:* %.2f
`, balance.Debt)
	}

	subscriptionString += `
%s
Выбери подписку на 1, 3, 6 или 12 месяцев.
Если баланса не хватит, я подготовлю ссылку на онлайн-оплату, а после успешного платежа подписка активируется автоматически.
`

	subscriptionString = fmt.Sprintf(subscriptionString, getSubscriptionDetails(status))
	subscriptionString = utils.EscapeMarkdownV2(subscriptionString)

	return c.Send(subscriptionString, &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdownV2,
		ReplyMarkup: kb,
	})
}

func HandleSendPaymentRequest(c telebot.Context) error {
	client := api.NewClient(c)
	packages, err := client.GetSubscriptionPackages()
	if err != nil {
		if api.IsMissingUserError(404, err.Error()) {
			return c.Send(missingUserMessage())
		}
		return c.Send("Не получилось загрузить доступные подписки. Попробуй чуть позже.")
	}

	if len(packages) == 0 {
		return c.Send("Сейчас нет доступных вариантов подписки. Попробуй чуть позже.")
	}

	return c.Send(getSubscriptionPackagePrompt(packages), getSubscriptionPackageKeyboard(packages))
}

func HandleChooseSubscriptionPackage(c telebot.Context) error {
	month, err := parseSubscriptionMonthCallback(callbackData(c.Callback().Data), "choose_subscription_package|")
	if err != nil {
		return c.Send("Не удалось найти выбранный вариант подписки. Выбери один из доступных вариантов.", getRetrySubscriptionPackageKeyboard())
	}

	client := api.NewClient(c)
	packages, err := client.GetSubscriptionPackages()
	if err != nil {
		if api.IsMissingUserError(404, err.Error()) {
			return c.Send(missingUserMessage())
		}
		return c.Send("Не получилось проверить данный тип подписки. Попробуй чуть позже.")
	}

	selectedPackage, ok := findSubscriptionPackage(packages, month)
	if !ok {
		return c.Send("Не удалось найти выбранный вариант подписки. Выбери один из доступных вариантов.", getRetrySubscriptionPackageKeyboard())
	}

	return submitSubscriptionPurchase(c, selectedPackage.Month)
}

func HandleSubmitPaymentRequest(c telebot.Context) error {
	data := callbackData(c.Callback().Data)

	kb := &telebot.ReplyMarkup{}
	btnToStart := kb.Data("К началу", "to_start")

	month, err := parseSubscriptionSubmitCallback(data)
	if err != nil {
		btnTryAgain := kb.Data("Повторить", "send_payment_request")
		kb.Inline(kb.Row(btnTryAgain, btnToStart))
		return c.Send("Не удалось найти выбранный вариант подписки. Попробуй ещё раз.", kb)
	}

	return submitSubscriptionPurchase(c, month)
}

func HandleDepositAction(c telebot.Context) error {
	data := callbackData(c.Callback().Data)
	parts := strings.SplitN(data, "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		_ = c.Respond(&telebot.CallbackResponse{})
		return c.Send("Unknown action.")
	}

	action := parts[0]
	transactionID := parts[1]
	client := api.NewClient(c)

	if err := c.Respond(&telebot.CallbackResponse{}); err != nil {
		return err
	}

	username := c.Sender().Username
	if username == "" {
		username = c.Sender().FirstName
	}

	var err error
	var responseText string

	switch action {
	case "approve_deposit":
		err = client.ApprovePaymentRequest(transactionID)
		if err == nil {
			responseText = fmt.Sprintf("Запрос @%s принят ✅", username)
		} else {
			responseText = "Ошибка =("
		}
	case "deny_deposit":
		err = client.DeclinePaymentRequest(transactionID)
		if err == nil {
			responseText = fmt.Sprintf("Запрос @%s отклонён ❌", username)
		} else {
			responseText = "Ошибка =("
		}
	default:
		responseText = "Unknown action."
	}

	return c.Send(responseText)
}

func getSubscriptionPackagePrompt(packages []api.SubscriptionPackage) string {
	lines := []string{"Выбери срок подписки:", ""}

	for _, subscriptionPackage := range packages {
		subscriptionText := "%s - %d ₽"

		if subscriptionPackage.DiscountPercent > 0 {
			subscriptionText += fmt.Sprintf(" (скидка %d%%)", subscriptionPackage.DiscountPercent)
		}

		lines = append(lines, fmt.Sprintf(
			subscriptionText,
			formatSubscriptionDuration(subscriptionPackage.Month),
			subscriptionPackage.Price,
		))
	}

	return strings.Join(lines, "\n")
}

func getSubscriptionPackageKeyboard(packages []api.SubscriptionPackage) *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}
	btnCancel := kb.Data("Отменить", "cancel_payment_and_return_to_start")

	var rows []telebot.Row
	for i := 0; i < len(packages); i += 2 {
		buttons := []telebot.Btn{
			kb.Data(
				fmt.Sprintf("%s - %d ₽", formatSubscriptionDuration(packages[i].Month), packages[i].Price),
				fmt.Sprintf("submit_payment_request|%d", packages[i].Month),
			),
		}

		if i+1 < len(packages) {
			buttons = append(buttons, kb.Data(
				fmt.Sprintf("%s - %d ₽", formatSubscriptionDuration(packages[i+1].Month), packages[i+1].Price),
				fmt.Sprintf("submit_payment_request|%d", packages[i+1].Month),
			))
		}

		rows = append(rows, kb.Row(buttons...))
	}

	rows = append(rows, kb.Row(btnCancel))
	kb.Inline(rows...)

	return kb
}

func parseSubscriptionMonthCallback(data string, prefix string) (int, error) {
	monthValue := strings.TrimSpace(strings.TrimPrefix(data, prefix))
	if monthValue == "" || monthValue == data {
		return 0, fmt.Errorf("missing month")
	}

	var month int
	if _, err := fmt.Sscanf(monthValue, "%d", &month); err != nil {
		return 0, err
	}

	if month <= 0 {
		return 0, fmt.Errorf("unsupported month")
	}

	return month, nil
}

func getRetrySubscriptionPackageKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}
	btnRetry := kb.Data("Выбрать срок подписки", "send_payment_request")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(
		kb.Row(btnRetry),
		kb.Row(btnToStart),
	)

	return kb
}

func parseSubscriptionSubmitCallback(data string) (int, error) {
	parts := strings.Split(strings.TrimSpace(data), "|")
	if len(parts) < 2 || parts[0] != "submit_payment_request" {
		return 0, fmt.Errorf("invalid callback")
	}

	month, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, err
	}
	if month <= 0 {
		return 0, fmt.Errorf("unsupported month")
	}

	return month, nil
}

func formatSubscriptionDuration(month int) string {
	switch month {
	case 1:
		return "1 месяц"
	case 3:
		return "3 месяца"
	case 6:
		return "6 месяцев"
	case 12:
		return "12 месяцев"
	default:
		return fmt.Sprintf("%d мес.", month)
	}
}

func findSubscriptionPackage(packages []api.SubscriptionPackage, month int) (api.SubscriptionPackage, bool) {
	for _, subscriptionPackage := range packages {
		if subscriptionPackage.Month == month {
			return subscriptionPackage, true
		}
	}

	return api.SubscriptionPackage{}, false
}

func buildSubscriptionPurchaseMessage(response api.PaymentResponse) string {
	message := strings.TrimSpace(response.Message)

	switch response.Status {
	case "activated":
		if message == "" && response.FormattedEndDate != "" {
			message = fmt.Sprintf("Подписка уже активирована до %s.", response.FormattedEndDate)
		}
		if message == "" {
			message = "Подписка успешно активирована."
		}
		return message + "\n\nНичего дополнительно оплачивать не нужно."
	case "deposit_required":
		if message == "" && response.DepositAmount > 0 {
			message = fmt.Sprintf("Для активации подписки необходимо оплатить %s ₽.", formatDepositAmount(response.DepositAmount))
		}
		if message == "" {
			message = "Для активации подписки требуется онлайн-оплата."
		}
		return message + "\n\nНажми кнопку ниже. После успешной оплаты подписка активируется автоматически."
	default:
		if message == "" {
			return "Извини, сейчас не получилось обработать запрос. Попробуй чуть позже."
		}
		return message
	}
}

func submitSubscriptionPurchase(c telebot.Context, month int) error {
	kb := &telebot.ReplyMarkup{}
	btnToStart := kb.Data("К началу", "to_start")

	client := api.NewClient(c)
	response, err := client.SendPaymentRequest(month, "tbank")
	if err != nil && api.IsMissingUserError(404, response.Message) {
		kb.Inline(kb.Row(btnToStart))
		return c.Send(missingUserMessage(), kb)
	}

	if err != nil {
		kb.Inline(kb.Row(btnToStart))
		return c.Send("Извини, сейчас не получилось создать оплату. Попробуй чуть позже.", kb)
	}

	if response.Status == "deposit_required" && response.ConfirmationURL != "" {
		btnPay := telebot.Btn{Text: "Оплатить онлайн", URL: response.ConfirmationURL}
		kb.Inline(
			kb.Row(btnPay),
			kb.Row(btnToStart),
		)
	} else {
		kb.Inline(kb.Row(btnToStart))
	}

	return c.Send(buildSubscriptionPurchaseMessage(response), kb)
}

func formatDepositAmount(amount float64) string {
	if math.Mod(amount, 1) == 0 {
		return fmt.Sprintf("%.0f", amount)
	}

	return fmt.Sprintf("%.2f", amount)
}
