package mailer

import (
	"net/smtp"
	"os"
	logger "raven/auth/Logging"
	"strings"
)

type MailService struct {
	smtpAuth *smtp.Auth
	host     string
}

func NewSmtpService(username string, password string, host string) *MailService {
	auth := smtp.PlainAuth(
		"",
		username,
		password,
		host,
	)

	return &MailService{
		smtpAuth: &auth,
		host:     host,
	}
}

func (ms *MailService) prepareTemplate(template string, data map[string]string) string {
	htmlBytes, err := os.ReadFile("Mailer/" + template)
	if err != nil {
		logger.Log("Unable to read email template: "+err.Error(), logger.Error)
		return ""
	}

	html := string(htmlBytes)

	for key, value := range data {
		html = strings.ReplaceAll(html, "{"+key+"}", value)
	}

	return html
}

func (ms *MailService) Send(subject, recipient, template string, parameters map[string]string) {
	html := ms.prepareTemplate(template+".html", parameters)

	from := "Raven <no-reply@raven.co.com>"
	to := recipient
	htmlBody := html

	message := []byte("From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: " + subject + "\n" +
		"MIME-Version: 1.0\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\n" +
		"\n" +
		htmlBody)

	err := smtp.SendMail(
		ms.host+":587",
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
