-- DuckDB dry-run over GTM / MCP telemetry fixtures.
-- duckdb -c ".read docs/gtm/queries/funnel.sql"

CREATE OR REPLACE TABLE events AS
SELECT * FROM read_json_auto('docs/gtm/fixtures/activation-events.sample.jsonl', format='newline_delimited');

SELECT step, count(*) AS n
FROM events
WHERE event = 'activation_step'
GROUP BY 1
ORDER BY 1;

SELECT tool,
       count(*) AS calls,
       count(*) FILTER (WHERE outcome = 'success') AS success,
       count(*) FILTER (WHERE outcome = 'policy_fail') AS policy_fail,
       count(*) FILTER (WHERE outcome = 'rejected') AS rejected
FROM events
WHERE tool IS NOT NULL
GROUP BY 1
ORDER BY calls DESC;
