package jobtracker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AppServer manages the in-memory applications cache and HTTP server.
type AppServer struct {
	dir          string
	port         int
	mu           sync.RWMutex
	applications []*Application
	summary      *ApplicationsSummary
}

// NewAppServer initializes the server with data loaded into memory.
func NewAppServer(dir string, port int) (*AppServer, error) {
	if port <= 0 {
		port = 8085
	}
	s := &AppServer{
		dir:  dir,
		port: port,
	}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

// Reload reads all application JSON files from disk into memory.
func (s *AppServer) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("failed to read applications directory %s: %w", s.dir, err)
	}

	var apps []*Application
	statusCounts := make(map[string]int)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "applications_summary.json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}

		var app Application
		if err := json.Unmarshal(data, &app); err != nil {
			continue
		}

		statusCounts[app.CurrentStatus]++
		apps = append(apps, &app)
	}

	// Sort strictly by latest update on top (descending)
	sort.SliceStable(apps, func(i, j int) bool {
		if apps[i].LastUpdatedDate.Equal(apps[j].LastUpdatedDate) {
			return apps[i].FirstAppliedDate.After(apps[j].FirstAppliedDate)
		}
		return apps[i].LastUpdatedDate.After(apps[j].LastUpdatedDate)
	})

	s.applications = apps
	s.summary = &ApplicationsSummary{
		GeneratedAt:       time.Now().UTC(),
		TotalApplications: len(apps),
		StatusCounts:      statusCounts,
	}

	return nil
}

// Start launches the HTTP server.
func (s *AppServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/applications", s.handleAPI)
	mux.HandleFunc("/api/reload", s.handleReload)

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("\n✨ Job Application Tracker UI is running at: http://localhost:%d\n", s.port)
	fmt.Printf("📊 Loaded %d applications into memory (sorted by latest update on top).\n", len(s.applications))
	fmt.Println("Press Ctrl+C to stop.")

	return http.ListenAndServe(addr, mux)
}

func (s *AppServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.applications)
}

func (s *AppServer) handleReload(w http.ResponseWriter, r *http.Request) {
	if err := s.Reload(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.RLock()
	count := len(s.applications)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "reloaded",
		"count":  count,
	})
}

