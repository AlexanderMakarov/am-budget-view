package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const I18N_DATE_FORMAT = "2006-01-02"

var formatSpecifierRegexp = regexp.MustCompile(`{{([^}]+)}}`)

// Translation system is based on i18next, see https://www.i18next.com.
// Translation files should be compatible with i18next JS/TS libs.
// But "github.com/yuangwei/go-i18next" realization is inconvenient, so use our own for Go.
// Notes (differences from https://www.i18next.com):
// - Namespaces of i18next are not supported.
// - Only JSON is supported for translations.
// - Default fallback for any `T` issue is `[fall reason] key, %s` where `%s` is a comma-separated list of "%+v" of arguments.
// Supported built-in formatting functions:
// - number (signDisplay, maximumSignificantDigits, minimumSignificantDigits, maximumFractionDigits, minimumFractionDigits, minimumIntegerDigits),
// - currency (from `MoneyWith2DecimalPlaces`),
// - date (Golang `time.Format`, default is `I18N_DATE_FORMAT`),
// - list (not https://tc39.es/ecma402/#listformat-objects, only 'separator' property is supported, ', ' by-default).
// - indent (rightIndent, leftIndent),
// - object (Golang `%+v`),
// - values (Golang `%v`),
// - error (Golang `%w`).

// I18nFsBackend is a struct that holds the embedded filesystem and found languages.
type I18nFsBackend struct {
	langs []string
	FS    embed.FS
}

func (b *I18nFsBackend) GetLocales() ([]string, error) {
	// If translations are already loaded, return the list of languages.
	if b.langs != nil {
		return b.langs, nil
	}
	// Otherwise, read available translations from the filesystem.
	var entries []os.DirEntry
	var err error

	if devMode {
		entries, err = os.ReadDir("locales")
		if err != nil {
			return nil, fmt.Errorf("can't read locales from 'locales' directory: %w", err)
		}
	} else {
		entries, err = b.FS.ReadDir("locales")
		if err != nil {
			return nil, fmt.Errorf("can't read locales from embedded filesystem: %w", err)
		}
	}
	b.langs = make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			b.langs = append(b.langs, entry.Name())
		}
	}
	return b.langs, nil
}

// LoadTranslations loads translations for all languages.
func (b *I18nFsBackend) LoadTranslations(defaultLang string) (map[string]map[string]interface{}, error) {
	locales, err := b.GetLocales()
	if err != nil {
		return nil, err
	}
	// Check default language is in the list
	if !slices.Contains(locales, defaultLang) {
		return nil, fmt.Errorf("default language '%s' is not in the list of languages", defaultLang)
	}
	translations := make(map[string]map[string]interface{})
	for _, locale := range locales {
		filePath := "locales/" + locale + "/translation.json"
		var data []byte
		var err error
		if devMode {
			data, err = os.ReadFile(filePath)
		} else {
			data, err = b.FS.ReadFile(filePath)
		}
		if err != nil {
			return nil, err
		}
		var translation map[string]interface{}
		if err := json.Unmarshal(data, &translation); err != nil {
			return nil, err
		}
		translations[locale] = translation
	}
	return translations, nil
}

// I18n is a translator based on i18next.
type I18n struct {
	backend      I18nFsBackend
	locale       string
	translations map[string]map[string]interface{}
	funcs        map[string]func(entry interface{}, props map[string]interface{}) string
	devMode      bool
}

// Init initializes the translator instance with the backend and default locale.
func (i18n *I18n) Init(backend I18nFsBackend, defaultLocale string, devMode bool) error {
	i18n.backend = backend
	i18n.locale = defaultLocale
	i18n.devMode = devMode
	var err error
	i18n.translations, err = i18n.backend.LoadTranslations(defaultLocale)
	if err != nil {
		return fmt.Errorf("failed to load translations: %w", err)
	}
	if i18n.devMode {
		i18n.validateKeys()
	}
	i18n.funcs = i18n.buildDefaultFormatters()
	return nil
}

// RegisterFunc registers a custom formatter function.
func (i18n *I18n) RegisterFunc(name string, function func(entry interface{}, props map[string]interface{}) string) {
	i18n.funcs[name] = function
}

