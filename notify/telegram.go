package notify

import (
	"strconv"

	"github.com/nikoksr/notify"
	notifyTelegram "github.com/nikoksr/notify/service/telegram"
)

type TelegramNotifier struct {
	n *notify.Notify
}

func (t *TelegramNotifier) Notify(subject, message string) error {
	return t.n.Send(nil, subject, message)
}

func NewTelegramNotifier(props map[string]string) Notifier {
	tg, err := notifyTelegram.New(props["bot_token"])
	if tg == nil || err != nil {
		return nil
	}
	if chatIDStr, ok := props["chat_id"]; ok {
		if chatID, err := strconv.ParseInt(chatIDStr, 10, 64); err == nil {
			tg.AddReceivers(chatID)
		}
	}
	n := notify.New()
	n.UseServices(tg)
	return &TelegramNotifier{n: n}
}

func init() {
	Register("telegram", NewTelegramNotifier)
}
