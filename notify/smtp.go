package notify

import (
	"crypto/tls"
	"net/smtp"
	"strings"

	"github.com/jordan-wright/email"
)

type SMTPNotifier struct {
	props map[string]string
}

func (s *SMTPNotifier) Notify(subject, message string) error {
	from := s.props["smtp_from"]
	if from == "" {
		from = s.props["smtp_user"]
	}
	to := strings.Split(s.props["to"], ",")
	e := email.NewEmail()
	e.From = from
	e.To = to
	e.Subject = subject
	e.Text = []byte(message)

	host := s.props["smtp_server"]
	port := s.props["smtp_port"]
	if port == "" {
		port = "25"
	}
	addr := host + ":" + port
	user := s.props["smtp_user"]
	pass := s.props["smtp_pass"]
	security := strings.ToLower(s.props["smtp_security"])

	switch security {
	case "ssl", "tls":
		return e.SendWithTLS(addr, smtp.PlainAuth("", user, pass, host), &tls.Config{ServerName: host})
	case "starttls":
		c, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		defer c.Close()
		if err = c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return err
		}
		if err = c.Auth(smtp.PlainAuth("", user, pass, host)); err != nil {
			return err
		}
		if err = c.Mail(from); err != nil {
			return err
		}
		for _, addr := range to {
			if err = c.Rcpt(addr); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err = w.Write([]byte(message)); err != nil {
			return err
		}
		return w.Close()
	default: // plain
		return e.Send(addr, smtp.PlainAuth("", user, pass, host))
	}
}

func NewSMTPNotifier(props map[string]string) Notifier {
	return &SMTPNotifier{props: props}
}

func init() {
	Register("smtp", NewSMTPNotifier)
}
