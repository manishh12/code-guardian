package llm

import (
        "bytes"
        "context"
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "strings"
        "time"

        "github.com/code-guardian/code-guardian/internal/config"
        "github.com/code-guardian/code-guardian/internal/domain"
)

type ChatResult struct {
        Text     string
        Metric   domain.LlmCallMetric
        Provider string
        Model    string
}

type Client struct {
        cfg    config.Config
        http   *http.Client
}

func New(cfg config.Config) *Client {
        return &Client{
                cfg:  cfg,
                http: &http.Client{Timeout: 90 * time.Second},
        }
}

func (c *Client) Info() (provider, model string) {
        return string(c.cfg.ResolveProvider()), c.cfg.ModelName()
}

func (c *Client) Run(ctx context.Context, agent domain.AgentName, system, user string) (ChatResult, error) {
        provider := c.cfg.ResolveProvider()
        model := c.cfg.ModelName()
        started := time.Now()

        var text string
        var err error
        switch provider {
        case config.ProviderGroq:
                text, err = c.chatOpenAICompat(ctx, "https://api.groq.com/openai/v1/chat/completions", c.cfg.GroqAPIKey, model, system, user)
        case config.ProviderOllama:
                text, err = c.chatOllama(ctx, model, system, user)
        default:
                text = mockResponse(agent, user)
        }
        if err != nil {
                return ChatResult{}, err
        }

        return ChatResult{
                Text:     text,
                Provider: string(provider),
                Model:    model,
                Metric: domain.LlmCallMetric{
                        Agent:     agent,
                        TokensIn:  estimateTokens(system + user),
                        TokensOut: estimateTokens(text),
                        LatencyMs: int(time.Since(started).Milliseconds()),
                },
        }, nil
}

func estimateTokens(text string) int {
        n := (len(text) + 3) / 4
        if n < 1 {
                return 1
        }
        return n
}

func mockResponse(agent domain.AgentName, user string) string {
        _ = user
        switch agent {
        case domain.AgentEditor:
                return strings.Join([]string{
                        "## Code Guardian review",
                        "",
                        "### Summary",
                        "Demo findings from the mock Go provider.",
                        "",
                        "### Architecture",
                        "- **medium**: webhook handlers should acknowledge quickly and process asynchronously.",
                        "",
                        "### Security",
                        "- **high**: unsanitized command execution and hardcoded secrets must not ship.",
                        "",
                        "### Guideline alignment",
                        "Matches security.md and architecture.md from RAG.",
                        "",
                        "_Mock provider — set GROQ_API_KEY or Ollama for live reviews._",
                }, "\n")
        case domain.AgentSecurity:
                return strings.Join([]string{
                        "- High: `exec` of user query enables RCE.",
                        "- High: hardcoded password in source.",
                        "- Medium: missing auth on `/run`.",
                }, "\n")
        default:
                return strings.Join([]string{
                        "- Missing structured error handling around external I/O.",
                        "- Prefer explicit request validation before side effects.",
                        "- Keep review comments aggregated, not per-agent.",
                }, "\n")
        }
}

type openAIReq struct {
        Model    string        `json:"model"`
        Messages []openAIMsg   `json:"messages"`
        Temperature float64    `json:"temperature"`
}

type openAIMsg struct {
        Role    string `json:"role"`
        Content string `json:"content"`
}

type openAIResp struct {
        Choices []struct {
                Message struct {
                        Content string `json:"content"`
                } `json:"message"`
        } `json:"choices"`
}

func (c *Client) chatOpenAICompat(ctx context.Context, url, apiKey, model, system, user string) (string, error) {
        body, _ := json.Marshal(openAIReq{
                Model: model,
                Temperature: 0,
                Messages: []openAIMsg{
                        {Role: "system", Content: system},
                        {Role: "user", Content: user},
                },
        })
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
        if err != nil {
                return "", err
        }
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", "Bearer "+apiKey)
        res, err := c.http.Do(req)
        if err != nil {
                return "", err
        }
        defer res.Body.Close()
        raw, _ := io.ReadAll(res.Body)
        if res.StatusCode >= 300 {
                return "", fmt.Errorf("llm %s: %s", res.Status, string(raw))
        }
        var parsed openAIResp
        if err := json.Unmarshal(raw, &parsed); err != nil {
                return "", err
        }
        if len(parsed.Choices) == 0 {
                return "", fmt.Errorf("llm returned no choices")
        }
        return parsed.Choices[0].Message.Content, nil
}

type ollamaReq struct {
        Model    string `json:"model"`
        Stream   bool   `json:"stream"`
        Messages []openAIMsg `json:"messages"`
        Options  map[string]any `json:"options"`
}

type ollamaResp struct {
        Message struct {
                Content string `json:"content"`
        } `json:"message"`
}

func (c *Client) chatOllama(ctx context.Context, model, system, user string) (string, error) {
        body, _ := json.Marshal(ollamaReq{
                Model:  model,
                Stream: false,
                Messages: []openAIMsg{
                        {Role: "system", Content: system},
                        {Role: "user", Content: user},
                },
                Options: map[string]any{"temperature": 0},
        })
        url := strings.TrimRight(c.cfg.OllamaBaseURL, "/") + "/api/chat"
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
        if err != nil {
                return "", err
        }
        req.Header.Set("Content-Type", "application/json")
        res, err := c.http.Do(req)
        if err != nil {
                return "", err
        }
        defer res.Body.Close()
        raw, _ := io.ReadAll(res.Body)
        if res.StatusCode >= 300 {
                return "", fmt.Errorf("ollama %s: %s", res.Status, string(raw))
        }
        var parsed ollamaResp
        if err := json.Unmarshal(raw, &parsed); err != nil {
                return "", err
        }
        return parsed.Message.Content, nil
}
