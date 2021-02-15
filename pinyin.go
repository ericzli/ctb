package main

import (
	"regexp"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// 将wo3转成wǒ, 输入用v代替ü
func TranslatePinyin(in string) string {
	loopUpTable := map[rune][]rune{
		'a': []rune{'a', 'ā', 'á', 'ǎ', 'à'},
		'o': []rune{'o', 'ō', 'ó', 'ǒ', 'ò'},
		'e': []rune{'e', 'ē', 'é', 'ě', 'è'},
		'i': []rune{'i', 'ī', 'í', 'ǐ', 'ì'},
		'u': []rune{'u', 'ū', 'ú', 'ǔ', 'ù'},
		'ü': []rune{'ü', 'ǖ', 'ǘ', 'ǚ', 'ǜ'},
		'v': []rune{'ü', 'ǖ', 'ǘ', 'ǚ', 'ǜ'},
	}

	// 获取声调，同时提取为rune
	runes := []rune{}
	tone := 0
	for i := 0; i < len(in); {
		r, n := utf8.DecodeRuneInString(in[i:])
		if unicode.IsDigit(r) {
			tone, _ = strconv.Atoi(in[i : i+n])
			if tone > 4 {
				tone = 0
			}
			break
		}
		i += n
		runes = append(runes, r)
	}

	// 声调标在哪
	toneScore := func(c rune) int {
		switch c {
		case 'a':
			return 4
		case 'o':
			return 3
		case 'e':
			return 2
		case 'i', 'u', 'v', 'ü':
			return 1
		}
		return 0
	}
	toneIndex := -1
	toneMaxScore := 0
	for i, r := range runes {
		if toneScore(r) >= toneMaxScore {
			toneIndex = i
			toneMaxScore = toneScore(r)
		}
		// 字符替换，主要是将v替成ü
		if table, ok := loopUpTable[r]; ok {
			runes[i] = table[0]
		}
	}

	// 标上声调
	if toneIndex >= 0 {
		if table, ok := loopUpTable[runes[toneIndex]]; ok {
			runes[toneIndex] = table[tone]
		}
	}

	// 转成string
	buf := make([]byte, len(in)+100)
	i := 0
	for _, r := range runes {
		i += utf8.EncodeRune(buf[i:], r)
	}
	return string(buf[:i])
}

func ReplaceAllPinyin(in string) string {
	reg := regexp.MustCompile("[a-z]+[1-4]")
	indexArr := reg.FindAllStringIndex(in, -1)

	var out string
	lastE := 0
	for _, v := range indexArr {
		s, e := v[0], v[1]
		out += in[lastE:s] + TranslatePinyin(in[s:e])
		lastE = e
	}
	out += in[lastE:]
	return out
}
