package mailer

import (
	_ "embed"
	"fmt"
	"net/smtp"
	"os"
	logger "raven/auth/Logging"
	"strings"
)

//go:embed mailVerification.html
var mailVerificationTemplate string

//go:embed resetPassword.html
var resetPasswordTemplate string

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

var templates = map[string]string{
	"mailVerification": mailVerificationTemplate,
	"resetPassword":    resetPasswordTemplate,
}

func (ms *MailService) prepareTemplate(templateName string, data map[string]string) string {
	raw, ok := templates[templateName]
	if !ok {
		logger.Log("Unknown email template: "+templateName, logger.Error)
		return ""
	}

	html := raw
	for key, value := range data {
		html = strings.ReplaceAll(html, "{"+key+"}", value)
	}

	return html
}

func (ms *MailService) Send(subject, recipient, template string, parameters map[string]string) {
	html := ms.prepareTemplate(template, parameters)

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

func (ms *MailService) AccountVerificationEmail(recipient, name, token, lang string) {
	ms.Send(
		"Verify your account",
		recipient,
		"mailVerification",
		map[string]string{"name": name, "link": fmt.Sprintf("%s/%s/verify?code=%s", os.Getenv("FRONTEND_URL"), lang, token)},
	)
}

func (ms *MailService) ResetPasswordEmail(recipient, name, token, lang string) {
	subjects := map[string]string{
		"en": "Reset your password",
		"ar": "إعادة تعيين كلمة المرور",
		"fi": "Salasanan nollaus",
		"fr": "Réinitialiser votre mot de passe",
		"es": "Restablecer contraseña",
	}

	subject, ok := subjects[lang]
	if !ok {
		subject = subjects["en"]
	}

	ms.Send(
		subject,
		recipient,
		"resetPassword",
		map[string]string{"name": name, "link": fmt.Sprintf("%s/%s/accountRecovery/password?resetcode=%s", os.Getenv("FRONTEND_URL"), lang, token)},
	)
}
