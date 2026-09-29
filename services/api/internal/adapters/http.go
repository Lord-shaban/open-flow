package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"io"
	"net/http"
	"time"
)

func boundedClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// Single request, no automatic paid/ambiguous submission retry; no raw errors.
func request(ctx context.Context, client *http.Client, method, endpoint string, headers map[string]string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, errors.New("provider payload invalid")
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, SafeError(err, method == "POST")
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, SafeError(err, method == "POST")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		category := provider.Transient
		known := method != "POST"
		switch res.StatusCode {
		case 400, 404, 422:
			category = provider.Unsupported
			known = true
		case 401, 403:
			category = provider.Authentication
			known = true
		case 402:
			category = provider.Billing
			known = true
		case 429:
			category = provider.Quota
			known = true
		}
		if !known {
			category = provider.AmbiguousSubmission
		}
		return nil, &provider.Error{Category: category, Message: "Provider request could not be completed", AcceptanceKnown: known}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (24<<20)+1))
	if err != nil || len(data) > 24<<20 {
		return nil, SafeError(errors.New("provider response unavailable"), method == "POST")
	}
	return data, nil
}
