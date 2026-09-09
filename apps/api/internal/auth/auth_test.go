package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("Abcdef12")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "Abcdef12") {
		t.Fatal("expected match")
	}
	if VerifyPassword(h, "wrong") {
		t.Fatal("expected mismatch")
	}
}

func TestValidatePassword(t *testing.T) {
	bad := []string{"", "short1", "alllettersonly", "12345678", "有中文没有数字"}
	for _, p := range bad {
		if ValidatePassword(p) == nil {
			t.Errorf("%q should be rejected", p)
		}
	}
	if err := ValidatePassword("Zy@123456"); err != nil {
		t.Errorf("Zy@123456 should pass: %v", err)
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	uid, tid := uuid.New(), uuid.New()
	at, err := issueAccess("secret", time.Minute, uid, tid, "alice", true)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := parseAccess("secret", at.Token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != uid.String() || claims.TenantID != tid.String() || claims.Username != "alice" || !claims.IsSuper || claims.ID != at.JTI {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	if _, err := parseAccess("other", at.Token); err == nil {
		t.Fatal("wrong secret must fail")
	}
	expired, _ := issueAccess("secret", -2*time.Minute, uid, tid, "alice", false)
	if _, err := parseAccess("secret", expired.Token); err == nil {
		t.Fatal("expired token must fail")
	}
}

func TestPrincipalHas(t *testing.T) {
	p := &Principal{perms: map[string]struct{}{"system:user:view": {}}}
	if !p.Has("system:user:view") || p.Has("system:user:delete") {
		t.Fatal("perm check wrong")
	}
	s := &Principal{IsSuper: true}
	if !s.Has("anything") {
		t.Fatal("super must have everything")
	}
	var nilP *Principal
	if nilP.Has("x") {
		t.Fatal("nil principal must have nothing")
	}
}
