package i18n

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app/i18n/font"
	"github.com/zetetos/simtezilo/app/i18n/languagedb"
)

// I18n represents a language and its associated translations.
type I18n struct {
	languageCode   string
	configLangCode *string
	languages      *languagedb.LanguageDB
	log            zerolog.Logger
}

// New creates a new I18n instance with the language set to the given code.
func New(langCode *string, log zerolog.Logger) (*I18n, error) {
	newI18n := &I18n{
		log: log.With().Str("component", "i18n").Logger(),
	}

	var err error

	newI18n.languages, err = languagedb.New(newI18n.log)
	if err != nil {
		return nil, fmt.Errorf("load languages: %w", err)
	}

	if !newI18n.languages.ValidateCode(*langCode) {
		log.Warn().Str("requested", *langCode).Msg("unsupported language, defaulting to 'en'")

		newI18n.languageCode = "en"
	}

	newI18n.languageCode = *langCode
	newI18n.configLangCode = langCode

	go newI18n.watchForConfigChanges()

	return newI18n, nil
}

// Languages returns a map of all available languages with their metadata.
func (i *I18n) Languages() map[string]languagedb.LanguageMetadata {
	return i.languages.Languages()
}

// LanguageCodes returns a slice of all available language codes.
func (i *I18n) LanguageCodes() []string {
	return i.languages.LanguageCodes()
}

// LanguageCode returns the currently configured language code.
func (i *I18n) LanguageCode() string {
	return i.languageCode
}

// SetLanguageCode sets the current language code.
// It returns an error if the language code is not supported.
func (i *I18n) SetLanguageCode(code string) error {
	if !i.languages.ValidateCode(code) {
		return fmt.Errorf("unsupported language code: %q", code)
	}

	i.languageCode = code
	*i.configLangCode = code

	return nil
}

// LanguageName returns the currently configured language code.
func (i *I18n) LanguageName() string {
	return i.languageCode
}

// GetString retrieves the translation for the given key.
func (i *I18n) GetString(key languagedb.Key) string {
	return i.languages.String(i.languageCode, key)
}

// GetStringsWithPrefix retrieves all translations that start with the specified prefix.
func (i *I18n) GetStringsWithPrefix(prefix string) map[string]string {
	return i.languages.GetStringsWithPrefix(i.languageCode, prefix)
}

// GetStringsWithPrefixForLanguage retrieves all translations for a specific language that start with the specified prefix.
func (i *I18n) GetStringsWithPrefixForLanguage(languageCode string, prefix string) map[string]string {
	return i.languages.GetStringsWithPrefix(languageCode, prefix)
}

// RegularFont returns the regular font for the language.
func (i *I18n) RegularFont() font.Font {
	return i.languages.RegularFont(i.languageCode)
}

// ValueFont returns the value font for the language.
func (i *I18n) ValueFont() font.Font {
	return i.languages.ValueFont(i.languageCode)
}

// TODO: is there a better way to integrate config changes?
// watchForConfigChanges monitors for changes in the configuration language code.
func (i *I18n) watchForConfigChanges() {
	i.log.Debug().Str("event", "start").Msg("config watch")

	for {
		if *i.configLangCode != i.languageCode {
			i.languageCode = *i.configLangCode
			i.log.Debug().Str("language", i.languageCode).Str("event", "change").Msg("config watch")
		}

		// TODO: this delay causes the setting title update to lag behind changes to the language
		time.Sleep(200 * time.Millisecond)
	}
}
