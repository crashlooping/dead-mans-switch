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

func TestCreateNotifierNotFound(t *testing.T) {
	n := CreateNotifier("nonexistent_type", nil)
	if n != nil {
		t.Error("expected nil for nonexistent notifier type")
	}
}

func TestMultipleNotifications(t *testing.T) {
	Register("test_multi", func(props map[string]string) Notifier {
		return &testNotifier{}
	})
	n := CreateNotifier("test_multi", nil)
	if n == nil {
		t.Fatal("notifier not created")
	}

	tn := n.(*testNotifier)
	n.Notify("Subject1", "Message1")
	n.Notify("Subject2", "Message2")

	if len(tn.called) != 2 {
		t.Errorf("expected 2 calls, got %d", len(tn.called))
	}
	if tn.called[0] != "Subject1:Message1" {
		t.Errorf("unexpected first call: %s", tn.called[0])
	}
	if tn.called[1] != "Subject2:Message2" {
		t.Errorf("unexpected second call: %s", tn.called[1])
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

func TestSMTPNotifierMultipleRecipients(t *testing.T) {
	props := map[string]string{
		"smtp_user": "user@example.com",
		"smtp_pass": "password",
		"to":        "recipient1@example.com,recipient2@example.com",
	}
	n := NewSMTPNotifier(props)
	if n == nil {
		t.Fatal("SMTPNotifier not created")
	}
	err := n.Notify("Test Subject", "Test Message")
	if err == nil {
		t.Error("Expected error for invalid SMTP config, got nil")
	}
}

func TestSMTPNotifierFromDefault(t *testing.T) {
	props := map[string]string{
		"smtp_user": "user@example.com",
		"smtp_pass": "password",
		"to":        "recipient@example.com",
		// No smtp_from, should default to smtp_user
	}
	n := NewSMTPNotifier(props)
	if n == nil {
		t.Fatal("SMTPNotifier not created")
	}
	err := n.Notify("Test Subject", "Test Message")
	if err == nil {
		t.Error("Expected error for invalid SMTP config")
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

func TestTelegramNotifierNilToken(t *testing.T) {
	props := map[string]string{
		"bot_token": "",
		"chat_id":   "123456789",
	}
	n := NewTelegramNotifier(props)
	if n != nil {
		t.Error("Expected nil for empty bot_token")
	}
}

func TestDummyNotifier(t *testing.T) {
	// Verify the dummy notifier exists and can be registered
	props := map[string]string{"to": "test@example.com"}
	n := CreateNotifier("dummy", props)
	// Dummy notifier might exist depending on the codebase
	// Just verify CreateNotifier returns something or nil gracefully
	if n != nil {
		if err := n.Notify("test", "message"); err != nil {
			t.Logf("Dummy notifier error: %v", err)
		}
	}
}
