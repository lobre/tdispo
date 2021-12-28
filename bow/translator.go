package bow

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/goodsign/monday"
	"golang.org/x/text/language"
)

const defaultLocale = "en_US"

type index map[string]string

// Translator allows to translate a message from english to a predefined
// set of languages parsed from csv files. Il also deals with date and time formats.
type Translator struct {
	dict    map[string]index
	matcher language.Matcher
}

// NewTranslator creates a translator.
func NewTranslator() *Translator {
	return &Translator{
		dict: make(map[string]index),
	}
}

// Parse parses all the csv files in the translations folder
// and build a dictionnary map that will serve as the database
// for translations.
func (tr *Translator) Parse(fsys fs.FS) error {
	matches, err := fs.Glob(fsys, "translations/*.csv")
	if err != nil {
		return err
	}

	// set en_US as default tag
	tags := []language.Tag{language.Make(defaultLocale)}

	for _, path := range matches {
		base := filepath.Base(path)
		lang := strings.TrimSuffix(base, filepath.Ext(base))

		tag, err := language.Parse(lang)
		if err != nil {
			return fmt.Errorf("language %s is not valid", lang)
		}

		tr.dict[localeFromTag(tag)], err = parseIndex(fsys, path)
		if err != nil {
			return err
		}

		tags = append(tags, tag)
	}

	tr.matcher = language.NewMatcher(tags)

	return nil
}

// parseIndex parses a csv file that contains key values entries into an index map.
func parseIndex(fsys fs.FS, path string) (index, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}

	index := make(map[string]string)

	r := csv.NewReader(f)
	for {
		line, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if len(line) != 2 {
			return nil, errors.New("error reading csv file")
		}

		index[line[0]] = line[1]
	}

	return index, nil
}

// Translate translates a message into the corresponding language.
// If the language or the message is not found, it will be returned untranslated.
func (tr *Translator) Translate(msg string, lang string) string {
	locale := localeFromTag(language.Make(lang))
	if locale == defaultLocale {
		return msg
	}

	if _, ok := tr.dict[locale]; !ok {
		return msg
	}

	out, ok := tr.dict[locale][msg]
	if !ok {
		return msg
	}

	return out
}

// FormatDateTime formats the time as "1/2/06 3:04 PM" in the given language.
func FormatDateTime(t time.Time, lang string) string {
	mlocale := mondayLocale(lang)
	return monday.Format(t, monday.DateTimeFormatsByLocale[mlocale], mlocale)
}

// FormatDateFull formats the time as "Monday, January 2, 2006" in the given language.
func FormatDateFull(t time.Time, lang string) string {
	mlocale := mondayLocale(lang)
	return monday.Format(t, monday.FullFormatsByLocale[mlocale], mlocale)
}

// FormatDateLong formats the time as "January 2, 2006" in the given language.
func FormatDateLong(t time.Time, lang string) string {
	mlocale := mondayLocale(lang)
	return monday.Format(t, monday.LongFormatsByLocale[mlocale], mlocale)
}

// FormatDateMedium formats the time as "Jan 02, 2006" in the given language.
func FormatDateMedium(t time.Time, lang string) string {
	mlocale := mondayLocale(lang)
	return monday.Format(t, monday.MediumFormatsByLocale[mlocale], mlocale)
}

// FormatDateShort formats the time as "1/2/06" in the given language.
func FormatDateShort(t time.Time, lang string) string {
	mlocale := mondayLocale(lang)
	return monday.Format(t, monday.ShortFormatsByLocale[mlocale], mlocale)
}

// FormatTime formats the time as "3:04 PM" in the given language.
func FormatTime(t time.Time, lang string) string {
	mlocale := mondayLocale(lang)
	return monday.Format(t, monday.TimeFormatsByLocale[mlocale], mlocale)
}

// ReqLang returns the language gathered from the request.
// It tries to retrieve it first using the "lang" cookie and otherwise
// using the "Accept-Language" request header. If the language is not recognized
// or not supported, it will return the default language (en_US).
func (tr *Translator) ReqLang(r *http.Request) string {
	lang, _ := r.Cookie("lang")
	tag, _ := language.MatchStrings(tr.matcher, lang.String(), r.Header.Get("Accept-Language"))
	return localeFromTag(tag)
}

// LangCode returns the language code a language.
func LangCode(lang string) string {
	base, _ := language.Make(lang).Base()
	return base.String()
}

// localeFromTag returns a locale from a language tag.
func localeFromTag(tag language.Tag) string {
	base, _ := tag.Base()
	region, _ := tag.Region()
	return fmt.Sprintf("%s_%s", base, region)
}

// mondayLocale returns a monday.Locale from a lang.
// If the locale corresponding to the lang does not exist in monday,
// the default locale is returned.
func mondayLocale(lang string) monday.Locale {
	locale := localeFromTag(language.Make(lang))
	for _, mlocale := range monday.ListLocales() {
		if locale == string(mlocale) {
			return mlocale
		}
	}
	return monday.Locale(defaultLocale)
}
