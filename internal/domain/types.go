package domain

type AgentName string

const (
        AgentLinter   AgentName = "linter"
        AgentSecurity AgentName = "security"
        AgentEditor   AgentName = "editor"
)

type LlmCallMetric struct {
        Agent     AgentName `json:"agent"`
        TokensIn  int       `json:"tokensIn"`
        TokensOut int       `json:"tokensOut"`
        LatencyMs int       `json:"latencyMs"`
}

type PullRequestContext struct {
        Owner      string   `json:"owner"`
        Repo       string   `json:"repo"`
        PRNumber   int      `json:"prNumber"`
        Title      string   `json:"title"`
        Diff       string   `json:"diff"`
        Files      []string `json:"files"`
        DeliveryID string   `json:"deliveryId,omitempty"`
}

type ReviewResult struct {
        CommentMarkdown string          `json:"commentMarkdown"`
        Metrics         []LlmCallMetric `json:"metrics"`
        Provider        string          `json:"provider"`
        Model           string          `json:"model"`
}
