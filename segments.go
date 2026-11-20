package macrasms

import (
	"strings"
	"unicode/utf8"
)

const gsmBasic = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞ\x1bÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"
const gsmExtended = "^{}\\[~]|€"

// Segments returns the number of SMS parts (credits per recipient) for text,
// using the same rules MacraSMS charges by: 160 GSM characters in one SMS and
// 153 per part when longer (^ { } \ [ ~ ] | and the euro sign count as two),
// otherwise 70 characters in one SMS and 67 per part.
func Segments(text string) int {
	if text == "" {
		return 1
	}
	units, gsm := 0, true
	for _, r := range text {
		switch {
		case strings.ContainsRune(gsmExtended, r):
			units += 2
		case strings.ContainsRune(gsmBasic, r):
			units++
		default:
			gsm = false
		}
		if !gsm {
			break
		}
	}
	if gsm {
		if units <= 160 {
			return 1
		}
		return (units + 152) / 153
	}
	n := utf8.RuneCountInString(text)
	if n <= 70 {
		return 1
	}
	return (n + 66) / 67
}