func (s *AppServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	appsJSON, err := json.Marshal(s.applications)
	summaryJSON, _ := json.Marshal(s.summary)
	s.mu.RUnlock()

	if err != nil {
		http.Error(w, "Failed to marshal data", http.StatusInternalServerError)
		return
	}

	html := renderDashboardHTML(string(appsJSON), string(summaryJSON))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func renderDashboardHTML(appsJSON, summaryJSON string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Job Applications Tracker</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg-main: #0b0f19;
      --bg-card: #111827;
      --bg-card-hover: #162032;
      --border-color: rgba(255, 255, 255, 0.08);
      --border-hover: rgba(255, 255, 255, 0.16);
      --text-main: #f1f5f9;
      --text-muted: #94a3b8;
      --text-dim: #64748b;
      --color-primary: #6366f1;
      --color-offer: #10b981;
      --color-interview: #8b5cf6;
      --color-review: #f59e0b;
      --color-applied: #38bdf8;
      --color-referred: #ec4899;
      --color-rejected: #f43f5e;
      --color-comm: #64748b;
      --font-main: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
      --font-mono: 'JetBrains Mono', monospace;
    }

    * { box-sizing: border-box; margin: 0; padding: 0; }

    body {
      background-color: var(--bg-main);
      color: var(--text-main);
      font-family: var(--font-main);
      line-height: 1.5;
      padding: 24px 32px;
      min-height: 100vh;
    }

    /* Container */
    .app-container {
      max-width: 1600px;
      margin: 0 auto;
    }

    /* Header */
    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 24px;
      flex-wrap: wrap;
      gap: 16px;
    }

    .brand {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .brand-icon {
      width: 42px;
      height: 42px;
      border-radius: 12px;
      background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 22px;
      box-shadow: 0 4px 16px rgba(99, 102, 241, 0.3);
    }

    .brand-title {
      font-size: 24px;
      font-weight: 700;
      letter-spacing: -0.02em;
      color: #fff;
    }

    .brand-subtitle {
      font-size: 13px;
      color: var(--text-muted);
    }

    /* Metrics bar */
    .metrics-bar {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
      gap: 14px;
      margin-bottom: 24px;
    }

    .metric-card {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: 14px;
      padding: 16px 20px;
      display: flex;
      flex-direction: column;
      gap: 6px;
      transition: all 0.2s ease;
    }

    .metric-card:hover {
      border-color: var(--border-hover);
      transform: translateY(-1px);
    }

    .metric-label {
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      font-weight: 600;
      color: var(--text-muted);
    }

    .metric-value {
      font-size: 26px;
      font-weight: 700;
      color: #fff;
      display: flex;
      align-items: baseline;
      gap: 6px;
    }

    /* Controls: Search and Filters */
    .controls-bar {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: 14px;
      padding: 16px 20px;
      margin-bottom: 24px;
      display: flex;
      flex-wrap: wrap;
      gap: 16px;
      align-items: center;
      justify-content: space-between;
    }

    .search-input-wrapper {
      position: relative;
      flex: 1;
      min-width: 280px;
      max-width: 480px;
    }

    .search-input-wrapper svg {
      position: absolute;
      left: 14px;
      top: 50%;
      transform: translateY(-50%);
      color: var(--text-dim);
      pointer-events: none;
    }

    .search-input {
      width: 100%;
      background: rgba(0, 0, 0, 0.3);
      border: 1px solid var(--border-color);
      border-radius: 10px;
      padding: 10px 14px 10px 40px;
      color: #fff;
      font-size: 14px;
      outline: none;
      transition: border-color 0.2s;
    }

    .search-input:focus {
      border-color: var(--color-primary);
      box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.2);
    }

    .filter-pills {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }

    .pill-btn {
      background: rgba(255, 255, 255, 0.04);
      border: 1px solid var(--border-color);
      color: var(--text-muted);
      padding: 6px 14px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 500;
      cursor: pointer;
      transition: all 0.15s ease;
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .pill-btn:hover {
      background: rgba(255, 255, 255, 0.08);
      color: #fff;
    }

    .pill-btn.active {
      background: var(--color-primary);
      border-color: var(--color-primary);
      color: #fff;
      font-weight: 600;
      box-shadow: 0 2px 10px rgba(99, 102, 241, 0.35);
    }

    .pill-badge {
      font-size: 11px;
      padding: 2px 6px;
      border-radius: 10px;
      background: rgba(0, 0, 0, 0.3);
    }

    /* Table */
    .table-container {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: 16px;
      overflow: hidden;
      box-shadow: 0 8px 30px rgba(0, 0, 0, 0.4);
    }

    table {
      width: 100%;
      border-collapse: collapse;
      table-layout: fixed;
      text-align: left;
    }

    thead {
      background: rgba(255, 255, 255, 0.02);
      border-bottom: 1px solid var(--border-color);
    }

    th {
      padding: 16px 20px;
      font-size: 12px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--text-dim);
    }

    th.col-company { width: 18%; }
    th.col-role { width: 27%; }
    th.col-timeline { width: 55%; }

    tbody tr {
      border-bottom: 1px solid var(--border-color);
      transition: background-color 0.15s ease;
    }

    tbody tr:last-child {
      border-bottom: none;
    }

    tbody tr:hover {
      background-color: var(--bg-card-hover);
    }

    td {
      padding: 18px 20px;
      vertical-align: middle;
    }

    /* Company Cell */
    .company-cell {
      display: flex;
      align-items: center;
      gap: 14px;
    }

    .company-avatar {
      width: 40px;
      height: 40px;
      border-radius: 10px;
      background: linear-gradient(135deg, rgba(99, 102, 241, 0.2) 0%, rgba(168, 85, 247, 0.2) 100%);
      border: 1px solid rgba(255, 255, 255, 0.1);
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: 700;
      font-size: 16px;
      color: #e2e8f0;
      flex-shrink: 0;
    }

    .company-info {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    .company-name {
      font-weight: 600;
      font-size: 15px;
      color: #fff;
    }

    .company-emails-count {
      font-size: 12px;
      color: var(--text-dim);
    }

    /* Role Cell */
    .role-info {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    .role-title {
      font-weight: 600;
      font-size: 14px;
      color: #f1f5f9;
      line-height: 1.35;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }

    .role-badges {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
    }

    .tag-badge {
      font-size: 11px;
      font-family: var(--font-mono);
      padding: 2px 7px;
      border-radius: 5px;
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid rgba(255, 255, 255, 0.07);
      color: var(--text-muted);
    }

    .status-tag {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      padding: 3px 8px;
      border-radius: 6px;
    }

    .status-OFFER { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
    .status-INTERVIEW { background: rgba(139, 92, 246, 0.15); color: #a78bfa; border: 1px solid rgba(139, 92, 246, 0.3); }
    .status-UNDER_REVIEW { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
    .status-APPLIED { background: rgba(56, 189, 248, 0.15); color: #38bdf8; border: 1px solid rgba(56, 189, 248, 0.3); }
    .status-REFERRED { background: rgba(236, 72, 153, 0.15); color: #f472b6; border: 1px solid rgba(236, 72, 153, 0.3); }
    .status-REJECTED { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
    .status-COMMUNICATION { background: rgba(100, 116, 139, 0.15); color: #94a3b8; border: 1px solid rgba(100, 116, 139, 0.3); }

    td.col-timeline-cell {
      max-width: 0;
      width: 55%;
      overflow: hidden;
      padding: 10px 16px;
    }

    /* Timeline Cell - Self-contained horizontal scroll */
    .timeline-container {
      position: relative;
      padding: 16px 12px 14px 12px;
      overflow-x: auto;
      overflow-y: hidden;
      white-space: nowrap;
      scroll-behavior: smooth;
      -webkit-overflow-scrolling: touch;
    }

    .timeline-container::-webkit-scrollbar {
      height: 6px;
    }

    .timeline-container::-webkit-scrollbar-track {
      background: rgba(255, 255, 255, 0.04);
      border-radius: 6px;
    }

    .timeline-container::-webkit-scrollbar-thumb {
      background: rgba(99, 102, 241, 0.4);
      border-radius: 6px;
      transition: background 0.2s;
    }

    .timeline-container::-webkit-scrollbar-thumb:hover {
      background: rgba(99, 102, 241, 0.8);
    }

    .timeline-track {
      display: flex;
      align-items: flex-start;
      position: relative;
      min-width: max-content;
      gap: 36px;
    }

    /* Straight line connecting the dots */
    .timeline-track::before {
      content: '';
      position: absolute;
      top: 10px;
      left: 12px;
      right: 12px;
      height: 2px;
      background: linear-gradient(90deg, rgba(255,255,255,0.15) 0%, rgba(255,255,255,0.25) 50%, rgba(255,255,255,0.15) 100%);
      z-index: 1;
    }

    .timeline-node {
      display: flex;
      flex-direction: column;
      align-items: center;
      position: relative;
      z-index: 2;
      min-width: 110px;
      cursor: pointer;
    }

    /* Dot on the straight line */
    .timeline-dot {
      width: 20px;
      height: 20px;
      border-radius: 50%;
      background: var(--bg-card);
      border: 3px solid var(--text-dim);
      display: flex;
      align-items: center;
      justify-content: center;
      transition: all 0.2s ease;
      position: relative;
    }

    .timeline-node:hover .timeline-dot {
      transform: scale(1.3);
      box-shadow: 0 0 12px currentColor;
    }

    .dot-OFFER { border-color: var(--color-offer); color: var(--color-offer); background: #064e3b; }
    .dot-INTERVIEW { border-color: var(--color-interview); color: var(--color-interview); background: #3b0764; }
    .dot-UNDER_REVIEW { border-color: var(--color-review); color: var(--color-review); background: #451a03; }
    .dot-APPLIED { border-color: var(--color-applied); color: var(--color-applied); background: #082f49; }
    .dot-REFERRED { border-color: var(--color-referred); color: var(--color-referred); background: #500724; }
    .dot-REJECTED { border-color: var(--color-rejected); color: var(--color-rejected); background: #4c0519; }
    .dot-COMMUNICATION { border-color: var(--color-comm); color: var(--color-comm); background: #1e293b; }

    /* Content under the dot */
    .timeline-content {
      margin-top: 10px;
      display: flex;
      flex-direction: column;
      align-items: center;
      text-align: center;
      gap: 3px;
    }

    .timeline-status {
      font-size: 12px;
      font-weight: 600;
      color: #e2e8f0;
      white-space: nowrap;
    }

    .timeline-date {
      font-size: 11px;
      font-family: var(--font-mono);
      color: var(--text-muted);
      white-space: nowrap;
    }

    .timeline-time {
      font-size: 10px;
      font-family: var(--font-mono);
      color: var(--text-dim);
      white-space: nowrap;
    }

    /* Tooltip popup for email details */
    .timeline-node .tooltip {
      position: absolute;
      bottom: calc(100% + 12px);
      left: 50%;
      transform: translateX(-50%) translateY(6px);
      background: #1e293b;
      border: 1px solid rgba(255, 255, 255, 0.15);
      border-radius: 8px;
      padding: 10px 14px;
      font-size: 12px;
      color: #f1f5f9;
      width: 260px;
      box-shadow: 0 8px 24px rgba(0, 0, 0, 0.6);
      pointer-events: none;
      opacity: 0;
      visibility: hidden;
      transition: all 0.2s ease;
      z-index: 100;
      text-align: left;
    }

    .timeline-node .tooltip::after {
      content: '';
      position: absolute;
      top: 100%;
      left: 50%;
      transform: translateX(-50%);
      border: 6px solid transparent;
      border-top-color: #1e293b;
    }

    .timeline-node:hover .tooltip {
      opacity: 1;
      visibility: visible;
      transform: translateX(-50%) translateY(0);
    }

    .tooltip-subject {
      font-weight: 600;
      color: #fff;
      margin-bottom: 4px;
      line-height: 1.3;
    }

    .tooltip-snippet {
      color: var(--text-muted);
      font-size: 11px;
      line-height: 1.4;
      display: -webkit-box;
      -webkit-line-clamp: 3;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }

    /* Pagination / Results info */
    .pagination-bar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px 20px;
      background: rgba(255, 255, 255, 0.02);
      border-top: 1px solid var(--border-color);
      flex-wrap: wrap;
      gap: 12px;
    }

    .results-count {
      font-size: 13px;
      color: var(--text-muted);
    }

    .page-nav {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .page-btn {
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid var(--border-color);
      color: var(--text-main);
      padding: 6px 12px;
      border-radius: 8px;
      font-size: 13px;
      cursor: pointer;
      transition: background 0.15s;
    }

    .page-btn:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.12);
    }

    .page-btn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    .page-indicator {
      font-size: 13px;
      color: var(--text-muted);
      font-family: var(--font-mono);
    }

    /* Empty state */
    .empty-state {
      padding: 60px 20px;
      text-align: center;
      color: var(--text-dim);
    }
  </style>
</head>
<body>
  <div class="app-container">
    <header>
      <div class="brand">
        <div class="brand-icon">📬</div>
        <div>
          <h1 class="brand-title">Job Application Tracker</h1>
          <p class="brand-subtitle">Chronological timeline of active & past job opportunities</p>
        </div>
      </div>
      <div>
        <button id="reloadBtn" class="pill-btn" onclick="reloadApplications()">
          🔄 Reload From Disk
        </button>
      </div>
    </header>

    <!-- Metrics Summary Cards -->
    <section class="metrics-bar" id="metricsBar"></section>

    <!-- Controls: Search & Status Filters -->
    <section class="controls-bar">
      <div class="search-input-wrapper">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"></circle>
          <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
        </svg>
        <input type="text" id="searchInput" class="search-input" placeholder="Search by company, role, team, or status..." oninput="handleSearch()">
      </div>

      <div class="filter-pills" id="statusFilterPills"></div>
    </section>

    <!-- Main Table -->
    <main class="table-container">
      <table>
        <thead>
          <tr>
            <th class="col-company">Company</th>
            <th class="col-role">Role & Details</th>
            <th class="col-timeline">Status Timeline (Chronological)</th>
          </tr>
        </thead>
        <tbody id="applicationsTableBody"></tbody>
      </table>

      <!-- Pagination bar -->
      <div class="pagination-bar">
        <div class="results-count" id="resultsCount">Showing 0 applications</div>
        <div class="page-nav">
          <button id="prevBtn" class="page-btn" onclick="prevPage()">← Previous</button>
          <span id="pageIndicator" class="page-indicator">Page 1 / 1</span>
          <button id="nextBtn" class="page-btn" onclick="nextPage()">Next →</button>
        </div>
      </div>
    </main>
  </div>

  <script id="data-apps" type="application/json">` + appsJSON + `</script>
  <script id="data-summary" type="application/json">` + summaryJSON + `</script>

  <script>
    const allApps = JSON.parse(document.getElementById('data-apps').textContent || '[]');
    const summary = JSON.parse(document.getElementById('data-summary').textContent || '{}');

    let filteredApps = [...allApps];
    let activeFilter = 'ALL';
    let searchQuery = '';
    let currentPage = 1;
    const pageSize = 50;

    // Format human-friendly status label
    function formatStatus(status) {
      const map = {
        'OFFER': 'Offer Received 🎉',
        'INTERVIEW': 'Interview / Screen 💬',
        'UNDER_REVIEW': 'Under Review ⏳',
        'APPLIED': 'Applied',
        'REFERRED': 'Referred 🤝',
        'REJECTED': 'Not Moving Forward 🚫',
        'WITHDRAWN': 'Withdrawn ↩️',
        'COMMUNICATION': 'Correspondence ✉️'
      };
      return map[status] || status;
    }

    function formatDate(isoStr) {
      if (!isoStr) return '';
      const d = new Date(isoStr);
      return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    }

    function formatTime(isoStr) {
      if (!isoStr) return '';
      const d = new Date(isoStr);
      return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
    }

    // Render Metrics
    function renderMetrics() {
      const bar = document.getElementById('metricsBar');
      const counts = summary.status_counts || {};

      const cards = [
        { label: 'Total Tracked', val: allApps.length, color: '#6366f1' },
        { label: 'Interviews', val: counts['INTERVIEW'] || 0, color: 'var(--color-interview)' },
        { label: 'Offers', val: counts['OFFER'] || 0, color: 'var(--color-offer)' },
        { label: 'Under Review', val: counts['UNDER_REVIEW'] || 0, color: 'var(--color-review)' },
        { label: 'Applied', val: counts['APPLIED'] || 0, color: 'var(--color-applied)' },
        { label: 'Referred', val: counts['REFERRED'] || 0, color: 'var(--color-referred)' },
        { label: 'Not Moving Forward', val: counts['REJECTED'] || 0, color: 'var(--color-rejected)' },
      ];

      bar.innerHTML = cards.map(c => ` + "`" + `
        <div class="metric-card">
          <span class="metric-label">${c.label}</span>
          <span class="metric-value" style="color: ${c.color}">${c.val}</span>
        </div>
      ` + "`" + `).join('');
    }

    // Render Filter Pills
    function renderFilterPills() {
      const pillContainer = document.getElementById('statusFilterPills');
      const counts = summary.status_counts || {};

      const filters = [
        { id: 'ALL', label: 'All', count: allApps.length },
        { id: 'INTERVIEW', label: 'Interviews', count: counts['INTERVIEW'] || 0 },
        { id: 'OFFER', label: 'Offers', count: counts['OFFER'] || 0 },
        { id: 'UNDER_REVIEW', label: 'Under Review', count: counts['UNDER_REVIEW'] || 0 },
        { id: 'APPLIED', label: 'Applied', count: counts['APPLIED'] || 0 },
        { id: 'REFERRED', label: 'Referred', count: counts['REFERRED'] || 0 },
        { id: 'REJECTED', label: 'Rejected', count: counts['REJECTED'] || 0 },
      ];

      pillContainer.innerHTML = filters.map(f => ` + "`" + `
        <button class="pill-btn ${activeFilter === f.id ? 'active' : ''}" onclick="setFilter('${f.id}')">
          ${f.label}
          <span class="pill-badge">${f.count}</span>
        </button>
      ` + "`" + `).join('');
    }

    function setFilter(filterId) {
      activeFilter = filterId;
      currentPage = 1;
      applyFilters();
      renderFilterPills();
    }

    function handleSearch() {
      searchQuery = document.getElementById('searchInput').value.toLowerCase().trim();
      currentPage = 1;
      applyFilters();
    }

    function applyFilters() {
      filteredApps = allApps.filter(app => {
        const matchesStatus = (activeFilter === 'ALL') || (app.current_status === activeFilter);
        if (!matchesStatus) return false;

        if (!searchQuery) return true;

        const hay = (
          (app.company || '') + ' ' +
          (app.role || '') + ' ' +
          (app.team || '') + ' ' +
          (app.job_id || '') + ' ' +
          (app.current_status || '')
        ).toLowerCase();

        return hay.includes(searchQuery);
      });

      renderTable();
    }

    // Render Table
    function renderTable() {
      const tbody = document.getElementById('applicationsTableBody');
      const total = filteredApps.length;
      const totalPages = Math.ceil(total / pageSize) || 1;

      if (currentPage > totalPages) currentPage = totalPages;
      if (currentPage < 1) currentPage = 1;

      const start = (currentPage - 1) * pageSize;
      const pageApps = filteredApps.slice(start, start + pageSize);

      document.getElementById('resultsCount').textContent = ` + "`" + `Showing ${total === 0 ? 0 : start + 1}–${Math.min(start + pageSize, total)} of ${total} applications` + "`" + `;
      document.getElementById('pageIndicator').textContent = ` + "`" + `Page ${currentPage} / ${totalPages}` + "`" + `;
      document.getElementById('prevBtn').disabled = currentPage <= 1;
      document.getElementById('nextBtn').disabled = currentPage >= totalPages;

      if (pageApps.length === 0) {
        tbody.innerHTML = ` + "`" + `
          <tr>
            <td colspan="3" class="empty-state">
              No applications match your search or filter criteria.
            </td>
          </tr>
        ` + "`" + `;
        return;
      }

      tbody.innerHTML = pageApps.map(app => {
        const initial = (app.company || 'C').charAt(0).toUpperCase();
        const timeline = app.timeline || [];

        return ` + "`" + `
          <tr>
            <!-- Column 1: Company -->
            <td>
              <div class="company-cell">
                <div class="company-avatar">${initial}</div>
                <div class="company-info">
                  <span class="company-name">${escapeHTML(app.company || 'Unknown')}</span>
                  <span class="company-emails-count">${app.total_emails} email${app.total_emails === 1 ? '' : 's'}</span>
                </div>
              </div>
            </td>

            <!-- Column 2: Role & Details -->
            <td>
              <div class="role-info">
                <div class="role-title" title="${escapeHTML(app.role || 'Position')}">${escapeHTML(app.role || 'Position')}</div>
                <div class="role-badges">
                  <span class="status-tag status-${app.current_status}">${app.current_status}</span>
                  ${app.team ? ` + "`" + `<span class="tag-badge">👥 ${escapeHTML(app.team)}</span>` + "`" + ` : ''}
                  ${app.job_id ? ` + "`" + `<span class="tag-badge"># ${escapeHTML(app.job_id)}</span>` + "`" + ` : ''}
                </div>
              </div>
            </td>

            <!-- Column 3: Timeline of Events (Straight line with dots, self-contained horizontal scroll) -->
            <td class="col-timeline-cell">
              <div class="timeline-container">
                <div class="timeline-track">
                  ${timeline.map(t => ` + "`" + `
                    <div class="timeline-node">
                      <!-- Hover Tooltip -->
                      <div class="tooltip">
                        <div class="tooltip-subject">${escapeHTML(t.subject || 'Email')}</div>
                        ${t.snippet ? ` + "`" + `<div class="tooltip-snippet">${escapeHTML(t.snippet)}</div>` + "`" + ` : ''}
                      </div>

                      <!-- Dot on the straight line -->
                      <div class="timeline-dot dot-${t.status}"></div>

                      <!-- Under the dot: Status, Date, Time -->
                      <div class="timeline-content">
                        <span class="timeline-status">${formatStatus(t.status)}</span>
                        <span class="timeline-date">${formatDate(t.date)}</span>
                        <span class="timeline-time">${formatTime(t.date)}</span>
                      </div>
                    </div>
                  ` + "`" + `).join('')}
                </div>
              </div>
            </td>
          </tr>
        ` + "`" + `;
      }).join('');
    }

    function escapeHTML(str) {
      if (!str) return '';
      return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
    }

    function prevPage() {
      if (currentPage > 1) {
        currentPage--;
        renderTable();
        window.scrollTo({ top: 0, behavior: 'smooth' });
      }
    }

    function nextPage() {
      const totalPages = Math.ceil(filteredApps.length / pageSize);
      if (currentPage < totalPages) {
        currentPage++;
        renderTable();
        window.scrollTo({ top: 0, behavior: 'smooth' });
      }
    }

    function reloadApplications() {
      const btn = document.getElementById('reloadBtn');
      btn.textContent = '⏳ Reloading...';
      btn.disabled = true;

      fetch('/api/reload')
        .then(res => res.json())
        .then(() => {
          window.location.reload();
        })
        .catch(err => {
          alert('Failed to reload: ' + err);
          btn.textContent = '🔄 Reload From Disk';
          btn.disabled = false;
        });
    }

    // Horizontal wheel scroll support for timeline cells
    document.addEventListener('wheel', (e) => {
      const container = e.target.closest('.timeline-container');
      if (container && container.scrollWidth > container.clientWidth) {
        if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
          container.scrollLeft += e.deltaY;
          e.preventDefault();
        }
      }
    }, { passive: false });

    // Initialize
    renderMetrics();
    renderFilterPills();
    renderTable();
  </script>
</body>
</html>`
}
