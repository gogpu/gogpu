package platform

import "github.com/gogpu/gpucontext"

// imeContentIsSensitive identifies content hints that make surrounding text
// unsafe to expose to a native input method.  ContentPurposePassword carries
// the same privacy requirement even when a caller omitted an explicit hint.
// Keep this policy in one place so every backend makes the same decision at
// the native boundary.
func imeContentIsSensitive(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) bool {
	return purpose == gpucontext.ContentPurposePassword ||
		hints.Has(gpucontext.ContentHintHiddenText) ||
		hints.Has(gpucontext.ContentHintSensitiveData)
}
