package authorization

import (
	"errors"
	"net/mail"
	"strings"
)

var InvalidEmailError = errors.New("Invalid E-Mail")

// Tests an email for basic validity
func EmailIsValid(email string) bool {
	addr, err := mail.ParseAddress(email)

	if err != nil {
		return false
	}

	mail := strings.Split(addr.Address, "@")

	gmailCheck := gmailUserVerification(mail[0], mail[1]) // mail[1] to remove the "@" character

	if !gmailCheck {
		return false
	}

	return true
}

func gmailUserVerification(username string, domain string) bool { //Checking the part before the @
	if strings.ToLower(domain) != "gmail.com" && strings.ToLower(domain) != "googlemail.com" {
		return true //If the domain is not gmail, skip further checks & just return as valid.
	}

	if strings.Contains(username, "+") {
		return false
	}

	return true
}
