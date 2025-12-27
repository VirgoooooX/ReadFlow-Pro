package worker

import (
	"strings"
	"unicode"
)

// GenerateSummary 从 HTML 生成摘要
// maxLength: 最大长度（字符数）
func GenerateSummary(htmlText string, maxLength int) string {
	// 清理 HTML 标签
	text := CleanHTMLTags(htmlText)
	
	if text == "" {
		return ""
	}
	
	// 如果文本本身就短，直接返回
	if len(text) <= maxLength {
		return text
	}
	
	// 截断到最大长度
	truncated := text[:maxLength]
	
	// 尝试在最后一个空格处截断（避免截断单词）
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 && lastSpace > maxLength/2 {
		// 只有当空格位置在后半部分时才使用，避免截得太短
		return truncated[:lastSpace] + "..."
	}
	
	return truncated + "..."
}

// CountWords 统计字数（中英文混合）
// 中文按字符数计算，英文按单词数计算
func CountWords(htmlText string) int {
	// 清理 HTML 标签
	plain := CleanHTMLTags(htmlText)
	
	if plain == "" {
		return 0
	}
	
	// 统计中文字符数
	chineseCount := 0
	for _, r := range plain {
		if isCJK(r) {
			chineseCount++
		}
	}
	
	// 将中文字符替换为空格，然后统计英文单词
	var builder strings.Builder
	for _, r := range plain {
		if isCJK(r) {
			builder.WriteRune(' ')
		} else {
			builder.WriteRune(r)
		}
	}
	
	// 分割并统计英文单词
	englishWords := strings.Fields(builder.String())
	englishCount := len(englishWords)
	
	return chineseCount + englishCount
}

// isCJK 判断是否为中日韩字符
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x4E00 && r <= 0x9FFF) || // CJK 统一表意文字
		(r >= 0x3400 && r <= 0x4DBF) || // CJK 扩展 A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK 扩展 B
		(r >= 0x2A700 && r <= 0x2B73F) || // CJK 扩展 C
		(r >= 0x2B740 && r <= 0x2B81F) || // CJK 扩展 D
		(r >= 0x2B820 && r <= 0x2CEAF) || // CJK 扩展 E
		(r >= 0xF900 && r <= 0xFAFF) || // CJK 兼容表意文字
		(r >= 0x2F800 && r <= 0x2FA1F) // CJK 兼容表意文字补充
}

// CalculateReadingTime 计算阅读时间（分钟）
// 假设阅读速度为 200 字/分钟
func CalculateReadingTime(wordCount int) int {
	if wordCount <= 0 {
		return 0
	}
	
	// 计算阅读时间，向上取整
	readingTime := (wordCount + 199) / 200
	
	return readingTime
}
