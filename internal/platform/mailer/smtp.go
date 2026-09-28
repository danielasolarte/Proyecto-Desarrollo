package mailer

import (
	"fmt"
	"net/smtp"
)

type SMTPMailer struct {
	Addr string
	From string
}

func NewSMTPMailer(addr, from string) *SMTPMailer {
	return &SMTPMailer{Addr: addr, From: from}
}

func (m *SMTPMailer) Send(to, subject, body string) error {
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", m.From, to, subject, body))
	return smtp.SendMail(m.Addr, nil, m.From, []string{to}, msg)
}
