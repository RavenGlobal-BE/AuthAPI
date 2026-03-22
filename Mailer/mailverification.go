package mailer

import (
	"net/mail"
	"regexp"
	"strings"
)

//var InvalidEmailError = errors.New("Invalid E-Mail")

// Tests an email for basic validity
func EmailIsValid(email string) bool {
	if email == "" {
		return false
	}

	email = strings.TrimSpace(email)
	addr, err := mail.ParseAddress(email)
	match, _ := regexp.MatchString("[a-z]", addr.Address)

	if err != nil || !match || addr.Address != email {
		return false
	}

	mailParts := strings.Split(strings.ToLower(addr.Address), "@")
	user, domain := mailParts[0], mailParts[1]

	switch domain {
	case "gmail.com":
		return !strings.Contains(user, "+")
	case "yahoo.com":
		return !strings.Contains(user, "!")
	}

	return true
}
