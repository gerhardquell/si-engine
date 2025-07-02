//**********************************************************************
//      sigoEngine.go
//**********************************************************************
//  Autor    : Gerhard Quell - gquell@skequell.de
//  CoAutor  : claude opus 4
//  Copyright: 2025 Gerhard Quell - SKEQuell
//  Erstellt : 20250630
//  Aenderung: 20250701
//**********************************************************************
// Beschreibung: Universelle SI/KI Engine - KISS Prinzip
//               Eine Datei, keine Dependencies, pure Power
//**********************************************************************

package main

import (
  "bufio"
  "bytes"
  "context"
  "encoding/json"
  "flag"
  "fmt"
  "io"
  "net/http"
  "os"
  "path/filepath"
  "strings"
  "sync"
  "time"
)

//**********************************************************************
// Basis Request/Response Strukturen
type Request struct {
  Model     string
  Prompt    string
  MaxTokens int
}

type Response struct {
  Model     string        `json:"model"`
  PID       int           `json:"pid"`
  Timestamp int64         `json:"timestamp"`
  Prompt    string        `json:"prompt,omitempty"`
  Response  string        `json:"response"`
  Error     string        `json:"error,omitempty"`
  Duration  time.Duration `json:"duration_ms"`
}

//**********************************************************************
// Session handling - minimal
type Session struct {
  History []Message `json:"history"`
}

type Message struct {
  Role    string `json:"role"`    // "user" oder "assistant"
  Content string `json:"content"`
}

//**********************************************************************
// Provider Config
type ProviderConfig struct {
  Endpoint  string            `json:"endpoint"`
  Model     string            `json:"model"`
  APIKey    string            `json:"api_key"`
  Headers   map[string]string `json:"headers,omitempty"`
  Type      string            `json:"type"` // "anthropic", "openai", "custom"
}

//**********************************************************************
// Circuit Breaker - einfach aber effektiv
type CircuitBreaker struct {
  failures  int
  lastFail  time.Time
  threshold int
  timeout   time.Duration
  mu        sync.Mutex
}

//**********************************************************************
func NewCircuitBreaker() *CircuitBreaker {
  return &CircuitBreaker{
    threshold: 3,
    timeout:   5 * time.Minute,
  }
}

//**********************************************************************
func (cb *CircuitBreaker) Do(fn func() error) error {
  cb.mu.Lock()
  defer cb.mu.Unlock()
  
  if time.Since(cb.lastFail) > cb.timeout {
    cb.failures = 0
  }
  
  if cb.failures >= cb.threshold {
    return fmt.Errorf("circuit open")
  }
  
  err := fn()
  if err != nil {
    cb.failures++
    cb.lastFail = time.Now()
  } else {
    cb.failures = 0
  }
  
  return err
}

//**********************************************************************
// Session functions
func loadSession(sessionID, model string) *Session {
  if sessionID == "" {
    return &Session{}
  }
  
  path := fmt.Sprintf(".sessions/%s-%s.json", model, sessionID)
  data, err := os.ReadFile(path)
  if err != nil {
    return &Session{}
  }
  
  var s Session
  json.Unmarshal(data, &s)
  return &s
}

//**********************************************************************
func (s *Session) save(sessionID, model string) {
  if sessionID == "" {
    return
  }
  
  os.MkdirAll(".sessions", 0755)
  path := fmt.Sprintf(".sessions/%s-%s.json", model, sessionID)
  
  data, _ := json.Marshal(s)
  os.WriteFile(path, data, 0644)
}

//**********************************************************************
func (s *Session) addMessage(role, content string) {
  s.History = append(s.History, Message{Role: role, Content: content})
  
  // Keep only last 20 messages
  if len(s.History) > 20 {
    s.History = s.History[len(s.History)-20:]
  }
}

//**********************************************************************
func (s *Session) buildPrompt(newPrompt string) string {
  if len(s.History) == 0 {
    return newPrompt
  }
  
  var ctx strings.Builder
  for _, msg := range s.History {
    if msg.Role == "user" {
      ctx.WriteString("Human: ")
    } else {
      ctx.WriteString("Assistant: ")
    }
    ctx.WriteString(msg.Content)
    ctx.WriteString("\n\n")
  }
  ctx.WriteString("Human: ")
  ctx.WriteString(newPrompt)
  
  return ctx.String()
}

