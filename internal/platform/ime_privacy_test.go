package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestIMEContentSensitivityPolicy(t *testing.T) {
	tests := []struct {
		name    string
		purpose gpucontext.ContentPurpose
		hints   gpucontext.ContentHint
		want    bool
	}{
		{name: "normal", purpose: gpucontext.ContentPurposeName},
		{name: "password-purpose", purpose: gpucontext.ContentPurposePassword, want: true},
		{name: "hidden-hint", hints: gpucontext.ContentHintHiddenText, want: true},
		{name: "sensitive-hint", hints: gpucontext.ContentHintSensitiveData, want: true},
		{name: "mixed-hints", hints: gpucontext.ContentHintCompletion | gpucontext.ContentHintSensitiveData, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := imeContentIsSensitive(test.purpose, test.hints); got != test.want {
				t.Fatalf("imeContentIsSensitive(%v, %v) = %v, want %v", test.purpose, test.hints, got, test.want)
			}
		})
	}
}
