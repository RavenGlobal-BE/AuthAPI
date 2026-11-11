package mailer

import (
	"bytes"
	"embed"
	"encoding/json/v2"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/*.html
var tmplFS embed.FS

//go:embed locales/*.json
var localeFS embed.FS

var langs = map[string]Locale{}
var templates = map[string]*template.Template{}

func loadLocale(lang string) (Locale, error) {
	loc, ok := langs[lang]
	if !ok {
		return Locale{}, fmt.Errorf("unsupported language %q", lang)
	}
	return loc, nil
}

func renderPage(name string, data any) (string, error) {
	t, ok := templates[name]
	if !ok {
		return "", fmt.Errorf("unsupported template %q", name)
	}

	var buf bytes.Buffer

	// Compares the data of the email against the name of the object and replaces them with the correct value
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (ms *MailService) LoadAllLocales() error {
	dirs, err := localeFS.ReadDir("locales")
	if err != nil {
		return err
	}

	for _, f := range dirs {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}

		data, err := localeFS.ReadFile("locales/" + f.Name())
		if err != nil {
			return err // error reading the file
		}

		var loc Locale
		if err := json.Unmarshal(data, &loc); err != nil {
			return fmt.Errorf("locale %s: %w", f.Name(), err)
		}

		langs[strings.TrimSuffix(f.Name(), ".json")] = loc
	}
	return nil
}
func (ms *MailService) LoadAllTemplates() error {
	dirs, err := tmplFS.ReadDir("templates")
	if err != nil {
		return err
	}

	for _, f := range dirs {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".html") {
			continue
		}

		t, err := template.New(f.Name()).Funcs(funcs).ParseFS(tmplFS, "templates/"+f.Name())
		if err != nil {
			return fmt.Errorf("template %s: %w", f.Name(), err)
		}

		templates[strings.TrimSuffix(f.Name(), ".html")] = t
	}
	return nil
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
