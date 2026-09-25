package aitoolsblocklist

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookup(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Fatal("missing accept header")
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer s.Close()
	c := New("test")
	c.BaseURL = s.URL
	result, err := c.Check(context.Background(), "example.com")
	if err != nil || result["ok"] != true {
		t.Fatalf("result=%v err=%v", result, err)
	}
}
