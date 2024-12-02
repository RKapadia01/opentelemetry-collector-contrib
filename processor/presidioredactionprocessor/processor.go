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
	"go.opentelemetry.io/collector/pdata/plog"
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
	}
}

func (s *presidioRedaction) processTraces(ctx context.Context, batch ptrace.Traces) (ptrace.Traces, error) {
	for i := 0; i < batch.ResourceSpans().Len(); i++ {
		rs := batch.ResourceSpans().At(i)
		s.processResourceSpan(ctx, rs)
	}

	return batch, nil
}

func (s *presidioRedaction) processLogs(ctx context.Context, logs plog.Logs) (plog.Logs, error) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		s.processResourceLog(ctx, rl)
	}

	return logs, nil
}

func (s *presidioRedaction) processResourceLog(ctx context.Context, rl plog.ResourceLogs) {
	for j := 0; j < rl.ScopeLogs().Len(); j++ {
		ils := rl.ScopeLogs().At(j)
		for k := 0; k < ils.LogRecords().Len(); k++ {
			log := ils.LogRecords().At(k)

			s.redactAttr(ctx, log.Attributes())

			redactedBody, err := s.getRedactedValue(ctx, log.Body().Str())
			if err != nil {
				s.logger.Error("Error calling presidio service", zap.Error(err))
				continue
			}

			log.Body().SetStr(redactedBody)
		}
	}
}

func (s *presidioRedaction) processResourceSpan(ctx context.Context, rs ptrace.ResourceSpans) {
	rsAttrs := rs.Resource().Attributes()
	s.redactAttr(ctx, rsAttrs)

	for j := 0; j < rs.ScopeSpans().Len(); j++ {
		ils := rs.ScopeSpans().At(j)
		for k := 0; k < ils.Spans().Len(); k++ {
			span := ils.Spans().At(k)
			spanAttrs := span.Attributes()

			s.redactAttr(ctx, spanAttrs)
		}
	}
}

func (s *presidioRedaction) redactAttr(ctx context.Context, attributes pcommon.Map) {
	attributes.Range(func(k string, v pcommon.Value) bool {
		redactedValue, err := s.getRedactedValue(ctx, v.Str())
		if err != nil {
			s.logger.Error("Error retrieving the redacted value", zap.Error(err))
			return true
		}
		attributes.PutStr(k, redactedValue)
		return true
	})
}

func (s *presidioRedaction) getRedactedValue(ctx context.Context, value string) (string, error) {
	analysisResults, err := s.callPresidioAnalyzer(ctx, value)
	if err != nil {
		return "", err
	}

	anonymizerResult, err := s.callPresidioAnonymizer(ctx, value, analysisResults)
	if err != nil {
		return "", err
	}

	return anonymizerResult.Text, nil
}

func (s *presidioRedaction) callPresidioAnalyzer(ctx context.Context, value string) (PresidioAnalyzerResponse, error) {
	// Create a PresidioAnalyzerRequest with default settings
	requestPayload := PresidioAnalyzerRequest{
		Text:           value,
		Language:       "en",
		ScoreThreshold: 0.5,
		Entities:       nil,
		Context:        nil,
	}

	// Marshal the request payload into JSON
	jsonPayload, err := json.Marshal(requestPayload)
	if err != nil {
		return PresidioAnalyzerResponse{}, fmt.Errorf("Failed to marshal request payload: %v", err)
	}

	// Construct the HTTP request
	url := s.config.AnalyzerEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return PresidioAnalyzerResponse{}, fmt.Errorf("Failed to create HTTP request: %v", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return PresidioAnalyzerResponse{}, fmt.Errorf("Failed to execute HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check for non-OK status codes
	if resp.StatusCode != http.StatusOK {
		return PresidioAnalyzerResponse{}, fmt.Errorf("Presidio service returned status code %d", resp.StatusCode)
	}

	// Parse the response
	var presidioAnalyzerResponse PresidioAnalyzerResponse
	err = json.NewDecoder(resp.Body).Decode(&presidioAnalyzerResponse)
	if err != nil {
		return PresidioAnalyzerResponse{}, err
	}

	return presidioAnalyzerResponse, nil
}

func (s *presidioRedaction) callPresidioAnonymizer(ctx context.Context, value string, analyzerResults PresidioAnalyzerResults) (PresidioAnonymizerResponse, error) {
	// Create a PresidioAnonymizerRequest with default settings
	requestPayload := PresidioAnonymizerRequest{
		Text:            value,
		Anonymizers:     PresidioAnonymizers{},
		AnalyzerResults: analyzerResults,
	}

	// Marshal the request payload into JSON
	jsonPayload, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("Failed to marshal request payload: %v", err)
	}

	// Construct the HTTP request
	url := s.config.AnonymizerEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("Failed to create HTTP request: %v", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Failed to execute HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check for non-OK status codes
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Presidio service returned status code %d", resp.StatusCode)
	}

	// Parse the response
	var presidioAnonymizerResponse PresidioAnonymizerResponse
	err = json.NewDecoder(resp.Body).Decode(&presidioAnonymizerResponse)
	if err != nil {
		return "", err
	}

	return presidioAnonymizerResponse, nil
}

type PresidioAnalyzerRequest struct {
	Text                  string   `json:"text"`
	Language              string   `json:"language"`
	CorrelationID         string   `json:"correlation_id,omitempty"`
	ScoreThreshold        float64  `json:"score_threshold,omitempty"`
	Entities              []string `json:"entities,omitempty"`
	ReturnDecisionProcess bool     `json:"return_decision_process,omitempty"`
	AdHocRecognizers      []string `json:"ad_hoc_recognizers,omitempty"`
	Context               []string `json:"context,omitempty"`
}

type PresidioAnalyzerResponse struct {
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Score      float64 `json:"score"`
	EntityType string  `json:"entity_type"`
}

type PresidioAnonymizerRequest struct {
	Text            string                   `json:"text"`
	Anonymizers     PresidioAnonymizer       `json:"anonymizers,omitempty"`
	AnalyzerResults PresidioAnalyzerResponse `json:"analyzer_results"`
}

type PresidioAnonymizer struct {
	Type string `json:"type"`
}

type PresidioReplaceAnonymizer struct {
	PresidioAnonymizer
	NewValue string `json:"new_value"`
}

type PresidioRedactAnonymizer struct {
	PresidioAnonymizer
}

type PresidioMaskAnonymizer struct {
	PresidioAnonymizer
	MaskingChar string `json:"masking_char"`
	CharsToMask int    `json:"chars_to_mask"`
	FromEnd     bool   `json:"from_end"`
}

type PresidioHashAnonymizer struct {
	PresidioAnonymizer
	HashType string `json:"hash_type"`
}

type PresidioEncryptAnonymizer struct {
	PresidioAnonymizer
	Key string `json:"key"`
}

type PresidioAnonymizerResponse struct {
	Operation  string `json:"operation,omitempty"`
	EntityType string `json:"entity_type"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Text       string `json:"text,omitempty"`
}
