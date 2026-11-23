package i18n

import "testing"

func TestTranslateEnglish(t *testing.T) {
	if got, want := Translate("en", "卡片不存在"), "Card not found."; got != want {
		t.Fatalf("Translate(en) = %q, want %q", got, want)
	}
}

func TestTranslateFallsBackToSource(t *testing.T) {
	const source = "未收录的消息"
	if got := Translate("en", source); got != source {
		t.Fatalf("unknown message = %q, want source %q", got, source)
	}
	if got := Translate("fr", "卡片不存在"); got != "卡片不存在" {
		t.Fatalf("unsupported language = %q, want Chinese source", got)
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{"en": "en", "zh": "zh", "": "zh", "EN": "zh"}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}
