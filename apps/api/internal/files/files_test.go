package files

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestKeys(t *testing.T) {
	account := uuid.Must(uuid.NewV7())
	key, err := NewKey(account, KindPhoto, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if !Belongs(key, account, KindPhoto) || Belongs(key, account, KindLogo) || Belongs(key, uuid.New(), KindPhoto) {
		t.Errorf("Belongs is wrong for %s", key)
	}
	if _, err := NewKey(account, KindPhoto, "image/gif"); err == nil {
		t.Error("gif accepted")
	}
	for _, bad := range []string{
		"accounts/../etc/passwd",
		"accounts/" + account.String() + "/photos/../../x.jpg",
		"/" + key,
	} {
		if ValidKey(bad) {
			t.Errorf("ValidKey(%q) = true", bad)
		}
	}
}

func TestLocalUploadFlow(t *testing.T) {
	store, err := NewLocal(t.TempDir(), []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.StripPrefix("", store.Handler()))
	defer srv.Close()
	ctx := WithBaseURL(context.Background(), srv.URL)

	key, _ := NewKey(uuid.Must(uuid.NewV7()), KindPhoto, "image/png")
	body := []byte("not really a png")
	up, err := store.PresignUpload(ctx, key, "image/png", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(up.URL, srv.URL+MediaPrefix+key+"?") {
		t.Fatalf("upload URL %s", up.URL)
	}

	put := func(target, contentType string, data []byte) int {
		req, _ := http.NewRequest(http.MethodPut, target, bytes.NewReader(data))
		req.Header.Set("Content-Type", contentType)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	if code := put(up.URL, "image/jpeg", body); code != http.StatusBadRequest {
		t.Errorf("wrong content type: %d", code)
	}
	if code := put(up.URL, "image/png", append(body, 'x')); code != http.StatusBadRequest {
		t.Errorf("wrong size: %d", code)
	}
	tampered, _ := url.Parse(up.URL)
	q := tampered.Query()
	q.Set("size", "99999")
	tampered.RawQuery = q.Encode()
	if code := put(tampered.String(), "image/png", body); code != http.StatusForbidden {
		t.Errorf("tampered size: %d", code)
	}
	if ok, _ := store.Exists(ctx, key); ok {
		t.Fatal("file exists before a valid upload")
	}

	if code := put(up.URL, "image/png", body); code != http.StatusOK {
		t.Fatalf("upload: %d", code)
	}
	if ok, err := store.Exists(ctx, key); !ok || err != nil {
		t.Fatalf("exists = %v, %v", ok, err)
	}
	res, err := http.Get(up.PublicURL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" {
		t.Errorf("get: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	store.Now = func() time.Time { return time.Now().Add(time.Hour) }
	if code := put(up.URL, "image/png", body); code != http.StatusForbidden {
		t.Errorf("expired link: %d", code)
	}
}

func TestR2Presign(t *testing.T) {
	store, err := NewR2(R2Config{
		Endpoint: "https://example.r2.cloudflarestorage.com", Bucket: "media",
		AccessKeyID: "key", SecretAccessKey: "secret", PublicURL: "https://media.staykey.direct/",
	})
	if err != nil {
		t.Fatal(err)
	}
	key, _ := NewKey(uuid.Must(uuid.NewV7()), KindLogo, "image/webp")
	up, err := store.PresignUpload(context.Background(), key, "image/webp", 1234)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(up.URL)
	if u.Host != "example.r2.cloudflarestorage.com" || u.Path != "/media/"+key || u.Query().Get("X-Amz-Signature") == "" {
		t.Errorf("presigned URL %s", up.URL)
	}
	if signed := u.Query().Get("X-Amz-SignedHeaders"); !strings.Contains(signed, "content-length") || !strings.Contains(signed, "content-type") {
		t.Errorf("signed headers %q should pin type and size", signed)
	}
	if up.PublicURL != "https://media.staykey.direct/"+key {
		t.Errorf("public URL %s", up.PublicURL)
	}
}
