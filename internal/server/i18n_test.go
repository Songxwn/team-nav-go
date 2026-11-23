package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLangPrecedence(t *testing.T) {
	cases := []struct {
		name   string
		url    string
		cookie string
		accept string
		want   string
	}{
		{"query beats cookie", "/?lang=en", "zh", "zh-CN", "en"},
		{"cookie beats accept", "/", "en", "zh-CN", "en"},
		{"accept english", "/", "", "en-US,en;q=0.9", "en"},
		{"accept chinese", "/", "", "zh-CN,zh;q=0.9", "zh"},
		{"unsupported falls back", "/", "", "fr-FR", "zh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			if tc.accept != "" {
				req.Header.Set("Accept-Language", tc.accept)
			}
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "tn_lang", Value: tc.cookie})
			}
			if got := requestLang(req); got != tc.want {
				t.Fatalf("requestLang() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriteErrorLocalizes(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &statusRecorder{ResponseWriter: rec, status: http.StatusOK, lang: "en"}

	writeError(w, http.StatusNotFound, "卡片不存在")

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "Card not found." {
		t.Fatalf("error = %q, want English translation", body.Error)
	}
}
