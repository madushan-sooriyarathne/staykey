package domain

import (
	"bytes"
	"errors"
	"testing"
)

func TestNewInviteTokenHashesToTheStoredHash(t *testing.T) {
	token, hash, err := NewInviteToken()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(HashInviteToken(token), hash) {
		t.Error("hash of the token doesn't match the stored hash")
	}
}

func TestNewInviteValidate(t *testing.T) {
	tests := []struct {
		name  string
		in    NewInvite
		field string
	}{
		{"caretaker with a local format number", NewInvite{Phone: "+94 71 555 0199", Role: RoleCaretaker}, ""},
		{"manager", NewInvite{Phone: "+94715550199", Role: RoleManager}, ""},
		{"owner can't be invited", NewInvite{Phone: "+94715550199", Role: RoleOwner}, "role"},
		{"number without a country code", NewInvite{Phone: "0715550199", Role: RoleCaretaker}, "phone"},
		{"international number", NewInvite{Phone: "+44 7700 900123", Role: RoleCaretaker}, "phone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.Normalize()
			var vErr *ValidationError
			err := tt.in.Validate()
			field := ""
			if errors.As(err, &vErr) {
				field = vErr.Field
			}
			if field != tt.field {
				t.Errorf("Validate() = %v, want an error on %q", err, tt.field)
			}
		})
	}
}

func TestMemberPatchRefusesOwnerRole(t *testing.T) {
	owner := RoleOwner
	var vErr *ValidationError
	if err := (MemberPatch{Role: &owner}).Validate(); !errors.As(err, &vErr) {
		t.Errorf("Validate() = %v, want a validation error", err)
	}
}
