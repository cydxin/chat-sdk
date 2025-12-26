package service

import (
	"net/http"
	"net/url"
	"testing"
)

func TestAuthService_ExtractToken_BearerFirst(t *testing.T) {
	a := NewAuthService(nil)

	req := &http.Request{Header: make(http.Header), URL: &url.URL{RawQuery: "token=q"}}
	req.Header.Set("Authorization", "Bearer headerToken")

	got := a.ExtractToken(req)
	if got != "headerToken" {
		t.Fatalf("expected headerToken, got %q", got)
	}
}

func TestAuthService_ExtractToken_QueryFallback(t *testing.T) {
	a := NewAuthService(nil)

	u, _ := url.Parse("http://example.com/path?token=queryToken")
	req := &http.Request{Header: make(http.Header), URL: u}

	got := a.ExtractToken(req)
	if got != "queryToken" {
		t.Fatalf("expected queryToken, got %q", got)
	}
}
