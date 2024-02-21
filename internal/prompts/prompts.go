package prompts

import (
	"fmt"
	"os"
)

const (
	AugurPrompt = ``
)

// Keeping the actual prompts hidden from you 🪄
func GetPrompt(prompt string) string {
	if promptFile := os.Getenv(prompt); promptFile != "" {
		if content, err := os.ReadFile(promptFile); err == nil {
			return string(content)
		}
	}
	fmt.Println("Using default prompt")
	return AugurPrompt
}
