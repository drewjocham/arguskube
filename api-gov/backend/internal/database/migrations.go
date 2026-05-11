package database

import "context"

func (db *DB) migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,

		`CREATE TABLE IF NOT EXISTS api_specs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name TEXT NOT NULL,
			version TEXT NOT NULL DEFAULT '1.0.0',
			content JSONB NOT NULL,
			format TEXT NOT NULL DEFAULT 'openapi_3_1',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS endpoints (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			operation_id TEXT NOT NULL DEFAULT '',
			request_body JSONB,
			responses JSONB,
			parameters JSONB,
			security TEXT[] DEFAULT '{}',
			tags TEXT[] DEFAULT '{}',
			embedding vector(768)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_endpoints_spec_id ON endpoints(spec_id)`,
		`CREATE INDEX IF NOT EXISTS idx_endpoints_method_path ON endpoints(method, path)`,

		`CREATE TABLE IF NOT EXISTS drift_reports (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			endpoint_id UUID REFERENCES endpoints(id) ON DELETE CASCADE,
			severity TEXT NOT NULL DEFAULT 'medium',
			category TEXT NOT NULL,
			score REAL NOT NULL DEFAULT 0,
			source TEXT NOT NULL DEFAULT 'observed',
			observed JSONB,
			expected JSONB,
			actual JSONB,
			suggestion TEXT NOT NULL DEFAULT '',
			resolved BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			resolved_at TIMESTAMPTZ
		)`,

		`CREATE INDEX IF NOT EXISTS idx_drift_reports_spec_id ON drift_reports(spec_id)`,
		`CREATE INDEX IF NOT EXISTS idx_drift_reports_severity ON drift_reports(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_drift_reports_resolved ON drift_reports(resolved)`,

		`CREATE TABLE IF NOT EXISTS generated_tests (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			endpoint_id UUID REFERENCES endpoints(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			headers JSONB DEFAULT '{}',
			body JSONB,
			expected_status INT NOT NULL DEFAULT 200,
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_generated_tests_spec_id ON generated_tests(spec_id)`,

		`CREATE TABLE IF NOT EXISTS streams (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			environment TEXT NOT NULL DEFAULT 'production',
			is_canary BOOLEAN NOT NULL DEFAULT FALSE,
			parent_stream_id UUID REFERENCES streams(id),
			traffic_weight INT NOT NULL DEFAULT 100,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS cross_stream_drifts (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			stream_a_id UUID REFERENCES streams(id),
			stream_b_id UUID REFERENCES streams(id),
			method TEXT NOT NULL, path TEXT NOT NULL, metric TEXT NOT NULL,
			value_a DOUBLE PRECISION, value_b DOUBLE PRECISION,
			diff_pct DOUBLE PRECISION, significance DOUBLE PRECISION,
			severity TEXT NOT NULL DEFAULT 'medium',
			detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS alert_history (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			stream_id UUID REFERENCES streams(id),
			category TEXT NOT NULL, field_hash TEXT NOT NULL,
			first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			count INT NOT NULL DEFAULT 1,
			suppressed_until TIMESTAMPTZ
		)`,

		`CREATE TABLE IF NOT EXISTS user_feedback (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			alert_id UUID REFERENCES alert_history(id),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			action TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS investigations (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			alert_id UUID REFERENCES alert_history(id),
			agent_name TEXT NOT NULL DEFAULT 'investigator',
			started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			result TEXT, confidence DOUBLE PRECISION,
			investigation_log JSONB
		)`,

		`CREATE TABLE IF NOT EXISTS anomaly_metrics (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			date DATE NOT NULL,
			total_alerts INT NOT NULL DEFAULT 0,
			true_positives INT NOT NULL DEFAULT 0,
			false_positives INT NOT NULL DEFAULT 0,
			precision DOUBLE PRECISION NOT NULL DEFAULT 0,
			recall DOUBLE PRECISION NOT NULL DEFAULT 0,
			avg_detection_time_s DOUBLE PRECISION DEFAULT 0,
			alert_fatigue_score DOUBLE PRECISION DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS llm_usage (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			spec_id TEXT NOT NULL, agent TEXT NOT NULL, model TEXT NOT NULL,
			prompt_tokens INT NOT NULL DEFAULT 0,
			completion_tokens INT NOT NULL DEFAULT 0,
			total_tokens INT NOT NULL DEFAULT 0,
			duration_ms INT NOT NULL DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS strategy_experiments (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			spec_id UUID NOT NULL REFERENCES api_specs(id) ON DELETE CASCADE,
			strategy_name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			ended_at TIMESTAMPTZ,
			baseline_fp_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
			baseline_detection_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
			result_fp_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
			result_detection_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
			improvement_pct DOUBLE PRECISION DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'running'
		)`,

		`CREATE TABLE IF NOT EXISTS user_profiles (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id TEXT NOT NULL UNIQUE, skill_level TEXT NOT NULL DEFAULT 'intermediate',
			session_count INT NOT NULL DEFAULT 0,
			common_issues TEXT[] DEFAULT '{}',
			last_active TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			metadata JSONB DEFAULT '{}'
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Pool.Exec(ctx, m); err != nil {
			return err
		}
	}
	return nil
}
