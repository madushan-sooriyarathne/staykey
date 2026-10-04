package domain

import "testing"

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"+94771234567":      "+94771234567",
		"+94 77 123 4567":   "+94771234567",
		"0094 77-123-4567":  "+94771234567",
		"+44 (7700) 900123": "+447700900123",
	}
	for in, want := range ok {
		got, err := NormalizePhone(in)
		if err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "0771234567", "+0771234567", "+94 77 abc", "+12345", "+1234567890123456", "94+771234567"} {
		if got, err := NormalizePhone(in); err == nil {
			t.Errorf("NormalizePhone(%q) = %q, want an error", in, got)
		}
	}
}

func TestRolePermissions(t *testing.T) {
	cases := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleOwner, PermBilling, true},
		{RoleManager, PermSettings, true},
		{RoleManager, PermBilling, false},
		{RoleManager, PermTeam, false},
		{RoleCaretaker, PermPrices, false},
		{RoleCaretaker, PermManage, false},
	}
	for _, c := range cases {
		if got := c.role.Can(c.perm); got != c.want {
			t.Errorf("%s.Can(%s) = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}
