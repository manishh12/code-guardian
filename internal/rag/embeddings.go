package rag

import (
        "hash/fnv"
        "math"
        "strconv"
        "strings"
        "unicode"
)

const Dims = 384

// EmbedText builds a free, local bag-of-words hashing embedding.
// Good enough for small guideline sets without paid embedding APIs.
func EmbedText(text string) []float32 {
        vec := make([]float32, Dims)
        tokens := tokenize(text)
        for _, token := range tokens {
                h := fnv.New32a()
                _, _ = h.Write([]byte(token))
                sum := h.Sum32()
                idx := int(sum % uint32(Dims))
                if sum&1 == 1 {
                        vec[idx] -= 1
                } else {
                        vec[idx] += 1
                }
        }
        var norm float64
        for _, v := range vec {
                norm += float64(v) * float64(v)
        }
        if norm == 0 {
                return vec
        }
        inv := float32(1 / math.Sqrt(norm))
        for i := range vec {
                vec[i] *= inv
        }
        return vec
}

func tokenize(text string) []string {
        text = strings.ToLower(text)
        var b strings.Builder
        var out []string
        flush := func() {
                if b.Len() == 0 {
                        return
                }
                tok := b.String()
                b.Reset()
                if len(tok) >= 2 {
                        out = append(out, tok)
                }
        }
        for _, r := range text {
                if unicode.IsLetter(r) || unicode.IsDigit(r) {
                        b.WriteRune(r)
                } else {
                        flush()
                }
        }
        flush()
        return out
}

func VectorLiteral(values []float32) string {
        parts := make([]string, len(values))
        for i, v := range values {
                parts[i] = strconv.FormatFloat(float64(v), 'f', 6, 32)
        }
        return "[" + strings.Join(parts, ",") + "]"
}
