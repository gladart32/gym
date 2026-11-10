package exercises

import "strings"

const (
	LangRu = "ru"
	LangEn = "en"
)

// LangFromAcceptLanguage picks ru or en from an Accept-Language header value,
// ru being the app-wide fallback locale.
func LangFromAcceptLanguage(header string) string {
	tag := strings.ToLower(strings.TrimSpace(strings.Split(header, ",")[0]))
	switch {
	case tag == "":
		return LangRu
	case strings.HasPrefix(tag, LangEn):
		return LangEn
	default:
		return LangRu
	}
}

// nameColumn maps a whitelisted lang to the localized name column.
func nameColumn(lang string) string {
	if lang == LangEn {
		return "name_en"
	}
	return "name_ru"
}