// buildDefaultFormatters builds default formatters.
func (i18n *I18n) buildDefaultFormatters() map[string]func(val interface{}, props map[string]interface{}) string {
	result := make(map[string]func(val interface{}, props map[string]interface{}) string)

	// object (Golang `%+v`).
	result["object"] = func(val interface{}, props map[string]interface{}) string {
		return fmt.Sprintf("%+v", val)
	}

	// values (Golang `%v`).
	result["values"] = func(val interface{}, props map[string]interface{}) string {
		return fmt.Sprintf("%+v", val)
	}

	// Number format - subset of https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/NumberFormat/NumberFormat
	result["number"] = i18n.number

	// Currency format - based on MoneyWith2DecimalPlaces.
	result["currency"] = func(val interface{}, props map[string]interface{}) string {
		var amount MoneyWith2DecimalPlaces
		switch v := val.(type) {
		case MoneyWith2DecimalPlaces:
			amount = v
		case float64:
			amount = MoneyWith2DecimalPlaces{int(v * 100)}
		default:
			return fmt.Sprintf("%+v", val)
		}
		if currency, ok := props["currency"]; ok {
			return fmt.Sprintf("%s %s", amount.StringNoIndent(), currency)
		}
		return amount.StringNoIndent()
	}

	// Date format, use I18N_DATE_FORMAT as default.
	result["date"] = func(val interface{}, props map[string]interface{}) string {
		if val, ok := val.(time.Time); ok {
			layout := I18N_DATE_FORMAT
			if fmt, ok := props["format"].(string); ok {
				layout = fmt
			}
			return val.Format(layout)
		}
		return fmt.Sprintf("%+v", val)
	}

	// List format, supports only "separator" property.
	result["list"] = func(value interface{}, props map[string]interface{}) string {
		var strSlice []string

		switch v := value.(type) {
		case []string:
			strSlice = v
		case []interface{}:
			strSlice = make([]string, len(v))
			for i, item := range v {
				strSlice[i] = fmt.Sprintf("%v", item)
			}
		default:
			// Check if it's any other kind of slice/array using reflection
			val := reflect.ValueOf(value)
			if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
				strSlice = make([]string, val.Len())
				for i := 0; i < val.Len(); i++ {
					strSlice[i] = fmt.Sprintf("%v", val.Index(i).Interface())
				}
			} else {
				// If it's not a slice/array at all, treat it as a single item
				return fmt.Sprintf("%v", value)
			}
		}

		if len(strSlice) == 0 {
			return ""
		}
		if len(strSlice) == 1 {
			return strSlice[0]
		}
		separator := ", "
		if sep, ok := props["separator"]; ok {
			separator = sep.(string)
		}
		return strings.Join(strSlice, separator)
	}

	// Indent format, supports "rightIndent" and "leftIndent" properties.
	result["indent"] = func(val interface{}, props map[string]interface{}) string {
		if indent, ok := props["rightIndent"]; ok {
			if intVal, ok := indent.(string); ok {
				indent, _ = strconv.Atoi(intVal)
			}
			return fmt.Sprintf("%-*s", indent, fmt.Sprintf("%v", val))
		}
		if indent, ok := props["leftIndent"]; ok {
			if intVal, ok := indent.(string); ok {
				indent, _ = strconv.Atoi(intVal)
			}
			return fmt.Sprintf("%*s", indent, fmt.Sprintf("%v", val))
		}
		return fmt.Sprintf("%+v", val)
	}

	// Error format - based on `%w`.
	result["error"] = func(val interface{}, props map[string]interface{}) string {
		return fmt.Sprintf("%v", val.(error))
	}
	return result
}

// SetLocale sets the locale for the translator.
func (i18n *I18n) SetLocale(locale string) error {
	if _, ok := i18n.translations[locale]; !ok {
		return fmt.Errorf("locale '%s' is not supported", locale)
	}
	i18n.locale = locale
	return nil
}

// validateKeys validates that all keys exists in all translations.
func (i18n *I18n) validateKeys() {
	keysInLocales := make(map[string][]string)
	locales := []string{}
	// Collect keys and locales.
	for locale, translations := range i18n.translations {
		for key := range translations {
			if _, ok := keysInLocales[key]; !ok {
				keysInLocales[key] = []string{}
			}
			keysInLocales[key] = append(keysInLocales[key], locale)
		}
		locales = append(locales, locale)
	}
	// Iterate all keys and check if they are present in all locales.
	for key, existInLocales := range keysInLocales {
		if len(existInLocales) != len(locales) {
			missedLocales := []string{}
			for _, locale := range locales {
				if !slices.Contains(existInLocales, locale) {
					missedLocales = append(missedLocales, locale)
				}
			}
			log.Printf("ERROR: key '%s' is missed in translations: '%s'", key, strings.Join(missedLocales, ", "))
		}
	}
}

