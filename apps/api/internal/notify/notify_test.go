package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestExpoReturnsTokensThatNoLongerWork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msgs []expoMessage
		if err := json.NewDecoder(r.Body).Decode(&msgs); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"data":[{"status":"ok","id":"1"},{"status":"error","message":"gone","details":{"error":"DeviceNotRegistered"}}]}`))
	}))
	defer srv.Close()

	dead, err := Expo{URL: srv.URL}.Send(context.Background(), []Push{
		{Token: "ExponentPushToken[live]", Title: "Request from Sam"},
		{Token: "ExponentPushToken[gone]", Title: "Request from Sam"},
	})
	if err != nil || !slices.Equal(dead, []string{"ExponentPushToken[gone]"}) {
		t.Errorf("dead = %v (err %v), want the unregistered token", dead, err)
	}
}

func TestExpoSendsInBatchesOfAHundred(t *testing.T) {
	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msgs []expoMessage
		if err := json.NewDecoder(r.Body).Decode(&msgs); err != nil {
			t.Error(err)
		}
		sizes = append(sizes, len(msgs))
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	pushes := make([]Push, 150)
	for i := range pushes {
		pushes[i] = Push{Token: fmt.Sprintf("ExponentPushToken[%d]", i)}
	}

	if _, err := (Expo{URL: srv.URL}).Send(context.Background(), pushes); err != nil || !slices.Equal(sizes, []int{100, 50}) {
		t.Errorf("batches = %v (err %v), want [100 50]", sizes, err)
	}
}

func TestCloudWhatsAppSendsTheTemplate(t *testing.T) {
	var got waMessage
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"messages":[{"id":"wamid.1"}]}`))
	}))
	defer srv.Close()

	c := CloudWhatsApp{Token: "secret", Template: "staykey_alert", URL: srv.URL}
	if err := c.SendWhatsApp(context.Background(), "+94771234567", "Request from Sam", "12 to 15 Oct"); err != nil {
		t.Fatal(err)
	}
	params := got.Template.Components[0].Parameters
	if auth != "Bearer secret" || got.To != "94771234567" || got.Template.Name != "staykey_alert" || params[1].Text != "12 to 15 Oct" {
		t.Errorf("sent %+v with %q", got, auth)
	}
}

func TestCloudWhatsAppReportsRefusals(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	if err := (CloudWhatsApp{URL: srv.URL}).SendWhatsApp(context.Background(), "+94771234567", "a", "b"); err == nil {
		t.Error("a 400 from the Cloud API should be an error")
	}
}

func TestLogWhatsAppKeepsNumbersAndTextOutOfTheLog(t *testing.T) {
	var buf bytes.Buffer
	l := LogWhatsApp{Logger: slog.New(slog.NewTextHandler(&buf, nil))}

	if err := l.SendWhatsApp(context.Background(), "+94771234567", "Request from Sam Lee", "12 to 15 Oct"); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); strings.Contains(out, "771234") || strings.Contains(out, "Sam Lee") || !strings.Contains(out, "***567") {
		t.Errorf("log line = %q, want only the masked number", out)
	}
}
