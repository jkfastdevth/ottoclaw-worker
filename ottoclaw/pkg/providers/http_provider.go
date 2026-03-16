// OttoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 OttoClaw contributors

package providers

import (
	"context"
	"time"

	"github.com/sipeed/ottoclaw/pkg/providers/openai_compat"
	"github.com/sipeed/ottoclaw/pkg/utils"
)

type HTTPProvider struct {
	delegate *openai_compat.Provider
	limiter  *utils.RateLimiter
}

func NewHTTPProvider(apiKey, apiBase, proxy string) *HTTPProvider {
	return &HTTPProvider{
		delegate: openai_compat.NewProvider(apiKey, apiBase, proxy),
	}
}

func NewHTTPProviderWithMaxTokensField(apiKey, apiBase, proxy, maxTokensField string) *HTTPProvider {
	return NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(apiKey, apiBase, proxy, maxTokensField, 0)
}

func NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(
	apiKey, apiBase, proxy, maxTokensField string,
	requestTimeoutSeconds int,
) *HTTPProvider {
	return &HTTPProvider{
		delegate: openai_compat.NewProvider(
			apiKey,
			apiBase,
			proxy,
			openai_compat.WithMaxTokensField(maxTokensField),
			openai_compat.WithRequestTimeout(time.Duration(requestTimeoutSeconds)*time.Second),
		),
	}
}

func (p *HTTPProvider) SetRPM(rpm int) {
	if rpm > 0 {
		p.limiter = utils.NewRateLimiter(rpm)
	}
}

func (p *HTTPProvider) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (*LLMResponse, error) {
	if p.limiter != nil {
		p.limiter.Wait()
	}
	return p.delegate.Chat(ctx, messages, tools, model, options)
}

func (p *HTTPProvider) GetDefaultModel() string {
	return ""
}
