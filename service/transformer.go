package service

import (
	"html"
	"regexp"
	"strings"
)

var (
	nbspRegex                = regexp.MustCompile(`&nbsp;`)
	pullTagRegex             = regexp.MustCompile(`(?s)<pull-quote.*?</pull-quote>`)
	webPullTagRegex          = regexp.MustCompile(`(?s)<web-pull-quote.*?</web-pull-quote>`)
	tableTagRegex            = regexp.MustCompile(`(?s)<table.*?</table>`)
	promoBoxTagRegex         = regexp.MustCompile(`(?s)<promo-box.*?</promo-box>`)
	webInlinePictureTagRegex = regexp.MustCompile(`(?s)<web-inline-picture.*?</web-inline-picture>`)
	tagRegex                 = regexp.MustCompile(`<[^>]*>`)
	duplicateWhiteSpaceRegex = regexp.MustCompile(`\s+`)
)

type TextTransformer func(string) string

func TransformText(text string, transformers ...TextTransformer) string {
	current := text
	for _, transformer := range transformers {
		current = transformer(current)
	}
	return current
}

func PullTagTransformer(input string) string {
	return pullTagRegex.ReplaceAllString(input, "")
}

func WebPullTagTransformer(input string) string {
	return webPullTagRegex.ReplaceAllString(input, "")
}

func TableTagTransformer(input string) string {
	return tableTagRegex.ReplaceAllString(input, "")
}

func PromoBoxTagTransformer(input string) string {
	return promoBoxTagRegex.ReplaceAllString(input, "")
}

func WebInlinePictureTagTransformer(input string) string {
	return webInlinePictureTagRegex.ReplaceAllString(input, "")
}

func HtmlEntityTransformer(input string) string {
	text := nbspRegex.ReplaceAllString(input, " ")
	return html.UnescapeString(text)
}

func TagsRemover(input string) string {
	return tagRegex.ReplaceAllString(input, "")
}

func OuterSpaceTrimmer(input string) string {
	return strings.TrimSpace(input)
}

func DuplicateWhiteSpaceRemover(input string) string {
	return duplicateWhiteSpaceRegex.ReplaceAllString(input, " ")
}

func DefaultValueTransformer(input string) string {
	if input == "" {
		return "."
	}
	return input
}