// T translates a key with named arguments and fallback to the key and args if translation is not found.
// Named arguments are passed as `argKey, argValue` pairs after the translation key.
// Usage is: `T("using n option from lst for job", "n", len(options), "l", options, "job", "preparation")`
// where translation key is defined as `  "using n option from list for": "using {{n}} option from {{lst, list(separator: ",")}} for {{job}}`
// and 'list' formatter (aka function) has 'separator' argument.
// Notes:
// - Translation key is not allowed to contain ':', '.' characters.
// - Argument keys should be unique and match values in translation.
// - Per value options (like with "formatParams" in https://www.i18next.com/translation-function/formatting) are not supported, everything should be formatted in translation (use different keys).
func (i18n *I18n) T(key string, args ...interface{}) string {
	var entry interface{}
	var ok bool
	if entry, ok = i18n.translations[i18n.locale][key]; !ok {
		return i18n.Tfallback("missed key", key, args...)
	}

	// Parse args as key-value pairs.
	props := make(map[string]interface{})
	var argKey string
	for i, arg := range args {
		if i%2 == 0 {
			if argKey, ok = arg.(string); !ok {
				return i18n.Tfallback(fmt.Sprintf("wrong call - odd argument '%s' is not a string", arg), key, args...)
			}
		} else {
			props[argKey] = arg
		}
	}

	// Handle different types of translations
	switch v := entry.(type) {
	case string:
		// Parse format specifiers like {{val, number}} or {{val, currency(name: USD)}}
		result := v
		matches := formatSpecifierRegexp.FindAllStringSubmatch(result, -1)

		for _, match := range matches {
			placeholder := match[0]
			parts := strings.Split(strings.TrimSpace(match[1]), ",")
			if len(parts) < 2 {
				// Simple interpolation without formatting.
				interpolationKey := strings.TrimSpace(match[1])
				if val, exists := props[interpolationKey]; exists {
					result = strings.Replace(result, placeholder, fmt.Sprintf("%v", val), -1)
				} else {
					return i18n.Tfallback(fmt.Sprintf("'%s' value is missed", interpolationKey), key, args...)
				}
				continue
			}

			// Handle formatting.
			propKey := strings.TrimSpace(parts[0])
			formatterSpec := strings.TrimSpace(parts[1])

			// Parse formatter name and options
			var formatterName string
			formatterOptions := make(map[string]interface{})

			if idx := strings.Index(formatterSpec, "("); idx != -1 {
				// Has options like "list(separator: ',')"
				if !strings.HasSuffix(formatterSpec, ")") {
					return i18n.Tfallback(fmt.Sprintf("malformed formatter call '%s' - missing closing bracket", formatterSpec), key, args...)
				}
				formatterName = strings.TrimSpace(formatterSpec[:idx])
				optionsStr := strings.TrimSpace(formatterSpec[idx+1 : len(formatterSpec)-1])

				// Parse options
				if optionsStr != "" {
					optionPairs := strings.Split(optionsStr, ",")
					for _, pair := range optionPairs {
						kv := strings.Split(pair, ":")
						if len(kv) != 2 {
							return i18n.Tfallback(fmt.Sprintf("malformed option '%s' in '%s' formatter call", pair, formatterName), key, args...)
						}
						optKey := strings.TrimSpace(kv[0])
						optVal := strings.TrimSpace(kv[1])

						// Remove quotes if present
						if strings.HasPrefix(optVal, "'") || strings.HasPrefix(optVal, "\"") {
							optVal = optVal[1 : len(optVal)-1]
						}

						formatterOptions[optKey] = optVal
					}
				}
			} else {
				formatterName = formatterSpec
			}

			formatter := i18n.funcs[formatterName]
			if formatter != nil {
				if propValue, exists := props[propKey]; exists {
					// Merge formatter options with props
					for k, v := range formatterOptions {
						props[k] = v
					}
					formatted := formatter(propValue, props)
					result = strings.Replace(result, placeholder, formatted, -1)
				} else {
					return i18n.Tfallback(fmt.Sprintf("'%s' in translation misses '%s' value for formatting", parts[0], propKey), key, args...)
				}
			} else {
				return i18n.Tfallback(fmt.Sprintf("'%s' in translation misses '%s' formatter name", parts[0], formatterName), key, args...)
			}
		}
		return result

	case map[string]interface{}:
		// Handle nested objects with potential formatting functions
		if format, exists := v["format"]; exists {
			if formatFunc, ok := i18n.funcs[format.(string)]; ok {
				return formatFunc(v, props)
			}
			return i18n.Tfallback("unknown format function", key, args...)
		}
		return i18n.Tfallback("invalid translation format", key, args...)
	default:
		return i18n.Tfallback("invalid translation type", key, args...)
	}
}

func (i18n *I18n) Tfallback(reason, key string, args ...interface{}) string {
	// Fallback: if translation fails, build list of "%+v" of args.
	var argsList []string
	for _, arg := range args {
		argsList = append(argsList, fmt.Sprintf("%+v", arg))
	}
	if i18n.devMode {
		panic(fmt.Sprintf("[%s: %s] %s, %s", i18n.locale, reason, key, strings.Join(argsList, ", ")))
	}
	return fmt.Sprintf("[%s: %s] %s, %s", i18n.locale, reason, key, strings.Join(argsList, ", "))
}

