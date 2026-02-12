package mailer

import "testing"

func TestEmailIsValid(t *testing.T) {
	var emailsToTest = map[string]bool{
		"imadamroug+amroug@gmail.com": false,
		"imadamroug89@gmail.com":      true,
		"imad.amroug@outlook.com":     true,
		"randomtext":                  false,
		"imad@raven.co.com":           true,
		"randomtext@.com":             false,
	}

	//Key    -> Email
	//Value  -> Expected Result
	for key, value := range emailsToTest {
		if result := EmailIsValid(key); result != value {
			t.Errorf("Expected \"%v\" for email: %s", value, key)
		}
	}
}
