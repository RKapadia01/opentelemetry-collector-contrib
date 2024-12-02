// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package presidioredactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/presidioredactionprocessor/internal/metadata"
)

// NewFactory creates a factory for the redaction processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

// createTracesProcessor creates an instance of redaction for processing traces
func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)

	presidioRedaction := newPresidioRedaction(ctx, oCfg, set.Logger)

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		next,
		presidioRedaction.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)

	presidioRedaction := newPresidioRedaction(ctx, oCfg, set.Logger)

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		next,
		presidioRedaction.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}
