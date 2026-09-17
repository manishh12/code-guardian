package githubapp

import (
        "context"
        "crypto/hmac"
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "strings"

        "github.com/code-guardian/code-guardian/internal/domain"
)

type Client struct {
        token  string
        http   *http.Client
}

func NewClient(token string) *Client {
        if token == "" {
                return nil
        }
        return &Client{token: token, http: http.DefaultClient}
}

func VerifySignature(secret string, body []byte, signatureHeader string) bool {
        if secret == "" || signatureHeader == "" {
                return false
        }
        mac := hmac.New(sha256.New, []byte(secret))
        mac.Write(body)
        expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
        return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

func DemoPullRequest() domain.PullRequestContext {
        return domain.PullRequestContext{
                Owner:    "local",
                Repo:     "demo",
                PRNumber: 1,
                Title:    "Add webhook handler without auth",
                Files:    []string{"cmd/server/main.go"},
                Diff: `## cmd/server/main.go
@@ -1,12 +1,18 @@
 package main
+import "os/exec"
 func main() {
-  // health only
+  http.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
+    cmd := r.URL.Query().Get("cmd")
+    out, _ := exec.Command("cmd", "/C", cmd).CombinedOutput()
+    w.Write(out)
+  })
+  password := "super-secret-password"
+  _ = password
 }
`,
        }
}

func (c *Client) LoadPullRequest(ctx context.Context, owner, repo string, prNumber int) (domain.PullRequestContext, error) {
        prURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)
        var pr struct {
                Title string `json:"title"`
        }
        if err := c.getJSON(ctx, prURL, &pr); err != nil {
                return domain.PullRequestContext{}, err
        }

        filesURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/files?per_page=100", owner, repo, prNumber)
        var files []struct {
                Filename string `json:"filename"`
                Patch    string `json:"patch"`
        }
        if err := c.getJSON(ctx, filesURL, &files); err != nil {
                return domain.PullRequestContext{}, err
        }

        names := make([]string, 0, len(files))
        var diff strings.Builder
        for i, f := range files {
                if i >= 40 {
                        break
                }
                names = append(names, f.Filename)
                patch := f.Patch
                if patch == "" {
                        patch = "(binary or too large)"
                }
                diff.WriteString("## ")
                diff.WriteString(f.Filename)
                diff.WriteString("\n")
                diff.WriteString(patch)
                diff.WriteString("\n\n")
        }

        return domain.PullRequestContext{
                Owner:    owner,
                Repo:     repo,
                PRNumber: prNumber,
                Title:    pr.Title,
                Files:    names,
                Diff:     diff.String(),
        }, nil
}

func (c *Client) PostComment(ctx context.Context, pr domain.PullRequestContext, body string) error {
        url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", pr.Owner, pr.Repo, pr.PRNumber)
        payload, _ := json.Marshal(map[string]string{"body": body})
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
        if err != nil {
                return err
        }
        req.Header.Set("Authorization", "Bearer "+c.token)
        req.Header.Set("Accept", "application/vnd.github+json")
        req.Header.Set("Content-Type", "application/json")
        res, err := c.http.Do(req)
        if err != nil {
                return err
        }
        defer res.Body.Close()
        if res.StatusCode >= 300 {
                raw, _ := io.ReadAll(res.Body)
                return fmt.Errorf("github comment %s: %s", res.Status, string(raw))
        }
        return nil
}

func (c *Client) getJSON(ctx context.Context, url string, dest any) error {
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
        if err != nil {
                return err
        }
        req.Header.Set("Authorization", "Bearer "+c.token)
        req.Header.Set("Accept", "application/vnd.github+json")
        res, err := c.http.Do(req)
        if err != nil {
                return err
        }
        defer res.Body.Close()
        raw, _ := io.ReadAll(res.Body)
        if res.StatusCode >= 300 {
                return fmt.Errorf("github %s: %s", res.Status, string(raw))
        }
        return json.Unmarshal(raw, dest)
}
