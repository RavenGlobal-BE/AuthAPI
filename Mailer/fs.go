package mailer

import (
	"bytes"
	"embed"
	"encoding/json/v2"
	"html/template"
)

//go:embed templates/*.html
var tmplFS embed.FS

//go:embed locales/*.json
var localeFS embed.FS

func loadLocale(lang string) (Locale, error) {
	var loc Locale
	data, err := localeFS.ReadFile("locales/" + lang + ".json")
	if err != nil {
		return loc, err // no such language
	}
	err = json.Unmarshal(data, &loc)
	return loc, err
}

func renderPage(name string, data any) (string, error) {
	t, err := template.New(name+".html").Funcs(funcs).ParseFS(tmplFS, "templates/"+name+".html")
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer

	// Compares the data of the email against the name of the object and replaces them with the correct value
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// EmailText contains a set of default values that will be shown in an email. This is the universal struct.
type EmailText struct {
	Subject       string `json:"subject"`
	Preheader     string `json:"preheader"`
	Heading       string `json:"heading"`
	Greeting      string `json:"greeting"`
	Intro         string `json:"intro"`
	Button        string `json:"button"`
	FallbackLabel string `json:"fallback_label"`
	Expiry        string `json:"expiry"`
	Disclaimer    string `json:"disclaimer"`
}

type emailData struct {
	Lang, Dir, Name, Link string
	Year                  int
	E                     any    // Holds the translated text of a given email
	X                     any    // Holds custom data that's needed to make the email complete.
	T                     Locale // the whole locale (the template uses .T.Footer)
}

type Locale struct {
	Meta struct {
		Dir string `json:"dir"` // "ltr" or "rtl"
	} `json:"meta"`

	AccountVerification EmailText `json:"account_verification"`
	ResetPassword       EmailText `json:"password_reset"`

	Footer struct {
		Question string `json:"question"`
		KBLink   string `json:"kb_link"`
		Rights   string `json:"rights"`
	} `json:"footer"`
}
