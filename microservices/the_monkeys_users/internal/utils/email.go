package utils

import (
	"errors"
	"fmt"
	"net/smtp"

	"github.com/the-monkeys/the_monkeys/config"
)

// SendMail sends an HTML email to the specified address using SMTP settings from config.
func SendMail(cfg *config.Config, toEmail, htmlBody string) error {
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
	if address == "" && host != "" {
		address = host + ":587"
	}

	if fromEmail == "" || smtpPassword == "" || host == "" || address == "" {
		return errors.New("email SMTP configuration incomplete")
	}

	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Your Monkeys Account Has Been Deleted\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n",
		fromEmail, toEmail,
	)

	auth := smtp.PlainAuth("", fromEmail, smtpPassword, host)
	return smtp.SendMail(address, auth, fromEmail, []string{toEmail}, []byte(headers+htmlBody))
}

// AccountDeletedEmailHTML returns the farewell email body sent after account deletion.
func AccountDeletedEmailHTML(firstName, lastName, username string) string {
	name := firstName
	if lastName != "" {
		name = firstName + " " + lastName
	}
	if name == "" {
		name = username
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;font-family:'Segoe UI',Tahoma,Geneva,Verdana,sans-serif;background-color:#f4f4f4">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f4;padding:40px 0">
<tr><td align="center">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,0.08)">

<!-- Header -->
<tr><td style="background-color:#1a1a2e;padding:32px 40px;text-align:center">
  <h1 style="color:#ffffff;margin:0;font-size:24px">Account Deleted</h1>
</td></tr>

<!-- Body -->
<tr><td style="padding:40px">
  <p style="color:#333;font-size:16px;line-height:1.6;margin:0 0 16px">Hello %s,</p>

  <p style="color:#333;font-size:16px;line-height:1.6;margin:0 0 16px">
    Your Monkeys account <strong>@%s</strong> has been permanently deleted as you requested.
  </p>

  <div style="background-color:#fff3cd;border-left:4px solid #ffc107;padding:16px;border-radius:4px;margin:24px 0">
    <p style="color:#856404;font-size:14px;line-height:1.6;margin:0">
      <strong>⚠ This action is irreversible.</strong> The following data has been permanently removed and <strong>cannot be recovered</strong>:
    </p>
    <ul style="color:#856404;font-size:14px;line-height:1.8;margin:8px 0 0;padding-left:20px">
      <li>Your profile and account information</li>
      <li>All blogs you authored (content and files)</li>
      <li>Your profile photos and uploaded media</li>
      <li>Likes, bookmarks, comments, and follows</li>
      <li>Co-author permissions and invitations</li>
      <li>Notification preferences</li>
    </ul>
  </div>

  <p style="color:#333;font-size:16px;line-height:1.6;margin:0 0 16px">
    If you did not request this deletion, please contact our support team immediately.
  </p>

  <p style="color:#666;font-size:14px;line-height:1.6;margin:24px 0 0">
    Thank you for being part of The Monkeys community. We wish you all the best.
  </p>
</td></tr>

<!-- Footer -->
<tr><td style="background-color:#f8f9fa;padding:24px 40px;text-align:center;border-top:1px solid #e9ecef">
  <p style="color:#6c757d;font-size:13px;margin:0 0 8px">
    Need help? <a href="https://monkeys.com.co/contact-us" style="color:#007bff;text-decoration:none">Contact Support</a>
    &nbsp;|&nbsp; <a href="mailto:monkeys.admin@monkeys.com.co" style="color:#007bff;text-decoration:none">monkeys.admin@monkeys.com.co</a>
  </p>
  <p style="color:#adb5bd;font-size:12px;margin:0">© The Monkeys — You received this because your account was deleted.</p>
</td></tr>

</table>
</td></tr>
</table>
</body>
</html>`, name, username)
}