// number formats a number based on the specified options.
func (i18n *I18n) number(value interface{}, props map[string]interface{}) string {
	var val MoneyWith2DecimalPlaces
	switch v := value.(type) {
	case MoneyWith2DecimalPlaces:
		val = v
	case float64:
		val = MoneyWith2DecimalPlaces{int(v * 100)}
	default:
		return fmt.Sprintf("%+v", value)
	}

	// Extract options from props
	options := make(map[string]interface{})
	for k, v := range props {
		options[k] = v
	}

	// Handle sign display
	signDisplay := "auto"
	if sd, ok := options["signDisplay"]; ok {
		signDisplay = sd.(string)
	}
	sign := ""
	switch signDisplay {
	case "always":
		if val.int >= 0 {
			sign = "+"
		} else {
			sign = "-"
		}
	case "exceptZero":
		if val.int > 0 {
			sign = "+"
		} else if val.int < 0 {
			sign = "-"
		}
	case "negative":
		if val.int < 0 && val.int != 0 {
			sign = "-"
		}
	case "never":
		// No sign
	default: // "auto"
		if val.int < 0 {
			sign = "-"
		}
	}
	if sign == "-" {
		val.int = -val.int
	}

	// Handle significant digits
	maxSigDigits := 21
	minSigDigits := 1
	if msd, ok := options["maximumSignificantDigits"]; ok {
		if intVal, ok := msd.(string); ok {
			maxSigDigits, _ = strconv.Atoi(intVal)
		} else {
			maxSigDigits = msd.(int)
		}
	}
	if msd, ok := options["minimumSignificantDigits"]; ok {
		if intVal, ok := msd.(string); ok {
			minSigDigits, _ = strconv.Atoi(intVal)
		} else {
			minSigDigits = msd.(int)
		}
	}

	// Handle fraction digits
	maxFracDigits := 3 // default for plain number formatting
	minFracDigits := 0
	if mfd, ok := options["maximumFractionDigits"]; ok {
		if intVal, ok := mfd.(string); ok {
			maxFracDigits, _ = strconv.Atoi(intVal)
		} else {
			maxFracDigits = mfd.(int)
		}
	}
	if mfd, ok := options["minimumFractionDigits"]; ok {
		if intVal, ok := mfd.(string); ok {
			minFracDigits, _ = strconv.Atoi(intVal)
		} else {
			minFracDigits = mfd.(int)
		}
	}

	// Handle minimum integer digits
	minIntDigits := 1
	if mid, ok := options["minimumIntegerDigits"]; ok {
		if intVal, ok := mid.(string); ok {
			minIntDigits, _ = strconv.Atoi(intVal)
		} else {
			minIntDigits = mid.(int)
		}
	}

	// Format the number based on significant digits or fraction digits
	var result string
	_, hasMaxSig := options["maximumSignificantDigits"]
	_, hasMinSig := options["minimumSignificantDigits"]
	if hasMaxSig || hasMinSig {
		// Use significant digits
		precision := maxSigDigits
		if precision > 21 {
			precision = 21
		}
		format := fmt.Sprintf("%%.%dg", precision)
		result = fmt.Sprintf(format, val)

		// Ensure minimum significant digits by padding with zeros
		parts := strings.Split(result, ".")
		intPart := parts[0]
		fracPart := ""
		if len(parts) > 1 {
			fracPart = parts[1]
		}

		totalDigits := len(strings.Replace(result, ".", "", 1))
		if totalDigits < minSigDigits {
			if len(parts) == 1 {
				fracPart = strings.Repeat("0", minSigDigits-totalDigits)
			} else {
				fracPart += strings.Repeat("0", minSigDigits-totalDigits)
			}
		}

		if fracPart != "" {
			result = intPart + "." + fracPart
		} else {
			result = intPart
		}
	} else {
		// Use fraction digits
		format := fmt.Sprintf("%%.%df", maxFracDigits)
		result = fmt.Sprintf(format, val)

		// Handle minimum fraction digits
		parts := strings.Split(result, ".")
		intPart := parts[0]
		fracPart := ""
		if len(parts) > 1 {
			fracPart = parts[1]
		}

		// Pad integer part with leading zeros if needed
		for len(intPart) < minIntDigits {
			intPart = "0" + intPart
		}

		// Pad fraction part with trailing zeros if needed
		for len(fracPart) < minFracDigits {
			fracPart += "0"
		}

		if fracPart != "" {
			result = intPart + "." + fracPart
		} else {
			result = intPart
		}
	}

	return sign + result
}
