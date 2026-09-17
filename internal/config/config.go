package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Provider string

const (
	ProviderGroq   Provider = "groq"
	ProviderOllama Provider = "ollama"
	ProviderMock   Provider = "mock"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	StaticDir            string
	LLMProvider          Provider
	GroqAPIKey           string
	GroqModel            string
	OllamaBaseURL        string
	OllamaModel          string
	GitHubWebhookSecret  string
	GitHubToken          string
	OpenAIInputUSDPer1M  float64
	OpenAIOutputUSDPer1M float64
	GuidelinesDir        string
}

func Load() Config {
	_ = godotenv.Load()
	_ = godotenv.Load(".env.example")

	root, _ := os.Getwd()
	guidelines := filepath.Join(root, "guidelines")
	if _, err := os.Stat(guidelines); err != nil {
		guidelines = filepath.Join(root, "..", "..", "guidelines")
	}

	return Config{
		Port:                 getenv("PORT", "8787"),
		DatabaseURL:          getenv("DATABASE_URL", "postgres://guardian:guardian@localhost:5432/code_guardian"),
		StaticDir:            getenv("STATIC_DIR", "web/dist"),
		LLMProvider:          Provider(getenv("LLM_PROVIDER", "mock")),
		GroqAPIKey:           os.Getenv("GROQ_API_KEY"),
		GroqModel:            getenv("GROQ_MODEL", "openai/gpt-oss-20b"),
		OllamaBaseURL:        getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:          getenv("OLLAMA_MODEL", "llama3.1"),
		GitHubWebhookSecret:  getenv("GITHUB_WEBHOOK_SECRET", "change-me"),
		GitHubToken:          os.Getenv("GITHUB_TOKEN"),
		OpenAIInputUSDPer1M:  getenvFloat("OPENAI_INPUT_USD_PER_1M", 0.15),
		OpenAIOutputUSDPer1M: getenvFloat("OPENAI_OUTPUT_USD_PER_1M", 0.60),
		GuidelinesDir:        guidelines,
	}
}

func (c Config) ResolveProvider() Provider {
	switch c.LLMProvider {
	case ProviderMock:
		return ProviderMock
	case ProviderOllama:
		return ProviderOllama
	case ProviderGroq:
		if c.GroqAPIKey != "" {
			return ProviderGroq
		}
		return ProviderMock
	default:
		if c.GroqAPIKey != "" {
			return ProviderGroq
		}
		return ProviderMock
	}
}

func (c Config) ModelName() string {
	switch c.ResolveProvider() {
	case ProviderGroq:
		return c.GroqModel
	case ProviderOllama:
		return c.OllamaModel
	default:
		return "fixture-reviewer"
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
