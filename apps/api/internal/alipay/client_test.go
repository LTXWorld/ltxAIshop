package alipay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestPagePaySignsRequest(t *testing.T) {
	client := testClient(t)
	client.now = func() time.Time { return time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC) }

	response, err := client.PagePay(PagePayRequest{
		OutTradeNo:  "LTX123",
		Subject:     "Order LTX123",
		TotalAmount: "198.00",
	})
	if err != nil {
		t.Fatalf("PagePay returned error: %v", err)
	}

	if response.Params.Get("method") != MethodPagePay {
		t.Fatalf("method = %q, want %q", response.Params.Get("method"), MethodPagePay)
	}
	if response.Params.Get("sign") == "" {
		t.Fatal("sign should be set")
	}
	if !client.Verify(response.Params) {
		t.Fatal("Verify returned false for signed params")
	}
	if !strings.Contains(response.FormHTML, "alipay.trade.page.pay") {
		t.Fatal("form HTML should include page pay method")
	}
}

func TestAmountFromCents(t *testing.T) {
	if got := AmountFromCents(19800); got != "198.00" {
		t.Fatalf("AmountFromCents = %q, want 198.00", got)
	}

	cents, err := CentsFromAmount("198.00")
	if err != nil {
		t.Fatalf("CentsFromAmount returned error: %v", err)
	}
	if cents != 19800 {
		t.Fatalf("cents = %d, want 19800", cents)
	}
}

func testClient(t *testing.T) *Client {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey returned error: %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey returned error: %v", err)
	}

	client, err := NewClient(Config{
		AppID:      "2021000000000000",
		GatewayURL: "https://openapi-sandbox.dl.alipaydev.com/gateway.do",
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		PublicKey:  base64.StdEncoding.EncodeToString(publicDER),
		NotifyURL:  "https://example.com/api/payments/alipay/notify",
		ReturnURL:  "https://example.com/payments/return",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}
