package policy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go-safe-agent-gateway/internal/model"
	"go-safe-agent-gateway/internal/repository"
	"go-safe-agent-gateway/internal/tool"
)

type PolicyEngine interface {
	Check(ctx context.Context, req *PolicyRequest) (*PolicyDecision, error)
}

type PolicyRequest struct {
	UserID     string
	SessionID  string
	ToolName   string
	Permission tool.PermissionLevel
	Input      map[string]any
}

type PolicyDecision struct {
	Allowed        bool
	Reason         string
	SanitizedInput map[string]any
	MaskFields     []string
}

type Config struct {
	AllowedTools        []string
	URLAllowlist        []string
	RateLimiter         repository.RedisClient
	RateLimitPerMinute  int
	RateLimitFailClosed bool
	RequireSQLLimit     bool
	BlockedSQLTables    []string
}

type Engine struct {
	allowedTools        map[string]struct{}
	urlAllowlist        map[string]struct{}
	rateLimiter         repository.RedisClient
	rateLimitPerMinute  int
	rateLimitFailClosed bool
	requireSQLLimit     bool
	blockedSQLTables    []string
}

var dangerousSQL = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|create|replace|grant|revoke|call|exec)\b`)

func NewEngine(cfg Config) *Engine {
	allowed := make(map[string]struct{}, len(cfg.AllowedTools))
	for _, name := range cfg.AllowedTools {
		if name = strings.TrimSpace(name); name != "" {
			allowed[name] = struct{}{}
		}
	}
	urls := make(map[string]struct{}, len(cfg.URLAllowlist))
	for _, host := range cfg.URLAllowlist {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			urls[host] = struct{}{}
		}
	}
	limit := cfg.RateLimitPerMinute
	if limit <= 0 {
		limit = 60
	}
	return &Engine{
		allowedTools:        allowed,
		urlAllowlist:        urls,
		rateLimiter:         cfg.RateLimiter,
		rateLimitPerMinute:  limit,
		rateLimitFailClosed: cfg.RateLimitFailClosed,
		requireSQLLimit:     cfg.RequireSQLLimit,
		blockedSQLTables:    cfg.BlockedSQLTables,
	}
}

func (e *Engine) Check(ctx context.Context, req *PolicyRequest) (*PolicyDecision, error) {
	if req == nil {
		return deny("missing policy request"), nil
	}
	sanitized, masks := SanitizeInput(req.Input)
	if _, ok := e.allowedTools[req.ToolName]; !ok {
		return denyWithInput("tool is not allowed", sanitized, masks), nil
	}
	if !allowedPermission(req.UserID, req.Permission) {
		return denyWithInput("permission denied", sanitized, masks), nil
	}
	if err := e.checkRateLimit(ctx, req.UserID, req.ToolName); err != nil {
		return denyWithInput(err.Error(), sanitized, masks), nil
	}
	if err := e.checkSQL(req); err != nil {
		return denyWithInput(err.Error(), sanitized, masks), nil
	}
	if err := e.checkURL(req); err != nil {
		return denyWithInput(err.Error(), sanitized, masks), nil
	}
	return &PolicyDecision{Allowed: true, Reason: "allowed", SanitizedInput: sanitized, MaskFields: masks}, nil
}

func (e *Engine) checkRateLimit(ctx context.Context, userID, toolName string) error {
	if e.rateLimiter == nil {
		return nil
	}
	limitCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	count, err := e.rateLimiter.IncrWithTTL(limitCtx, repository.RateLimitKey(userID, toolName), time.Minute)
	if err != nil {
		if e.rateLimitFailClosed {
			return fmt.Errorf("rate limit unavailable")
		}
		return nil
	}
	if count > int64(e.rateLimitPerMinute) {
		return fmt.Errorf("rate limit exceeded")
	}
	return nil
}

func (e *Engine) checkSQL(req *PolicyRequest) error {
	raw, ok := stringInput(req.Input, "sql")
	if !ok && req.ToolName != "query_mysql_readonly" {
		return nil
	}
	if !ok || strings.TrimSpace(raw) == "" {
		return errors.New("sql is required")
	}
	sqlText := strings.TrimSpace(raw)
	lower := strings.ToLower(sqlText)
	if strings.Contains(lower, "--") || strings.Contains(lower, "/*") || strings.Contains(lower, "#") {
		return errors.New("sql comments are not allowed")
	}
	withoutTrailingSemicolon := strings.TrimRight(strings.TrimSpace(sqlText), ";")
	if strings.Contains(withoutTrailingSemicolon, ";") {
		return errors.New("multiple sql statements are not allowed")
	}
	if dangerousSQL.MatchString(sqlText) {
		return errors.New("dangerous sql statement rejected")
	}
	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with")) {
		return errors.New("only select queries are allowed")
	}
	if strings.HasPrefix(lower, "with") && !strings.Contains(lower, "select") {
		return errors.New("cte query must contain select")
	}
	for _, table := range e.blockedSQLTables {
		if table = strings.ToLower(strings.TrimSpace(table)); table != "" && regexp.MustCompile(`\b`+regexp.QuoteMeta(table)+`\b`).MatchString(lower) {
			return fmt.Errorf("blocked table access rejected")
		}
	}
	if e.requireSQLLimit && !regexp.MustCompile(`(?i)\blimit\s+\d+\b`).MatchString(sqlText) {
		return errors.New("sql limit is required")
	}
	return nil
}

func (e *Engine) checkURL(req *PolicyRequest) error {
	raw, ok := stringInput(req.Input, "url")
	if !ok && req.ToolName != "http_get" {
		return nil
	}
	if !ok || strings.TrimSpace(raw) == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url scheme is not allowed")
	}
	host := strings.ToLower(u.Hostname())
	if isPrivateHost(host) {
		return errors.New("private url host is not allowed")
	}
	if len(e.urlAllowlist) == 0 {
		return errors.New("url host is not allowlisted")
	}
	for allowed := range e.urlAllowlist {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return errors.New("url host is not allowlisted")
}

func allowedPermission(userID string, perm tool.PermissionLevel) bool {
	switch perm {
	case tool.PermissionReadOnly:
		return true
	case tool.PermissionSensitive:
		return userID == "admin" || userID == "system"
	case tool.PermissionAdmin:
		return userID == "system"
	default:
		return false
	}
}

func SanitizeInput(input map[string]any) (map[string]any, []string) {
	out := make(map[string]any, len(input))
	masks := make([]string, 0)
	for k, v := range input {
		if IsSensitiveField(k) {
			out[k] = "***"
			masks = append(masks, k)
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			sanitized, nestedMasks := SanitizeInput(nested)
			out[k] = sanitized
			masks = append(masks, nestedMasks...)
			continue
		}
		out[k] = v
	}
	return out, masks
}

func IsSensitiveField(name string) bool {
	n := strings.ToLower(name)
	for _, marker := range []string{"password", "token", "secret", "api_key", "apikey", "phone", "email", "id_card"} {
		if strings.Contains(n, marker) {
			return true
		}
	}
	return false
}

func PolicyRejectModel(req *PolicyRequest, decision *PolicyDecision) model.PolicyReject {
	input := map[string]any{}
	reason := "policy rejected"
	if decision != nil {
		input = decision.SanitizedInput
		reason = decision.Reason
	}
	return model.PolicyReject{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		ToolName:  req.ToolName,
		Reason:    reason,
		InputJSON: mustJSON(input),
	}
}

func stringInput(input map[string]any, key string) (string, bool) {
	v, ok := input[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func isPrivateHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func deny(reason string) *PolicyDecision {
	return denyWithInput(reason, map[string]any{}, nil)
}

func denyWithInput(reason string, input map[string]any, masks []string) *PolicyDecision {
	return &PolicyDecision{Allowed: false, Reason: reason, SanitizedInput: input, MaskFields: masks}
}
