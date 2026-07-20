package main

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"lattice-web/backend/models"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password was stored in plaintext")
	}
	if !checkPassword(hash, "correct horse battery staple") {
		t.Fatal("valid password rejected")
	}
	if checkPassword(hash, "wrong password") {
		t.Fatal("invalid password accepted")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	a := &authService{secret: []byte("test-secret")}
	user := models.User{ID: uuid.New(), Role: "admin"}
	token, err := a.generateToken(user)
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	claims, err := a.parseToken(token)
	if err != nil {
		t.Fatalf("parseToken: %v", err)
	}
	if claims.Subject != user.ID.String() {
		t.Fatalf("subject = %q, want %q", claims.Subject, user.ID.String())
	}
	if claims.Role != "admin" {
		t.Fatalf("role = %q, want admin", claims.Role)
	}
}

func TestParseTokenRejectsWrongSecret(t *testing.T) {
	signer := &authService{secret: []byte("real-secret")}
	attacker := &authService{secret: []byte("other-secret")}
	token, err := signer.generateToken(models.User{ID: uuid.New()})
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	if _, err := attacker.parseToken(token); err == nil {
		t.Fatal("token signed with a different secret was accepted")
	}
}

func TestParseTokenRejectsNoneAlg(t *testing.T) {
	a := &authService{secret: []byte("secret")}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, authClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: uuid.New().String()},
	})
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}
	if _, err := a.parseToken(signed); err == nil {
		t.Fatal("alg=none token was accepted")
	}
}

func TestValidateEmail(t *testing.T) {
	valid := []string{"a@b.co", "User.Name+tag@example.com", "x@sub.domain.org"}
	for _, e := range valid {
		if _, err := validateEmail(e); err != nil {
			t.Errorf("validateEmail(%q) unexpected error: %v", e, err)
		}
	}
	invalid := []string{"", "notanemail", "a@b", "a@.com", "@b.com", "a b@c.com"}
	for _, e := range invalid {
		if _, err := validateEmail(e); err == nil {
			t.Errorf("validateEmail(%q) expected error", e)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if _, err := validatePassword("12345678"); err != nil {
		t.Errorf("8-char password rejected: %v", err)
	}
	if _, err := validatePassword("short"); err == nil {
		t.Error("short password accepted")
	}
	long := make([]byte, maxPasswordLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := validatePassword(string(long)); err == nil {
		t.Error("over-long password accepted")
	}
}
