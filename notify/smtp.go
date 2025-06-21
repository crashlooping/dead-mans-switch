package notify

import (
	"context"

	"github.com/nikoksr/notify"
	notifyMail "github.com/nikoksr/notify/service/mail"
)

type SMTPNotifier struct {
	n *notify.Notify
}

func (s *SMTPNotifier) Notify(subject, message string) error {
	return s.n.Send(context.Background(), subject, message)
}

func NewSMTPNotifier(props map[string]string) Notifier {
	mail := notifyMail.New(props["smtp_user"], props["smtp_pass"])
	mail.AddReceivers(props["to"])
	n := notify.New()
	n.UseServices(mail)
	return &SMTPNotifier{n: n}
}

func init() {
	Register("smtp", NewSMTPNotifier)
}
