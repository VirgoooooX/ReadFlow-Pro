package utils

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// TextProcessor 文本处理器
type TextProcessor struct{}

// NewTextProcessor 创建文本处理器
func NewTextProcessor() *TextProcessor {
	return &TextProcessor{}
}

// CountWords 统计字数（支持中英文混合）
// 中文按字符数，英文按单词数
func (p *TextProcessor) CountWords(htmlText string) int {
	// 1. 去除HTML标签
	plainText := p.StripHTML(htmlText)

	// 2. 统计中文字符数
	chineseCount := 0
	for _, r := range plainText {
		if unicode.Is(unicode.Han, r) {
			chineseCount++
		}
	}

	// 3. 统计英文单词数
	// 移除中文字符
	textWithoutChinese := regexp.MustCompile(`[\p{Han}]+`).ReplaceAllString(plainText, " ")

	// 分割单词
	words := strings.Fields(textWithoutChinese)
	englishWordCount := 0
	for _, word := range words {
		// 只统计包含字母的单词
		if regexp.MustCompile(`[a-zA-Z]`).MatchString(word) {
			englishWordCount++
		}
	}

	// 4. 总字数 = 中文字符数 + 英文单词数
	totalWords := chineseCount + englishWordCount

	return totalWords
}

// EstimateReadingTime 估算阅读时间（分钟）
// 基于字数和平均阅读速度
func (p *TextProcessor) EstimateReadingTime(wordCount int) int {
	// 平均阅读速度：
	// - 中文：300-400字/分钟
	// - 英文：200-250词/分钟
	// 使用混合速度：250 words/min
	const wordsPerMinute = 250

	if wordCount == 0 {
		return 0
	}

	minutes := (wordCount + wordsPerMinute - 1) / wordsPerMinute // 向上取整
	if minutes < 1 {
		return 1
	}

	return minutes
}

// GenerateSummary 从HTML生成摘要
func (p *TextProcessor) GenerateSummary(htmlText string, maxLength int) string {
	// 1. 去除HTML标签
	plainText := p.StripHTML(htmlText)

	// 2. 清理多余空白
	plainText = p.cleanWhitespace(plainText)

	// 3. 截取指定长度
	if maxLength <= 0 {
		maxLength = 200 // 默认200字符
	}

	// 使用rune计数（支持多字节字符）
	runes := []rune(plainText)
	if len(runes) <= maxLength {
		return plainText
	}

	// 截取并添加省略号
	summary := string(runes[:maxLength])

	// 尝试在句子或单词边界截断
	if lastPeriod := strings.LastIndexAny(summary, ".。!！?？"); lastPeriod > maxLength/2 {
		summary = summary[:lastPeriod+1]
	} else if lastSpace := strings.LastIndex(summary, " "); lastSpace > maxLength/2 {
		summary = summary[:lastSpace] + "..."
	} else {
		summary += "..."
	}

	return summary
}

// StripHTML 去除HTML标签
func (p *TextProcessor) StripHTML(htmlText string) string {
	if htmlText == "" {
		return ""
	}

	// 解析HTML
	doc, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		// 如果解析失败，使用正则表达式fallback
		return p.stripHTMLRegex(htmlText)
	}

	// 提取纯文本
	var text strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return text.String()
}

// stripHTMLRegex 使用正则表达式去除HTML标签（fallback方法）
func (p *TextProcessor) stripHTMLRegex(htmlText string) string {
	// 移除HTML标签
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(htmlText, " ")

	// 解码HTML实体
	text = p.decodeHTMLEntities(text)

	return text
}

// decodeHTMLEntities 解码HTML实体
func (p *TextProcessor) decodeHTMLEntities(text string) string {
	replacements := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&#39;":    "'",
		"&apos;":   "'",
		"&nbsp;":   " ",
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&hellip;": "...",
		"&ldquo;":  "\"",
		"&rdquo;":  "\"",
		"&lsquo;":  "'",
		"&rsquo;":  "'",
	}

	result := text
	for entity, replacement := range replacements {
		result = strings.ReplaceAll(result, entity, replacement)
	}

	return result
}

