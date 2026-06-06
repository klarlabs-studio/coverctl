package report

import (
	"html/template"
	"io"
	"time"

	"go.klarlabs.de/coverctl/internal/domain"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Coverage Report</title>
    <style>
        :root {
            --pass: #16A34A;
            --fail: #DC2626;
            --warn: #CA8A04;
            --bg: #0f172a;
            --card: #1e293b;
            --text: #f8fafc;
            --muted: #94a3b8;
            --border: #334155;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            padding: 2rem;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 {
            font-size: 2rem;
            margin-bottom: 0.5rem;
            font-weight: 600;
        }
        .timestamp {
            color: var(--muted);
            font-size: 0.875rem;
            margin-bottom: 2rem;
        }
        .summary {
            display: flex;
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .summary-card {
            background: var(--card);
            border-radius: 0.5rem;
            padding: 1rem 1.5rem;
            border: 1px solid var(--border);
        }
        .summary-card.pass { border-left: 4px solid var(--pass); }
        .summary-card.fail { border-left: 4px solid var(--fail); }
        .summary-label {
            font-size: 0.75rem;
            text-transform: uppercase;
            color: var(--muted);
            letter-spacing: 0.05em;
        }
        .summary-value {
            font-size: 1.5rem;
            font-weight: 600;
        }
        .summary-value.pass { color: var(--pass); }
        .summary-value.fail { color: var(--fail); }
        table {
            width: 100%;
            border-collapse: collapse;
            background: var(--card);
            border-radius: 0.5rem;
            overflow: hidden;
            margin-bottom: 2rem;
        }
        th, td {
            padding: 0.75rem 1rem;
            text-align: left;
            border-bottom: 1px solid var(--border);
        }
        th {
            background: rgba(0,0,0,0.2);
            font-weight: 600;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--muted);
        }
        tr:last-child td { border-bottom: none; }
        tr:hover { background: rgba(255,255,255,0.02); }
        .status {
            display: inline-block;
            padding: 0.25rem 0.5rem;
            border-radius: 0.25rem;
            font-size: 0.75rem;
            font-weight: 600;
        }
        .status.pass { background: rgba(22, 163, 74, 0.2); color: var(--pass); }
        .status.fail { background: rgba(220, 38, 38, 0.2); color: var(--fail); }
        .status.warn { background: rgba(202, 138, 4, 0.2); color: var(--warn); }
        .progress-bar {
            width: 100%;
            height: 6px;
            background: var(--border);
            border-radius: 3px;
            overflow: hidden;
        }
        .progress-fill {
            height: 100%;
            border-radius: 3px;
            transition: width 0.3s ease;
        }
        .progress-fill.pass { background: var(--pass); }
        .progress-fill.fail { background: var(--fail); }
        .coverage-cell {
            display: flex;
            align-items: center;
            gap: 0.75rem;
        }
        .coverage-percent {
            min-width: 4rem;
            font-weight: 500;
        }
        .section-title {
            font-size: 1.25rem;
            margin-bottom: 1rem;
            font-weight: 600;
        }
        .warnings {
            background: rgba(202, 138, 4, 0.1);
            border: 1px solid rgba(202, 138, 4, 0.3);
            border-radius: 0.5rem;
            padding: 1rem;
            margin-bottom: 2rem;
        }
        .warnings h3 {
            color: var(--warn);
            font-size: 0.875rem;
            text-transform: uppercase;
            margin-bottom: 0.5rem;
        }
        .warnings ul {
            list-style: none;
            color: var(--muted);
        }
        .warnings li::before {
            content: "⚠ ";
            color: var(--warn);
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Coverage Report</h1>
        <p class="timestamp">Generated {{.Timestamp}}</p>

        <div class="summary">
            <div class="summary-card {{if .Passed}}pass{{else}}fail{{end}}">
                <div class="summary-label">Status</div>
                <div class="summary-value {{if .Passed}}pass{{else}}fail{{end}}">
                    {{if .Passed}}PASS{{else}}FAIL{{end}}
                </div>
            </div>
            <div class="summary-card">
                <div class="summary-label">Domains</div>
                <div class="summary-value">{{len .Domains}}</div>
            </div>
            {{if .Files}}
            <div class="summary-card">
                <div class="summary-label">File Rules</div>
                <div class="summary-value">{{len .Files}}</div>
            </div>
            {{end}}
        </div>

        {{if .Warnings}}
        <div class="warnings">
            <h3>Warnings</h3>
            <ul>
                {{range .Warnings}}
                <li>{{.}}</li>
                {{end}}
            </ul>
        </div>
        {{end}}

        {{if .Domains}}
        <h2 class="section-title">Domain Coverage</h2>
        <table>
            <thead>
                <tr>
                    <th>Domain</th>
                    <th>Coverage</th>
                    <th>Required</th>
                    <th>Status</th>
                </tr>
            </thead>
            <tbody>
                {{range .Domains}}
                <tr>
                    <td>{{.Domain}}</td>
                    <td>
                        <div class="coverage-cell">
                            <span class="coverage-percent">{{printf "%.1f" .Percent}}%</span>
                            <div class="progress-bar">
                                <div class="progress-fill {{if eq .Status "PASS"}}pass{{else}}fail{{end}}"
                                     style="width: {{if gt .Percent 100.0}}100{{else}}{{printf "%.0f" .Percent}}{{end}}%"></div>
                            </div>
                        </div>
                    </td>
                    <td>{{printf "%.1f" .Required}}%</td>
                    <td><span class="status {{if eq .Status "PASS"}}pass{{else}}fail{{end}}">{{.Status}}</span></td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}

        {{if .Files}}
        <h2 class="section-title">File Rules</h2>
        <table>
            <thead>
                <tr>
                    <th>File</th>
                    <th>Coverage</th>
                    <th>Required</th>
                    <th>Status</th>
                </tr>
            </thead>
            <tbody>
                {{range .Files}}
                <tr>
                    <td>{{.File}}</td>
                    <td>
                        <div class="coverage-cell">
                            <span class="coverage-percent">{{printf "%.1f" .Percent}}%</span>
                            <div class="progress-bar">
                                <div class="progress-fill {{if eq .Status "PASS"}}pass{{else}}fail{{end}}"
                                     style="width: {{if gt .Percent 100.0}}100{{else}}{{printf "%.0f" .Percent}}{{end}}%"></div>
                            </div>
                        </div>
                    </td>
                    <td>{{printf "%.1f" .Required}}%</td>
                    <td><span class="status {{if eq .Status "PASS"}}pass{{else}}fail{{end}}">{{.Status}}</span></td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}
    </div>
</body>
</html>`

type htmlData struct {
	domain.Result
	Timestamp string
}

func writeHTML(w io.Writer, result domain.Result) error {
	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return err
	}
	data := htmlData{
		Result:    result,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}
	return tmpl.Execute(w, data)
}
