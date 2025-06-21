package notify

import "testing"

type testNotifier struct {
	called []string
}

func (t *testNotifier) Notify(subject, message string) error {
	t.called = append(t.called, subject+":"+message)
	return nil
}

func TestRegisterAndCreateNotifier(t *testing.T) {
	Register("test", func(props map[string]string) Notifier {
		return &testNotifier{}
	})
	n := CreateNotifier("test", nil)
	if n == nil {
		t.Fatal("notifier not created")
	}
	tn, ok := n.(*testNotifier)
	if !ok {
		t.Fatal("wrong type")
	}
	err := n.Notify("subj", "msg")
	if err != nil {
		t.Errorf("Notify returned error: %v", err)
	}
	if len(tn.called) != 1 {
		t.Error("Notify not called")
	}
}

func TestSMTPNotifier(t *testing.T) {
	props := map[string]string{
		"smtp_user": "user@example.com",
		"smtp_pass": "password",
		"to":        "recipient@example.com",
	}
	n := NewSMTPNotifier(props)
	if n == nil {
		t.Fatal("SMTPNotifier not created")
	}
	err := n.Notify("Test Subject", "Test Message")
	// We expect an error because credentials/server are fake, but it should be a proper error
	if err == nil {
		t.Error("Expected error for invalid SMTP config, got nil")
	}
}

func TestTelegramNotifier(t *testing.T) {
	props := map[string]string{
		"bot_token": "123456:fake-token",
		"chat_id":   "123456789",
	}
	n := NewTelegramNotifier(props)
	if n == nil {
		t.Log("TelegramNotifier could not be created with fake credentials, skipping Notify test.")
		return
	}
	err := n.Notify("Test Subject", "Test Message")
	// We expect an error because credentials are fake, but it should be a proper error
	if err == nil {
		t.Error("Expected error for invalid Telegram config, got nil")
	}
}
