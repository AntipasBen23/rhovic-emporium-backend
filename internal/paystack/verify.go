package paystack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type VerifyResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Status    string `json:"status"`    // success, failed
		Amount    int64  `json:"amount"`
		Reference string `json:"reference"`
	} `json:"data"`
	Message string `json:"message"`
}

func (c *Client) Verify(ctx context.Context, reference string) (VerifyResponse, error) {
	if c.secret == "" {
		// dev mode: accept
		return VerifyResponse{
			Status: true,
			Data: struct {
				Status    string `json:"status"`
				Amount    int64  `json:"amount"`
				Reference string `json:"reference"`
			}{Status: "success", Amount: 0, Reference: reference},
		}, nil
	}

	u := "https://api.paystack.co/transaction/verify/" + reference
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+c.secret)

	client := &http.Client{Timeout: 12 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return VerifyResponse{}, err
	}
	defer res.Body.Close()

	var vr VerifyResponse
	if err := json.NewDecoder(res.Body).Decode(&vr); err != nil {
		return VerifyResponse{}, err
	}
	if !vr.Status {
		return vr, errors.New(vr.Message)
	}
	return vr, nil
}