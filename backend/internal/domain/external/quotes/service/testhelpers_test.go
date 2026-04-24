package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/meowmix1337/argus/backend/internal/platform/httpclient"
)

type fakeHTTPClient struct {
	responseBody any
	rawBytes     []byte
	err          error
}

func (f *fakeHTTPClient) Get(_ context.Context, _ string, result any, _ ...httpclient.RequestOption) error {
	if f.err != nil {
		return f.err
	}
	b, err := json.Marshal(f.responseBody)
	if err != nil {
		return fmt.Errorf("fakeHTTPClient: marshal: %w", err)
	}
	return json.Unmarshal(b, result)
}

func (f *fakeHTTPClient) Post(_ context.Context, _ string, _ any, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) Put(_ context.Context, _ string, _ any, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) Delete(_ context.Context, _ string, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) GetBytes(_ context.Context, _ string, _ ...httpclient.RequestOption) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.rawBytes != nil {
		return f.rawBytes, nil
	}
	return json.Marshal(f.responseBody)
}