// cleanWhitespace 清理多余的空白字符
func (p *TextProcessor) cleanWhitespace(text string) string {
	// 1. 替换所有空白字符为单个空格
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	// 2. 去除首尾空白
	text = strings.TrimSpace(text)

	return text
}

// CalculateDifficulty 计算文章难度
// 基于词汇复杂度、句子长度等
func (p *TextProcessor) CalculateDifficulty(htmlText string) string {
	wordCount := p.CountWords(htmlText)

	if wordCount == 0 {
		return "easy"
	}

	// 简化版难度评估
	// 1. 基于文章长度
	if wordCount < 300 {
		return "easy"
	} else if wordCount < 1000 {
		return "medium"
	} else {
		return "hard"
	}

	// 更复杂的实现可以考虑：
	// - 平均句子长度
	// - 词汇复杂度（长单词比例）
	// - 专业术语密度
}

// ExtractFirstParagraph 提取第一段文本
func (p *TextProcessor) ExtractFirstParagraph(htmlText string) string {
	plainText := p.StripHTML(htmlText)
	plainText = p.cleanWhitespace(plainText)

	// 查找第一个句号或换行
	if idx := strings.IndexAny(plainText, ".\n"); idx > 0 && idx < 500 {
		return plainText[:idx+1]
	}

	// 如果没有找到，返回前200字符
	runes := []rune(plainText)
	if len(runes) > 200 {
		return string(runes[:200]) + "..."
	}

	return plainText
}

// TruncateText 截断文本到指定长度（按UTF-8字符计数）
func (p *TextProcessor) TruncateText(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}

	return string(runes[:maxLen]) + "..."
}

// ContainsChinese 检查文本是否包含中文
func (p *TextProcessor) ContainsChinese(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// IsValidUTF8 检查文本是否为有效的UTF-8
func (p *TextProcessor) IsValidUTF8(text string) bool {
	return utf8.ValidString(text)
}

// NormalizeWhitespace 规范化空白字符
// 保留段落结构，但清理多余空白
func (p *TextProcessor) NormalizeWhitespace(text string) string {
	// 1. 统一换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// 2. 多个连续换行替换为两个换行（段落分隔）
	re := regexp.MustCompile(`\n{3,}`)
	text = re.ReplaceAllString(text, "\n\n")

	// 3. 行内多余空格替换为单个空格
	re = regexp.MustCompile(`[^\S\n]+`)
	text = re.ReplaceAllString(text, " ")

	// 4. 去除行首行尾空格
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	return text
}

// ExtractKeywords 提取关键词（简单版本）
func (p *TextProcessor) ExtractKeywords(htmlText string, maxKeywords int) []string {
	plainText := p.StripHTML(htmlText)
	plainText = strings.ToLower(plainText)

	// 分词
	words := strings.Fields(plainText)

	// 统计词频（忽略停用词）
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "as": true, "by": true, "is": true,
		"was": true, "are": true, "were": true, "be": true, "been": true,
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
	}

	wordFreq := make(map[string]int)
	for _, word := range words {
		// 过滤短词和停用词
		if len(word) < 3 || stopWords[word] {
			continue
		}
		wordFreq[word]++
	}

	// 排序（简化：返回频率最高的词）
	type wordCount struct {
		word  string
		count int
	}
	var counts []wordCount
	for word, count := range wordFreq {
		counts = append(counts, wordCount{word, count})
	}

	// 简单排序
	for i := 0; i < len(counts); i++ {
		for j := i + 1; j < len(counts); j++ {
			if counts[j].count > counts[i].count {
				counts[i], counts[j] = counts[j], counts[i]
			}
		}
	}

	// 返回前N个关键词
	keywords := []string{}
	for i := 0; i < maxKeywords && i < len(counts); i++ {
		keywords = append(keywords, counts[i].word)
	}

	return keywords
}
