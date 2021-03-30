package email

import (
	"strconv"

	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

type Email struct {
	cfg    Config
	logger *zap.Logger
}

func NewEmail(conf Config, logger *zap.Logger) Email {
	return Email{
		cfg:    conf,
		logger: logger,
	}
}

func (e *Email) SendMail(mailTo []string, subject, body string) error {
	port, _ := strconv.Atoi(e.cfg.Port)
	m := gomail.NewMessage()
	m.SetHeader("From", "<"+e.cfg.User+">")
	m.SetHeader("To", mailTo...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)
	d := gomail.NewDialer(e.cfg.Host, port, e.cfg.User, e.cfg.Pwd)
	err := d.DialAndSend(m)
	return err
}