//**********************************************************************
// Config loading
func loadConfig(model string) (*ProviderConfig, error) {
  path := fmt.Sprintf(".%s.config", model)
  data, err := os.ReadFile(path)
  if err != nil {
    return nil, fmt.Errorf("config not found: %s", path)
  }
  
  var cfg ProviderConfig
  if err := json.Unmarshal(data, &cfg); err != nil {
    return nil, fmt.Errorf("invalid config: %v", err)
  }
  
  // Expand env vars
  if strings.HasPrefix(cfg.APIKey, "${") && strings.HasSuffix(cfg.APIKey, "}") {
    envVar := strings.TrimSuffix(strings.TrimPrefix(cfg.APIKey, "${"), "}")
    cfg.APIKey = os.Getenv(envVar)
  }
  
  if cfg.APIKey == "" {
    return nil, fmt.Errorf("no API key")
  }
  
  return &cfg, nil
}

//**********************************************************************
// Generic API call
func callAPI(ctx context.Context, cfg *ProviderConfig, prompt string, maxTokens int) (string, error) {
  client := &http.Client{Timeout: 30 * time.Second}
  
  var reqBody map[string]interface{}
  
  switch cfg.Type {
  case "anthropic":
    reqBody = map[string]interface{}{
      "model": cfg.Model,
      "messages": []map[string]string{
        {"role": "user", "content": prompt},
      },
      "max_tokens": maxTokens,
    }
    
  case "openai", "":
    reqBody = map[string]interface{}{
      "model": cfg.Model,
      "messages": []map[string]string{
        {"role": "user", "content": prompt},
      },
      "max_tokens": maxTokens,
    }
    
  default:
    return "", fmt.Errorf("unknown provider type: %s", cfg.Type)
  }
  
  jsonData, _ := json.Marshal(reqBody)
  req, err := http.NewRequestWithContext(ctx, "POST", cfg.Endpoint, bytes.NewBuffer(jsonData))
  if err != nil {
    return "", err
  }
  
  // Headers
  req.Header.Set("Content-Type", "application/json")
  
  if cfg.Type == "anthropic" {
    req.Header.Set("x-api-key", cfg.APIKey)
    req.Header.Set("anthropic-version", "2023-06-01")
  } else {
    req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
  }
  
  for k, v := range cfg.Headers {
    req.Header.Set(k, v)
  }
  
  // Execute
  resp, err := client.Do(req)
  if err != nil {
    return "", err
  }
  defer resp.Body.Close()
  
  body, _ := io.ReadAll(resp.Body)
  
  var result map[string]interface{}
  if err := json.Unmarshal(body, &result); err != nil {
    return "", fmt.Errorf("parse error: %v", err)
  }
  
  // Extract response
  if cfg.Type == "anthropic" {
    if errMsg, ok := result["error"].(map[string]interface{}); ok {
      return "", fmt.Errorf("%v", errMsg["message"])
    }
    if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
      if text, ok := content[0].(map[string]interface{})["text"].(string); ok {
        return text, nil
      }
    }
  } else {
    if errMsg, ok := result["error"].(map[string]interface{}); ok {
      return "", fmt.Errorf("%v", errMsg["message"])
    }
    if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
      if msg, ok := choices[0].(map[string]interface{})["message"].(map[string]interface{}); ok {
        if content, ok := msg["content"].(string); ok {
          return content, nil
        }
      }
    }
  }
  
  return "", fmt.Errorf("unexpected response format")
}

//**********************************************************************
// Log functions
func logError(format string, args ...interface{}) {
  fmt.Fprintf(os.Stderr, "ERR %d: ", os.Getpid())
  fmt.Fprintf(os.Stderr, format+"\n", args...)
}

