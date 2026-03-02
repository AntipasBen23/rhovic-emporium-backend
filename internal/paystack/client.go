package paystack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type Client struct {
	secret string
	http   *http.Client
}

func New(secret string) *Client {
	return &Client{
		secret: secret,
		http:   &http.Client{Timeout: 12 * time.Second},
	}
}

type InitRequest struct {
	Email  string `json:"email"`
	Amount int64  `json:"amount"` // kobo
	Ref    string `json:"reference"`
}

type InitResponse struct {
	AuthorizationURL string
	AccessCode       string
	Reference        string
}

func (c *Client) Initialize(ctx context.Context, req InitRequest) (InitResponse, error) {
	// Dev fallback: allow running without Paystack configured.
	if c.secret == "" {
		return InitResponse{
			AuthorizationURL: "https://example.com/paystack-mock",
			AccessCode:       "mock",
			Reference:        req.Ref,
		}, nil
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", "https://api.paystack.co/transaction/initialize", bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+c.secret)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(httpReq)
	if err != nil {
		return InitResponse{}, err
	}
	defer res.Body.Close()

	var raw struct {
		Status bool `json:"status"`
		Data   struct {
			AuthorizationURL string `json:"authorization_url"`
			AccessCode       string `json:"access_code"`
			Reference        string `json:"reference"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return InitResponse{}, err
	}
	if !raw.Status {
		return InitResponse{}, errors.New(raw.Message)
	}
	return InitResponse{
		AuthorizationURL: raw.Data.AuthorizationURL,
		AccessCode:       raw.Data.AccessCode,
		Reference:        raw.Data.Reference,
	}, nil
}