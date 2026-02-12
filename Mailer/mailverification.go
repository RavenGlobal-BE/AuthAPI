package mailer

import (
	"net/mail"
	"strings"
)

//var InvalidEmailError = errors.New("Invalid E-Mail")

// Tests an email for basic validity
func EmailIsValid(email string) bool {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}

	mailParts := strings.Split(strings.ToLower(addr.Address), "@")

	user, domain := mailParts[0], mailParts[1]

	switch domain {
	case "gmail.com":
		return !strings.Contains(user, "+")
	}

	return true
}
