package token

import (
	"testing"

	"github.com/jfrog/jfrog-client-go/auth/accesstoken"
	"github.com/stretchr/testify/assert"
)

const testUserSub = "jfac@01ftqhpvg3kn760kg8qbt411qy/users/admin"

func makeParsed(sub, scp string, ext *accesstoken.ParsedExt) *accesstoken.ParsedToken {
	return &accesstoken.ParsedToken{
		Payload:   accesstoken.Payload{Sub: sub, Scp: scp, Iss: "jfac@01ftqhpvg3kn760kg8qbt411qy"},
		ParsedExt: ext,
	}
}

func TestIdentify_AdminToken(t *testing.T) {
	assert.Equal(t, []TokenType{TokenTypeAdmin}, IdentifyTokenTypes(makeParsed("my_token", AdminScope, nil)))
}

func TestIdentify_IdentityToken(t *testing.T) {
	assert.Equal(t, []TokenType{TokenTypeIdentity}, IdentifyTokenTypes(makeParsed(testUserSub, "applied-permissions/user", nil)))
}

func TestIdentify_AppliedPermissionsGroupsToken(t *testing.T) {
	parsed := makeParsed("my_token", "applied-permissions/groups:readers,writers", nil)
	assert.Equal(t, []TokenType{TokenTypeGroups}, IdentifyTokenTypes(parsed))
}

func TestIdentify_ScopedToken(t *testing.T) {
	assert.Equal(t, []TokenType{TokenTypeScoped}, IdentifyTokenTypes(makeParsed("my_token", "system:metrics:r", nil)))
}

func TestIdentify_MultiScopeToken(t *testing.T) {
	parsed := makeParsed(testUserSub, "applied-permissions/user system:metrics:r", nil)
	assert.Equal(t, []TokenType{TokenTypeIdentity, TokenTypeScoped}, IdentifyTokenTypes(parsed))
}

func TestIdentify_UnknownScope(t *testing.T) {
	parsed := makeParsed("my_token", "something-not-matching-any-rule", nil)
	assert.Equal(t, []TokenType{TokenTypeUnknown}, IdentifyTokenTypes(parsed))
}

func TestIdentify_Internal(t *testing.T) {
	for _, parsed := range []*accesstoken.ParsedToken{
		makeParsed(testUserSub, "internal:x:r", nil),
		makeParsed(testUserSub, "applied-permissions/user internal:x:r", nil),
		makeParsed("abcd@01h", AdminScope, nil),
		makeParsed("abcd@01h/nodes/n1", "api:*", nil),
		makeParsed("abcd@01h/users/alice", "applied-permissions/user", &accesstoken.ParsedExt{Sid: "s1"}),
	} {
		assert.Equal(t, []TokenType{TokenTypeInternal}, IdentifyTokenTypes(parsed), "sub %q scp %q", parsed.Payload.Sub, parsed.Payload.Scp)
	}
}

func TestIdentify_NotInternal(t *testing.T) {
	for _, tc := range []struct {
		parsed *accesstoken.ParsedToken
		want   []TokenType
	}{
		{makeParsed("jfac@01h/users/admin", AdminScope, nil), []TokenType{TokenTypeAdmin}},
		{makeParsed("my_token", AdminScope, nil), []TokenType{TokenTypeAdmin}},
		{makeParsed(testUserSub, "applied-permissions/user", &accesstoken.ParsedExt{Revocable: "true"}), []TokenType{TokenTypeIdentity}},
	} {
		assert.Equal(t, tc.want, IdentifyTokenTypes(tc.parsed), "sub %q scp %q", tc.parsed.Payload.Sub, tc.parsed.Payload.Scp)
	}
}

func TestIdentify_GroupsPrefix(t *testing.T) {
	assert.Equal(t, []TokenType{TokenTypeGroups}, IdentifyTokenTypes(makeParsed("my_token", "applied-permissions/groups:readers,writers", nil)))
	assert.Equal(t, []TokenType{TokenTypeUnknown}, IdentifyTokenTypes(makeParsed("my_token", "applied-permissions/group:readers", nil)))
}

func TestIdentify_AtcScopes(t *testing.T) {
	for _, tc := range []struct {
		scope string
		want  []TokenType
	}{
		{"applied-permissions/user", []TokenType{TokenTypeIdentity}},
		{AdminScope, []TokenType{TokenTypeAdmin}},
		{GroupsScopePrefix + "readers" + " " + AdminScope, []TokenType{TokenTypeGroups, TokenTypeAdmin}},
		{"system:metrics:r", []TokenType{TokenTypeScoped}},
	} {
		assert.Equal(t, tc.want, IdentifyTokenTypes(makeParsed(testUserSub, tc.scope, nil)), tc.scope)
	}
}
