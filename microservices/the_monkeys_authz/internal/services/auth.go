package services

import (
	"errors"
	"net/smtp"

	"github.com/sirupsen/logrus"
)

func (srv *AuthzSvc) SendMail(email, emailBody string) error {
	logrus.Infof("Send mail routine triggered")

	cfg := srv.config

	// Prefer generic email config; fallback to gmail config if generic is empty
	fromEmail := cfg.Email.SMTPMail
	if fromEmail == "" {
		fromEmail = cfg.Gmail.SMTPMail
	}

	smtpPassword := cfg.Email.SMTPPassword
	if smtpPassword == "" {
		smtpPassword = cfg.Gmail.SMTPPassword
	}

	host := cfg.Email.SMTPHost
	if host == "" {
		host = cfg.Gmail.SMTPHost
	}

	address := cfg.Email.SMTPAddress
	if address == "" {
		address = cfg.Gmail.SMTPAddress
	}

	// If address still empty but we have a host, default to :587
	if address == "" && host != "" {
		address = host + ":587"
	}

	if fromEmail == "" || smtpPassword == "" || host == "" || address == "" {
		return errors.New("email smtp configuration incomplete (check .env variables)")
	}

	// Build message with proper headers (CRLF) so some servers don't reject it
	subject := "Subject: The Monkeys support\r\n"
	headers := "From: " + fromEmail + "\r\n" +
		"To: " + email + "\r\n" +
		subject +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n"

	message := []byte(headers + emailBody)

	auth := smtp.PlainAuth("", fromEmail, smtpPassword, host)

	if err := smtp.SendMail(address, auth, fromEmail, []string{email}, message); err != nil {
		logrus.Errorf("error occurred while sending verification email to %s: %v", email, err)
		return err
	}

	logrus.Infof("verification email successfully sent to %s", email)
	return nil
}
