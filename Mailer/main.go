package mailer

import (
	"net/smtp"
	"os"
	logger "raven/auth/Logging"
	"strings"
)

type MailService struct {
	smtpAuth *smtp.Auth
}

func NewSmtpService(username string, password string, host string) *MailService {
	auth := smtp.PlainAuth(
		"",
		username, //Username
		password, //Password
		host,     //Host
	)

	return &MailService{
		smtpAuth: &auth,
	}
}

func (ms *MailService) Send() {
	htmlBytes, err := os.ReadFile("Mailer/mailVerificationTemplate.html")
	if err != nil {
		logger.Log("Unable to read email template: "+err.Error(), logger.Error)
		return
	}

	html := string(htmlBytes)

	html = strings.ReplaceAll(html, "{name}", "Farshad")

	from := "Raven ONE Account System <no-reply@raven.co.com>"
	to := "imadamroug89@gmail.com"
	subject := "Verify your account"
	htmlBody := html

	message := []byte("From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: " + subject + "\n" +
		"MIME-Version: 1.0\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\n" +
		"\n" +
		htmlBody)

	err = smtp.SendMail(
		"email-smtp.eu-north-1.amazonaws.com:587",
		*ms.smtpAuth,
		"no-reply@raven.co.com",
		[]string{to},
		message,
	)

	if err != nil {
		logger.Log("Unable to sign into AWS SES: "+err.Error(), logger.Error)
		return
	}
}
