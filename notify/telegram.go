package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type TelegramNotifier struct {
	BotToken string
	ChatID   string
}

func (t *TelegramNotifier) Notify(subject, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	body := map[string]interface{}{
		"chat_id": t.ChatID,
		"text":    fmt.Sprintf("%s\n%s", subject, message),
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram send failed: %s", resp.Status)
	}
	return nil
}

func NewTelegramNotifier(props map[string]string) Notifier {
	return &TelegramNotifier{
		BotToken: props["bot_token"],
		ChatID:   props["chat_id"],
	}
}

func init() {
	Register("telegram", NewTelegramNotifier)
}
