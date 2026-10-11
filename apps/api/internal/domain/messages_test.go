package domain

import (
	"strings"
	"testing"
)

func TestNormalizeTemplateBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"trimmed", "  Hi {guest_name}  ", false},
		{"empty", "   ", true},
		{"too long", strings.Repeat("a", 1001), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NormalizeTemplateBody(tt.body); (err != nil) != tt.wantErr {
				t.Errorf("NormalizeTemplateBody error = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultsCoverEveryEvent(t *testing.T) {
	prefs := DefaultNotificationPrefs()
	if len(prefs) != len(NotificationEvents) {
		t.Fatalf("%d default preferences for %d events", len(prefs), len(NotificationEvents))
	}
	for i, p := range prefs {
		if p.Event != NotificationEvents[i] {
			t.Errorf("preference %d is %s, want %s", i, p.Event, NotificationEvents[i])
		}
	}
}
