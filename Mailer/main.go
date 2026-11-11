package mailer

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	logger "raven/auth/Logging"
	"strings"
	"time"
)

//go:embed templates/mailVerification.html
var mailVerificationTemplate string

//go:embed templates/resetPassword.html
var resetPasswordTemplate string

type MailService struct {
	smtpAuth smtp.Auth
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
		smtpAuth: auth,
		host:     host,
	}
}

var templates = map[string]string{
	"mailVerification": mailVerificationTemplate,
	"resetPassword":    resetPasswordTemplate,
}

var funcs = template.FuncMap{
	"raw": func(s string) template.HTML { return template.HTML(s) }, // only for the template's own markup, never user data
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

func (ms *MailService) Send(subject, recipient, html string) error {
	// Rejects anything that isn't a single valid address, including
	// values with line breaks that could inject extra headers.
	to, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient %q: %w", recipient, err)
	}

	// Quoted-printable keeps every line under the 998-byte SMTP limit.
	// Your button line is already about 700 characters before the link is added.
	var body bytes.Buffer
	qp := quotedprintable.NewWriter(&body)
	if _, err := qp.Write([]byte(html)); err != nil {
		return err
	}
	if err := qp.Close(); err != nil {
		return err
	}

	var msg bytes.Buffer
	msg.WriteString("From: Raven <no-reply@raven.co.com>\r\n")
	msg.WriteString("To: " + to.String() + "\r\n")
	msg.WriteString("Subject: " + mime.BEncoding.Encode("UTF-8", subject) + "\r\n") // non-ASCII subjects
	msg.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.Write(body.Bytes())

	if err := smtp.SendMail(ms.host+":587", ms.smtpAuth, "no-reply@raven.co.com", []string{to.Address}, msg.Bytes()); err != nil {
		logger.Log("Failed to send email via SES: "+err.Error(), logger.Error)
		return err
	}
	return nil
}

func (ms *MailService) AccountVerificationEmail(recipient, name, token, lang string) error {
	loc, err := loadLocale(lang)
	if err != nil {
		lang = "en"
		if loc, err = loadLocale("en"); err != nil {
			return err
		}
	}

	link := fmt.Sprintf("%s/%s/accountRecovery/password?resetcode=%s",
		os.Getenv("FRONTEND_URL"), lang, url.QueryEscape(token))

	html, err := renderPage("mailVerification", emailData{
		Lang: lang,
		Dir:  loc.Meta.Dir,
		Name: name,
		Link: link,
		Year: time.Now().Year(),
		E:    loc.AccountVerification, // AccountVerificationEmail passes loc.AccountVerification
		T:    loc,
	})
	if err != nil {
		return err
	}

	return ms.Send(loc.AccountVerification.Subject, recipient, html)
}

func (ms *MailService) ResetPasswordEmail(recipient, name, token, lang string) error {
	loc, err := loadLocale(lang)
	if err != nil {
		lang = "en"
		if loc, err = loadLocale("en"); err != nil {
			return err
		}
	}

	link := fmt.Sprintf("%s/%s/accountRecovery/password?resetcode=%s",
		os.Getenv("FRONTEND_URL"), lang, url.QueryEscape(token))

	html, err := renderPage("resetPassword", emailData{
		Lang: lang,
		Dir:  loc.Meta.Dir,
		Name: name,
		Link: link,
		Year: time.Now().Year(),
		E:    loc.ResetPassword, // AccountVerificationEmail passes loc.AccountVerification
		T:    loc,
	})
	if err != nil {
		return err
	}

	return ms.Send(loc.ResetPassword.Subject, recipient, html)
}
