package main

import (
	"fmt"
	"testing"

	"github.com/distribution/distribution/v3/registry/api/errcode"
	"go.podman.io/common/pkg/retry"
)

func TestRegistryErrorTypeIdentity(t *testing.T) {
	for _, tc := range []struct {
		code            errcode.ErrorCode
		retryable       bool
		missingManifest bool
	}{
		{errcode.ErrorCodeUnauthorized, false, false},
		{errcode.ErrorCodeDenied, false, false},
		{errcode.ErrorCodeNameUnknown, false, false},
		{errcode.ErrorCodeManifestUnknown, false, true},
		{errcode.ErrorCodeBlobUnknown, true, false},
		{errcode.ErrorCodeTooManyRequests, true, false},
		{errcode.ErrorCodeUnknown, true, false},
	} {
		t.Run(tc.code.String(), func(t *testing.T) {
			err := fmt.Errorf("wrapped registry operation: %w", tc.code.WithMessage("fixture"))
			if got := retry.IsErrorRetryable(err); got != tc.retryable {
				t.Fatalf("registry retry classification changed: got %v want %v", got, tc.retryable)
			}
			if got := isDockerManifestUnknownError(err); got != tc.missingManifest {
				t.Fatalf("manifest classification changed: got %v want %v", got, tc.missingManifest)
			}
		})
	}
}
