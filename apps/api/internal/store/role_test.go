package store_test

import (
	"context"
	"testing"

	"staykey.direct/api/internal/store/storetest"
)

func TestAnOrdinaryRoleDoesNotBypassRowSecurity(t *testing.T) {
	t.Parallel()
	s := storetest.New(t)

	bypass, err := s.BypassesRowSecurity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if bypass {
		t.Error("the test role bypasses row-level security, want an ordinary role")
	}
}
