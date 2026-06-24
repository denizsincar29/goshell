package ui

const appCSS = `
:root {
  --bg: #1e1e1e;
  --bg-panel: #2a2a2a;
  --bg-input: #1a1a1a;
  --fg: #f0f0f0;
  --fg-muted: #b8b8b8;
  --border: #555;
  --accent: #5b9dd9;
  --accent-fg: #ffffff;
  --danger: #e25555;
  --success: #4caf50;
  --focus-ring: #ffd24a;
  --font-size: 16px;
  font-size: var(--font-size);
}

* { box-sizing: border-box; }

html, body {
  margin: 0;
  padding: 0;
  background: var(--bg);
  color: var(--fg);
  font-family: "Segoe UI", Arial, sans-serif;
  line-height: 1.5;
}

.skip-link {
  position: absolute;
  left: -999px;
  top: 0;
  background: var(--accent);
  color: var(--accent-fg);
  padding: 8px 16px;
  z-index: 1000;
}
.skip-link:focus {
  left: 8px;
  top: 8px;
}

.visually-hidden {
  position: absolute !important;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

/* Always-visible, high-contrast focus indicator. Never remove this. */
:focus {
  outline: 3px solid var(--focus-ring);
  outline-offset: 2px;
}
:focus:not(:focus-visible) {
  outline: 3px solid var(--focus-ring);
}

.app-header {
  padding: 12px 16px;
  border-bottom: 2px solid var(--border);
  display: flex;
  align-items: baseline;
  gap: 16px;
}
.app-header h1 {
  margin: 0;
  font-size: 1.4rem;
}
.conn-status {
  margin: 0;
  color: var(--fg-muted);
  font-size: 0.95rem;
}

main {
  padding: 16px;
  max-width: 1200px;
  margin: 0 auto;
}

h2 { font-size: 1.25rem; margin-top: 0; }
h3, h4 { font-size: 1.05rem; }

fieldset {
  border: 1px solid var(--border);
  border-radius: 4px;
  margin-bottom: 16px;
  padding: 12px 16px;
}
legend {
  font-weight: 600;
  padding: 0 6px;
}

.field-row {
  margin-bottom: 10px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-width: 480px;
}
.field-row label {
  font-weight: 500;
}

label {
  display: block;
}

input[type="text"],
input[type="password"],
input[type="number"],
select,
textarea {
  background: var(--bg-input);
  color: var(--fg);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 8px 10px;
  font-size: 1rem;
  font-family: inherit;
  width: 100%;
}
textarea {
  font-family: "Cascadia Code", Consolas, "Courier New", monospace;
  font-size: 0.92rem;
  white-space: pre;
}

.checkbox-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-weight: normal;
  width: auto;
}
.checkbox-label input { width: auto; }

button {
  background: var(--bg-panel);
  color: var(--fg);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 8px 14px;
  font-size: 0.95rem;
  cursor: pointer;
}
button:hover { background: #383838; }
button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
button.primary {
  background: var(--accent);
  color: var(--accent-fg);
  border-color: var(--accent);
  font-weight: 600;
}
button.primary:hover { background: #4a8cc8; }
button.danger {
  border-color: var(--danger);
  color: var(--danger);
}

.button-row, .toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin: 10px 0;
}
.toolbar label {
  width: auto;
  margin-right: 2px;
}
.toolbar input[type="text"] {
  width: auto;
  min-width: 160px;
  flex: 1 1 160px;
}

/* ---- Tabs ---- */
.tab-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  border-bottom: 2px solid var(--border);
  margin-bottom: 0;
}
.tab-btn {
  border-radius: 4px 4px 0 0;
  border-bottom: none;
  background: var(--bg);
}
.tab-btn[aria-selected="true"] {
  background: var(--bg-panel);
  border-bottom: 3px solid var(--accent);
  font-weight: 700;
}
.tab-panel {
  border: 1px solid var(--border);
  border-top: none;
  padding: 16px;
  background: var(--bg-panel);
  border-radius: 0 0 4px 4px;
}

.session-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.session-target {
  font-weight: 600;
}

/* ---- Tables ---- */
.table-wrap {
  overflow-x: auto;
  margin: 10px 0;
  max-height: 50vh;
  border: 1px solid var(--border);
}
table {
  border-collapse: collapse;
  width: 100%;
  background: var(--bg-input);
}
th, td {
  border: 1px solid var(--border);
  padding: 6px 10px;
  text-align: left;
  font-size: 0.92rem;
}
th { background: #333; position: sticky; top: 0; }
.sort-btn {
  background: none;
  border: none;
  padding: 0;
  font-weight: 600;
  color: var(--fg);
  text-decoration: underline;
}
tr[aria-selected="true"] {
  background: #2f4f6b;
  outline: 2px solid var(--accent);
}
tr.row-active { color: var(--success); font-weight: 600; }
tr.row-failed { color: var(--danger); font-weight: 600; }

/* ---- Resource summary ---- */
.res-summary {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 16px;
  background: var(--bg-input);
  padding: 10px 14px;
  border-radius: 4px;
}
.res-summary dt { font-weight: 600; color: var(--fg-muted); }
.res-summary dd { margin: 0; }

/* ---- Progress ---- */
progress {
  width: 100%;
  height: 22px;
  margin: 6px 0;
}

/* ---- Dialogs ---- */
dialog {
  background: var(--bg-panel);
  color: var(--fg);
  border: 2px solid var(--accent);
  border-radius: 6px;
  padding: 20px;
  max-width: 90vw;
}
dialog.dlg-large {
  width: 90vw;
  max-width: 900px;
}
dialog::backdrop {
  background: rgba(0,0,0,0.6);
}
dialog textarea {
  width: 100%;
}
.saved-hosts-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.saved-hosts-list li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px;
  border-bottom: 1px solid var(--border);
  gap: 10px;
}

/* ---- Crontab structured entries ---- */
.cron-entry-list {
  list-style: none;
  padding: 0;
  margin: 10px 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.cron-entry-btn {
  width: 100%;
  text-align: left;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 14px;
  background: var(--bg-input);
}
.cron-entry-btn:hover { background: #242424; }
.cron-entry-title {
  font-weight: 600;
}
.cron-entry-schedule {
  color: var(--fg-muted);
  font-size: 0.88rem;
}
.inline-row {
  display: flex;
  flex-direction: row;
  gap: 16px;
  max-width: none;
}
.inline-row > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1 1 200px;
}
.weekday-checks {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin: 6px 0;
}
.sub-legend {
  font-weight: 600;
  padding: 0;
  border: none;
  font-size: 0.95rem;
}
.hint {
  color: var(--fg-muted);
  font-size: 0.85rem;
  margin: 4px 0 0;
}
.cron-preview {
  background: var(--bg-input);
  padding: 10px 14px;
  border-radius: 4px;
  border: 1px solid var(--border);
  font-weight: 600;
}

/* ---- File browser tree ---- */
.files-layout {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  flex-wrap: wrap;
}
.files-tree-wrap {
  flex: 2 1 420px;
  max-height: 55vh;
  overflow-y: auto;
  border: 1px solid var(--border);
  background: var(--bg-input);
}
.files-tree {
  list-style: none;
  margin: 0;
  padding: 4px;
}
.files-tree ul {
  list-style: none;
  margin: 0;
  padding-left: 1.4em;
}
.tree-node-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  cursor: pointer;
  border-radius: 3px;
}
.tree-node-row:hover { background: #2c2c2c; }
.tree-node-row[aria-selected="true"] {
  background: #2f4f6b;
  outline: 2px solid var(--accent);
}
.tree-twisty {
  display: inline-block;
  width: 1em;
  text-align: center;
  color: var(--fg-muted);
}
.files-details {
  flex: 1 1 260px;
  min-width: 240px;
}

/* Respect reduced motion */
@media (prefers-reduced-motion: reduce) {
  * { transition: none !important; animation: none !important; }
}

/* High contrast mode friendliness: ensure borders always present even if colors are overridden by OS */
@media (prefers-contrast: more) {
  button, input, select, textarea, table, th, td {
    border-width: 2px;
  }
}
`
