package main

import (
	"fmt"
	"os"

	"tg-rss/scraper"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run debug_compression.go <url>")
		fmt.Println("Example: go run debug_compression.go https://lenta.ru/news/...")
		os.Exit(1)
	}

	url := os.Args[1]
	fmt.Printf("Тестируем скреппер с URL: %s\n\n", url)

	// Сначала тестируем с включенным сжатием
	fmt.Println("=== ТЕСТ С ВКЛЮЧЕННЫМ СЖАТИЕМ ===")
	scraper.SetDisableCompression(false)

	content, err := scraper.ScrapeNewsContent(url)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	} else {
		fmt.Printf("Длина текста: %d символов\n", len(content.FullText))
		if len(content.FullText) > 0 {
			// Проверяем на бинарные символы
			binaryChars := 0
			for _, r := range content.FullText {
				if r < 32 && r != 9 && r != 10 && r != 13 {
					binaryChars++
				}
			}
			fmt.Printf("Бинарных символов: %d (%.1f%%)\n", binaryChars, float64(binaryChars)/float64(len(content.FullText))*100)

			if binaryChars > len(content.FullText)/10 {
				fmt.Println("⚠️  ОБНАРУЖЕНЫ СЖАТЫЕ ДАННЫЕ!")
				fmt.Printf("Пример: %q\n", content.FullText[:min(200, len(content.FullText))])
			} else {
				fmt.Println("✅ Данные выглядят нормально")
			}
		}
	}

	fmt.Println("\n=== ТЕСТ С ОТКЛЮЧЕННЫМ СЖАТИЕМ ===")
	scraper.SetDisableCompression(true)
	defer scraper.SetDisableCompression(false)

	content2, err2 := scraper.ScrapeNewsContent(url)
	if err2 != nil {
		fmt.Printf("Ошибка: %v\n", err2)
	} else {
		fmt.Printf("Длина текста: %d символов\n", len(content2.FullText))
		if len(content2.FullText) > 0 {
			binaryChars := 0
			for _, r := range content2.FullText {
				if r < 32 && r != 9 && r != 10 && r != 13 {
					binaryChars++
				}
			}
			fmt.Printf("Бинарных символов: %d (%.1f%%)\n", binaryChars, float64(binaryChars)/float64(len(content2.FullText))*100)

			if binaryChars > len(content2.FullText)/10 {
				fmt.Println("⚠️  ОБНАРУЖЕНЫ СЖАТЫЕ ДАННЫЕ!")
			} else {
				fmt.Println("✅ Данные выглядят нормально")
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
