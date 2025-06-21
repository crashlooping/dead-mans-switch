package notify

import (
	"fmt"
	"net/smtp"
)

type SMTPNotifier struct {
	To     string
	Server string
	User   string
	Pass   string
	From   string
}

func (s *SMTPNotifier) Notify(subject, message string) error {
	from := s.From
	if from == "" {
		from = s.User
	}
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n", from, s.To, subject)
	body := headers + message
	auth := smtp.PlainAuth("", s.User, s.Pass, s.Server)
	addr := s.Server
	if !containsPort(addr) {
		addr += ":587"
	}
	return smtp.SendMail(addr, auth, from, []string{s.To}, []byte(body))
}

func containsPort(addr string) bool {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return true
		}
	}
	return false
}

func NewSMTPNotifier(props map[string]string) Notifier {
	return &SMTPNotifier{
		To:     props["to"],
		Server: props["smtp_server"],
		User:   props["smtp_user"],
		Pass:   props["smtp_pass"],
		From:   props["smtp_from"],
	}
}

func init() {
	Register("smtp", NewSMTPNotifier)
}
