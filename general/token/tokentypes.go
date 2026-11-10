package token

import (
	"regexp"
	"strings"

	"github.com/jfrog/jfrog-client-go/auth/accesstoken"
)

type TokenType string

const (
	TokenTypeAdmin    TokenType = "Admin Token"
	TokenTypeIdentity TokenType = "Identity Token"
	TokenTypeGroups   TokenType = "Applied Permissions Groups Token"
	TokenTypeScoped   TokenType = "Scoped Token"
	TokenTypeInternal TokenType = "Internal Token"
	TokenTypeUnknown  TokenType = "Unknown Token Type"

	userScope           = "applied-permissions/user"
	internalScopePrefix = "internal:"
)

var (
	serviceSubjectPattern = regexp.MustCompile(`^[a-z]{4,6}@[a-zA-Z0-9*]+(/nodes/[^/]+)?$`)
	scopedTokenPattern    = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*:.+:[a-zA-Z*]+$`)
)

func IdentifyTokenTypes(t *accesstoken.ParsedToken) []TokenType {
	if isInternal(t) {
		return []TokenType{TokenTypeInternal}
	}
	var types []TokenType
	seen := map[TokenType]bool{}
	for _, scope := range t.Payload.Scopes() {
		tokenType := classifyScope(scope)
		if !seen[tokenType] {
			seen[tokenType] = true
			types = append(types, tokenType)
		}
	}
	return types
}

func isInternal(t *accesstoken.ParsedToken) bool {
	for _, scope := range t.Payload.Scopes() {
		if strings.HasPrefix(scope, internalScopePrefix) {
			return true
		}
	}
	return serviceSubjectPattern.MatchString(t.Payload.Sub) || (t.ParsedExt != nil && t.ParsedExt.Sid != "")
}

func classifyScope(scope string) TokenType {
	switch {
	case scope == AdminScope:
		return TokenTypeAdmin
	case scope == userScope:
		return TokenTypeIdentity
	case strings.HasPrefix(scope, GroupsScopePrefix):
		return TokenTypeGroups
	case scopedTokenPattern.MatchString(scope):
		return TokenTypeScoped
	default:
		return TokenTypeUnknown
	}
}
