package bow

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/goodsign/monday"
	"golang.org/x/text/language"
)

const (
	defaultLocale = "en_US"
	placeholder   = "%"
)

type index map[string]string

// Translator allows to translate a message from english to a predefined
// set of locales parsed from csv files. Il also deals with date and time formats.
type Translator struct {
	dict      map[string]index
	regexDict map[string]index // used for translations with placeholders
	matcher   language.Matcher
}

// NewTranslator creates a translator.
func NewTranslator() *Translator {
	return &Translator{
		dict:      make(map[string]index),
		regexDict: make(map[string]index),
	}
}

// Parse parses all the csv files in the translations folder and
// build dictionnary maps that will serve as databases for translations.
// The name of csv file should be a BCP 47 compatible string.
// When % is used in a csv translation, it will serve as a placeholder
// and its value won’t be altered during the translation.
func (tr *Translator) Parse(fsys fs.FS) error {
	matches, err := fs.Glob(fsys, "translations/*.csv")
	if err != nil {
		return err
	}

	// first tag is the default one
	tags := []language.Tag{language.Make(defaultLocale)}

	for _, path := range matches {
		base := filepath.Base(path)
		lang := strings.TrimSuffix(base, filepath.Ext(base))

		tag, err := language.Parse(lang)
		if err != nil {
			return fmt.Errorf("language %s is not valid", lang)
		}

		locale := localeFromTag(tag)
		tr.dict[locale], tr.regexDict[locale], err = parseIndex(fsys, path)
		if err != nil {
			return err
		}

		tags = append(tags, tag)
	}

	tr.matcher = language.NewMatcher(tags)

	return nil
}

// parseIndex parses a csv file that contains key values entries into index maps.
// It returns one regular index map for static translations, and a regex index map
// which contains regex patterns and replacements for translations with placeholders.
func parseIndex(fsys fs.FS, path string) (index, index, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, nil, err
	}

	index := make(map[string]string)
	regIndex := make(map[string]string)

	r := csv.NewReader(f)
	for {
		line, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, nil, err
		}
		if len(line) != 2 {
			return nil, nil, errors.New("error reading csv file")
		}

		pat := fmt.Sprintf("(^|[^%s])%s([^%s]|$)", placeholder, placeholder, placeholder)
		re := regexp.MustCompile(pat)

		// no placeholder found
		if !re.MatchString(line[0]) {
			index[line[0]] = line[1]
			continue
		}

		key := re.ReplaceAllString(line[0], `${1}(.+)${2}`)

		var i = 0
		val := re.ReplaceAllStringFunc(line[1], func(s string) string {
			i++
			return strings.ReplaceAll(s, placeholder, fmt.Sprintf("${%d}", i))
		})

		regIndex[key] = val
	}

	return index, regIndex, nil
}

// Translate translates a message into the language of the corresponding locale.
// If the locale or the message is not found, it will be returned untranslated.
func (tr *Translator) Translate(msg string, locale string) string {
	if locale == defaultLocale {
		return msg
	}

	if _, ok := tr.dict[locale]; !ok {
		return msg
	}

	out, ok := tr.dict[locale][msg]
	if ok {
		return out
	}

	for k, v := range tr.regexDict[locale] {
		re := regexp.MustCompile(k)
		if !re.MatchString(msg) {
			continue
		}

		out = re.ReplaceAllString(msg, v)
	}

	if out != "" {
		return out
	}

	return msg
}

// FormatDateTime formats the time as "1/2/06 3:04 PM" in the given locale.
func FormatDateTime(t time.Time, locale string) string {
	mlocale := mondayLocale(locale)
	return monday.Format(t, monday.DateTimeFormatsByLocale[mlocale], mlocale)
}

// FormatDateFull formats the time as "Monday, January 2, 2006" in the given locale.
func FormatDateFull(t time.Time, locale string) string {
	mlocale := mondayLocale(locale)
	return monday.Format(t, monday.FullFormatsByLocale[mlocale], mlocale)
}

// FormatDateLong formats the time as "January 2, 2006" in the given locale.
func FormatDateLong(t time.Time, locale string) string {
	mlocale := mondayLocale(locale)
	return monday.Format(t, monday.LongFormatsByLocale[mlocale], mlocale)
}

// FormatDateMedium formats the time as "Jan 02, 2006" in the given locale.
func FormatDateMedium(t time.Time, locale string) string {
	mlocale := mondayLocale(locale)
	return monday.Format(t, monday.MediumFormatsByLocale[mlocale], mlocale)
}

// FormatDateShort formats the time as "1/2/06" in the given locale.
func FormatDateShort(t time.Time, locale string) string {
	mlocale := mondayLocale(locale)
	return monday.Format(t, monday.ShortFormatsByLocale[mlocale], mlocale)
}

// FormatTime formats the time as "3:04 PM" in the given locale.
func FormatTime(t time.Time, locale string) string {
	mlocale := mondayLocale(locale)
	return monday.Format(t, monday.TimeFormatsByLocale[mlocale], mlocale)
}

// LangCode returns the language code of a locale.
func LangCode(locale string) string {
	base, _ := language.Make(locale).Base()
	return base.String()
}

// ReqLocale returns the locale gathered from the request.
// It tries to retrieve it first using the "lang" cookie and otherwise
// using the "Accept-Language" request header. If the locale is not recognized
// or not supported, it will return the default locale (en_US).
func (tr *Translator) ReqLocale(r *http.Request) string {
	lang, _ := r.Cookie("lang")
	tag, _ := language.MatchStrings(tr.matcher, lang.String(), r.Header.Get("Accept-Language"))
	return localeFromTag(tag)
}

// localeFromTag returns a locale from a language tag.
func localeFromTag(tag language.Tag) string {
	base, _ := tag.Base()
	region, _ := tag.Region()
	return fmt.Sprintf("%s_%s", base, region)
}

// mondayLocale returns a monday.Locale from a locale.
// If the locale does not exist in monday, the default one is returned.
func mondayLocale(locale string) monday.Locale {
	for _, mlocale := range monday.ListLocales() {
		if locale == string(mlocale) {
			return mlocale
		}
	}
	return monday.Locale(defaultLocale)
}
