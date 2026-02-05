package languagedb

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/golang/freetype/truetype"
	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app/i18n/font"
)

// LanguageMetadata holds the metadata for a language.
type LanguageMetadata struct {
	Code           string `json:"code"`
	DefaultCountry string `json:"defaultCountry"`
	Name           string `json:"name"`
}

// languageFont holds the font file name and size for a language.
type languageFont struct {
	File string  `json:"file"`
	Size float64 `json:"size"`
}

// languageFonts holds a list of font variations and sizes for a language.
type languageFonts map[string]languageFont

// languageTranslations contains a list of key-value pairs for translations for a language.
type languageTranslations map[string]string

// languageData holds the data and attributes for a language.
type languageData struct {
	Metadata     LanguageMetadata     `json:"metadata"`
	Fonts        languageFonts        `json:"fonts"`
	Translations languageTranslations `json:"translations"`
}

// LanguageDB manages the loading and retrieval of translations for multiple languages.
type LanguageDB struct {
	db  map[string]languageData
	log zerolog.Logger
}

// New creates a new LanguageDB instance and loads the embedded language files.
func New(log zerolog.Logger) (*LanguageDB, error) {
	translation := &LanguageDB{
		db:  make(map[string]languageData),
		log: log.With().Str("component", "i18n").Logger(),
	}

	err := translation.loadDBFromEmbeddedFS()
	if err != nil {
		return nil, fmt.Errorf("load languages: %w", err)
	}

	return translation, nil
}

// NewFromJSON creates a new LanguageDB instance and loads language data from the provided JSON byte slice.
func NewFromJSON(jsonData []byte, log zerolog.Logger) (*LanguageDB, error) {
	translation := &LanguageDB{
		db:  make(map[string]languageData),
		log: log.With().Str("component", "i18n").Logger(),
	}

	err := translation.loadDBFromJSON(jsonData)
	if err != nil {
		return nil, fmt.Errorf("load languages: %w", err)
	}

	return translation, nil
}

// Languages returns a map of all available languages with their associated metadata.
func (l *LanguageDB) Languages() map[string]LanguageMetadata {
	languages := make(map[string]LanguageMetadata)

	for code, lang := range l.db {
		languages[code] = lang.Metadata
	}

	return languages
}

// LanguageCodes returns a slice of all available language codes.
func (l *LanguageDB) LanguageCodes() []string {
	languages := l.Languages()

	codes := make([]string, 0, len(languages))
	for code := range languages {
		codes = append(codes, code)
	}

	return codes
}

// ValidateCode checks if the provided language code is valid.
func (l *LanguageDB) ValidateCode(code string) bool {
	languages := l.Languages()

	_, found := languages[code]

	return found
}

// String retrieves the translation for the given language code and key.
// It falls back to English if the translation is not found in the requested language.
// If the translation cannot be found even in English it returns an empty string.
func (l *LanguageDB) String(code string, key Key) (value string) {
	key = key.ToLower()

	if language, ok := l.db[code]; ok {
		if value, ok := language.Translations[key.String()]; ok {
			return value
		}
	} else {
		l.log.Warn().
			Str("key", key.String()).
			Str("lang code", code).
			Str("result", "failure").
			Msg("invalid language code")
	}

	l.log.Warn().
		Str("key", key.String()).
		Str("lang code", code).
		Str("result", "failure").
		Msg("translation lookup")

	// Fallback to English
	if code != "en" {
		value = l.String("en", key)
	}

	return value
}

// GetStringsWithPrefix retrieves all translations for the given language code that start with the specified prefix.
// It returns a map where keys are the full translation keys and values are the translated strings.
func (l *LanguageDB) GetStringsWithPrefix(code string, prefix string) map[string]string {
	result := make(map[string]string)
	prefix = strings.ToLower(prefix)

	if language, ok := l.db[code]; ok {
		for key, value := range language.Translations {
			if strings.HasPrefix(key, prefix) {
				result[key] = value
			}
		}
	}

	return result
}

// RegularFont returns the font used for displaying regular text for the given language code.
func (l *LanguageDB) RegularFont(code string) font.Font {
	return l.getFont("regular", code)
}

// ItalicFont returns the font used for displaying italic text for the given language code.
func (l *LanguageDB) ItalicFont(code string) font.Font {
	return l.getFont("italic", code)
}

// ValueFont returns the font used for displaying values for the given language code.
func (l *LanguageDB) ValueFont(code string) font.Font {
	return l.getFont("value", code)
}

// getFont returns the font for the given language code.
func (l *LanguageDB) getFont(variation string, code string) font.Font {
	var truetypeFont *truetype.Font

	var fontSize float64

	// Attempt to load the requested font
	if language, ok := l.db[code]; ok {
		var err error

		var fontName string

		switch strings.ToLower(variation) {
		case "regular":
			fontName = language.Fonts["regular"].File
			fontSize = language.Fonts["regular"].Size
		case "italic":
			fontName = language.Fonts["italic"].File
			fontSize = language.Fonts["italic"].Size
		case "value":
			fontName = language.Fonts["value"].File
			fontSize = language.Fonts["value"].Size
		default:
			fontName = language.Fonts["regular"].File
			fontSize = language.Fonts["regular"].Size

			l.log.Warn().
				Str("variation", variation).
				Msg("unknown font variation, defaulting to regular")
		}

		truetypeFont, err = font.GetFont(fontName)
		if err != nil {
			l.log.Error().
				Err(err).
				Str("lang code", code).
				Str("font", fontName).
				Msg("load regular font")
		}
	}

	// Fallback to English regular if the font couldn't be loaded
	if truetypeFont == nil && (code != "en" || variation != "regular") {
		truetypeFont = l.getFont("regular", "en").Font
	}

	return font.Font{
		Font:  truetypeFont,
		Scale: fontSize,
	}
}

//go:embed lang/*
var languageFiles embed.FS

// loadDBFromEmbeddedFS loads all language files from the embedded filesystem.
func (l *LanguageDB) loadDBFromEmbeddedFS() error {
	if len(l.db) > 0 {
		return errors.New("language database already loaded")
	}

	entries, err := languageFiles.ReadDir("lang")
	if err != nil {
		return fmt.Errorf("read embedded language directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			l.log.Warn().
				Str("file", entry.Name()).
				Msg("skipping non-JSON file in language directory")

			continue
		}

		data, err := languageFiles.ReadFile("lang/" + entry.Name())
		if err != nil {
			l.log.Error().
				Err(err).
				Str("file", entry.Name()).
				Msg("read embedded language file")

			continue
		}

		var langData languageData

		err = json.Unmarshal(data, &langData)
		if err != nil {
			l.log.Error().
				Err(err).
				Str("file", entry.Name()).
				Msg("unmarshal language JSON")

			continue
		}

		if langData.Metadata.Code != "" {
			l.db[langData.Metadata.Code] = langData
		}
	}

	return nil
}

// loadDBFromJSON loads language data from a JSON byte slice.
func (l *LanguageDB) loadDBFromJSON(data []byte) error {
	if len(l.db) > 0 {
		return errors.New("language database already loaded")
	}

	var langDB []languageData

	err := json.Unmarshal(data, &langDB)
	if err != nil {
		l.log.Error().
			Err(err).
			Msg("unmarshal language JSON")
	}

	for _, langData := range langDB {
		if langData.Metadata.Code != "" {
			l.db[langData.Metadata.Code] = langData
		}
	}

	return nil
}
