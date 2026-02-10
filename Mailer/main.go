package mailer

import (
	"net/smtp"
	"os"
	"strings"
)

func Send() {
	var auth = smtp.PlainAuth(
		"",
		"", //Username
		"", //Password
		"email-smtp.eu-north-1.amazonaws.com",
	)

	htmlBytes, err := os.ReadFile("Mailer/mailVerificationTemplate.html")
	if err != nil {
		panic(err)
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
		auth,
		"no-reply@raven.co.com",
		[]string{to},
		message,
	)

	if err != nil {
		panic(err)
	}
}
