package sms

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"staykey.direct/api/internal/domain"
)

type recorder struct{ to []string }

func (r *recorder) Send(_ context.Context, to, _ string) error {
	r.to = append(r.to, to)
	return nil
}

func TestRouterSendsSriLankanNumbers(t *testing.T) {
	lk := &recorder{}
	if err := (Router{LK: lk}).Send(context.Background(), "+94771234567", "hi"); err != nil {
		t.Fatal(err)
	}
	if len(lk.to) != 1 || lk.to[0] != "+94771234567" {
		t.Fatalf("LK sender got %v, want [+94771234567]", lk.to)
	}
}

func TestRouterRefusesInternationalNumbers(t *testing.T) {
	lk := &recorder{}
	err := (Router{LK: lk}).Send(context.Background(), "+447700900123", "hi")
	var vErr *domain.ValidationError
	if !errors.As(err, &vErr) || vErr.Field != "phone" || len(lk.to) != 0 {
		t.Fatalf("err = %v, LK sender got %v; want a phone validation error and nothing sent", err, lk.to)
	}
}

func TestOzoneSendsTheForm(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		got = r.PostForm
		w.Write([]byte(`{"status_code": 204, "msg_id": "14364"}`))
	}))
	defer srv.Close()

	o := Ozone{UserID: "11", APIKey: "key", SenderID: "oZone Demo", URL: srv.URL}
	if err := o.Send(context.Background(), "+94771234567", "Your code is 123456."); err != nil {
		t.Fatal(err)
	}
	want := url.Values{
		"user_id":              {"11"},
		"api_key":              {"key"},
		"sender_id":            {"oZone Demo"},
		"recipient_contact_no": {"94771234567"},
		"message":              {"Your code is 123456."},
	}
	if got.Encode() != want.Encode() {
		t.Fatalf("form = %v, want %v", got, want)
	}
}

func TestOzoneReplies(t *testing.T) {
	tests := []struct {
		name       string
		httpStatus int
		reply      string
		wantErr    bool
		wantField  bool // a *domain.ValidationError on phone
	}{
		{name: "sent", httpStatus: 200, reply: `{"status_code": 204, "msg_id": "1"}`},
		{name: "sent, quoted status", httpStatus: 200, reply: `{"status_code": "204", "msg_id": "1"}`},
		{name: "invalid number", httpStatus: 200, reply: `{"status_code": 203}`, wantErr: true, wantField: true},
		{name: "wrong length", httpStatus: 200, reply: `{"status_code": 205}`, wantErr: true, wantField: true},
		{name: "no balance", httpStatus: 200, reply: `{"status_code": 208}`, wantErr: true},
		{name: "server error", httpStatus: 500, reply: `oops`, wantErr: true},
		{name: "not json", httpStatus: 200, reply: `<html>`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.httpStatus)
				w.Write([]byte(tt.reply))
			}))
			defer srv.Close()

			err := Ozone{URL: srv.URL}.Send(context.Background(), "+94771234567", "hi")
			var vErr *domain.ValidationError
			if (err != nil) != tt.wantErr || errors.As(err, &vErr) != tt.wantField {
				t.Fatalf("err = %v, want error %v, validation error %v", err, tt.wantErr, tt.wantField)
			}
		})
	}
}

func TestOzoneStopsWhenCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status_code": 204}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (Ozone{URL: srv.URL}).Send(ctx, "+94771234567", "hi"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
