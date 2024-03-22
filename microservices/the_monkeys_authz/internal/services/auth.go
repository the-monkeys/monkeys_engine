package services

import (
	"net/smtp"

	"github.com/sirupsen/logrus"
)

func (srv *AuthzSvc) SendMail(email, emailBody string) error {
	logrus.Infof("Send mail routine triggered")

	fromEmail := srv.config.Email.SMTPMail        //ex: "John.Doe@gmail.com"
	smtpPassword := srv.config.Email.SMTPPassword // ex: "ieiemcjdkejspqz"
	address := srv.config.Email.SMTPAddress
	to := []string{email}

	subject := "Subject: The Monkeys support\n"

	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	message := []byte(subject + mime + emailBody)

	auth := smtp.PlainAuth("", fromEmail, smtpPassword, srv.config.Email.SMTPHost)

	if err := smtp.SendMail(address, auth, fromEmail, to, message); err != nil {
		logrus.Errorf("error occurred while sending verification email, error: %+v", err)
		return nil

	}

	return nil
}
