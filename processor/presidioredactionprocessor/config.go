// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package presidioredactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

type Config struct {
	AnalyzerEndpoint   string `mapstructure:"analyzer_endpoint"`
	AnonymizerEndpoint string `mapstructure:"anonymizer_endpoint"`
}
