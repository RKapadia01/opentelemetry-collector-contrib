// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package presidioredactionprocessor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type presidioRedaction struct {
	config *Config
	logger *zap.Logger
}

func newPresidioRedaction(_ context.Context, cfg *Config, logger *zap.Logger) *presidioRedaction {
	return &presidioRedaction{
		config: cfg,
		logger: logger,
	},

func (s *presidioRedaction) processTraces(ctx context.Context, batch ptrace.Traces) (ptrace.Traces, error) {
	for i := 0; i < batch.ResourceSpans().Len(); i++ {
		rs := batch.ResourceSpans().At(i)
		s.processResourceSpan(ctx, rs)
	}

	return batch, nil
}

func (s *presidioRedaction) processResourceSpan(ctx context.Context, rs ptrace.ResourceSpans) {
	// Redact the resource span attributes if necessary?
	// rsAttrs := rs.Resource().Attributes()
	// s.redactAttr(ctx, rsAttrs)

	for j := 0; j < rs.ScopeSpans().Len(); j++ {
		ils := rs.ScopeSpans().At(j)
		for k := 0; k < ils.Spans().Len(); k++ {
			span := ils.Spans().At(k)
			spanAttrs := span.Attributes()

			// Redact the span attributes if necessary
			s.redactAttr(ctx, spanAttrs)
		}
	}
}

func (s *presidioRedaction) redactAttr(ctx context.Context, attributes pcommon.Map) {
	// for each attribute in the map, call out to the presidio service to redact the attribute
	attributes.Range(func(k string, v pcommon.Value) bool {
		redactedValue, err := s.callPresidioService(ctx, v.Str())
		if err != nil {
			s.logger.Error("Error calling presidio service", zap.Error(err))
			return true
		}
		attributes.PutStr(k, redactedValue)
		return true
	})
}

func (s *presidioRedaction) callPresidioService(ctx context.Context, value string) (string, error) {
	url := s.config.PresidioEndpoint // Presidio service endpoint

	// Create the request payload
	payload := map[string]string{
		"text": value,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check for non-OK status codes
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Presidio service returned status code %d", resp.StatusCode)
	}

	// Parse the response
	var result struct {
		RedactedText string `json:"redactedText"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	return result.RedactedText, nil
}