//**********************************************************************
// Help with sessions and models
func showHelp() {
  fmt.Println("SIGO - SI Gateway in GO\n")
  fmt.Println("Usage: sigo [options] [prompt]")
  fmt.Println("       echo 'prompt' | sigo [options]")
  fmt.Println("       sigo [options] < file.txt\n")
  
  flag.PrintDefaults()
  
  // List models
  fmt.Println("\nAvailable models:")
  configs, _ := filepath.Glob(".*.config")
  if len(configs) == 0 {
    fmt.Println("  (none configured)")
  }
  for _, cfg := range configs {
    name := strings.TrimPrefix(cfg, ".")
    name = strings.TrimSuffix(name, ".config")
    fmt.Printf("  %s\n", name)
  }
  
  // List sessions
  fmt.Println("\nActive sessions:")
  sessions, _ := filepath.Glob(".sessions/*.json")
  if len(sessions) == 0 {
    fmt.Println("  (none)")
  }
  for _, s := range sessions {
    base := filepath.Base(s)
    parts := strings.Split(strings.TrimSuffix(base, ".json"), "-")
    if len(parts) >= 2 {
      fmt.Printf("  -s %s (model: %s)\n", 
        strings.Join(parts[1:], "-"), parts[0])
    }
  }
  
  fmt.Println("\nExamples:")
  fmt.Println("  sigo 'Hello Claude'")
  fmt.Println("  sigo -m gpt4 -s project 'Continue our discussion'")
  fmt.Println("  echo 'Explain quantum physics' | sigo -n 2000")
}

//**********************************************************************
func getInput() (string, error) {
  // Args first
  if flag.NArg() > 0 {
    return strings.Join(flag.Args(), " "), nil
  }
  
  // Then stdin
  stat, _ := os.Stdin.Stat()
  if (stat.Mode() & os.ModeCharDevice) == 0 {
    data, err := io.ReadAll(os.Stdin)
    return strings.TrimSpace(string(data)), err
  }
  
  // Interactive
  reader := bufio.NewReader(os.Stdin)
  fmt.Fprint(os.Stderr, "Prompt: ")
  input, err := reader.ReadString('\n')
  return strings.TrimSpace(input), err
}

//**********************************************************************
func main() {
  // Flags
  var (
    model     = flag.String("m",     "claude4", "Model to use")
    sessionID = flag.String("s", "", "Session ID")
    maxTokens = flag.Int("n", 1024,  "Max tokens")
    timeout   = flag.Int("t", 30,    "Timeout seconds")
    retries   = flag.Int("r", 3,     "Retry count")
    quiet     = flag.Bool("q", false, "Quiet mode")
    jsonOut   = flag.Bool("j", false, "JSON output")
    help      = flag.Bool("h", false, "Show help")
  )
  
  flag.Parse()
  
  // Help
  if *help || (flag.NArg() == 0 && flag.NFlag() == 0) {
    showHelp()
    os.Exit(0)
  }
  
  // Load config
  cfg, err := loadConfig(*model)
  if err != nil {
    logError("Config: %v", err)
    os.Exit(1)
  }
  
  // Get input
  prompt, err := getInput()
  if err != nil || prompt == "" {
    logError("No input")
    os.Exit(1)
  }
  
  // Session
  session := loadSession(*sessionID, *model)
  contextPrompt := session.buildPrompt(prompt)
  
  // Circuit breaker
  breaker := NewCircuitBreaker()
  
  // Execute with retries
  var resp Response
  resp.Model = *model
  resp.PID = os.Getpid()
  resp.Timestamp = time.Now().Unix()
  resp.Prompt = prompt
  
  start := time.Now()
  
  ctx, cancel := context.WithTimeout(context.Background(), 
    time.Duration(*timeout)*time.Second)
  defer cancel()
  
  var lastErr error
  for i := 0; i < *retries; i++ {
    err := breaker.Do(func() error {
      result, err := callAPI(ctx, cfg, contextPrompt, *maxTokens)
      if err != nil {
        return err
      }
      resp.Response = result
      return nil
    })
    
    if err == nil {
      break
    }
    lastErr = err
    
    if i < *retries-1 {
      time.Sleep(time.Duration(i+1) * time.Second)
    }
  }
  
  if lastErr != nil {
    resp.Error = lastErr.Error()
  }
  
  resp.Duration = time.Since(start) / time.Millisecond
  
  // Save session
  if resp.Error == "" && *sessionID != "" {
    session.addMessage("user", prompt)
    session.addMessage("assistant", resp.Response)
    session.save(*sessionID, *model)
  }
  
  // Output
  if *jsonOut {
    json.NewEncoder(os.Stdout).Encode(resp)
  } else {
    if resp.Error != "" {
      if !*quiet {
        logError("%s", resp.Error)
      }
      os.Exit(1)
    }
    fmt.Println(resp.Response)
  }
  
  if resp.Error != "" {
    os.Exit(1)
  }
}
