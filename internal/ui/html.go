package ui

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>GoShell — Accessible SSH Manager</title>
<link rel="stylesheet" href="/static/app.css">
</head>
<body>

<a class="skip-link" href="#main-content">Skip to main content</a>

<header class="app-header">
  <h1>GoShell</h1>
  <p id="connection-status" role="status" aria-live="polite" class="conn-status">Not connected</p>
</header>

<div id="status-live" role="status" aria-live="polite" class="visually-hidden"></div>
<div id="alert-live" role="alert" aria-live="assertive" class="visually-hidden"></div>

<main id="main-content">

  <!-- ===================== CONNECT SCREEN ===================== -->
  <section id="screen-connect" aria-labelledby="connect-heading">
    <h2 id="connect-heading">Connect to a server</h2>

    <form id="connect-form" autocomplete="off">

      <fieldset>
        <legend>Known hosts</legend>

        <label for="known-hosts-select">Choose a saved or configured host</label>
        <select id="known-hosts-select" name="known_host">
          <option value="">— manual entry —</option>
        </select>
      </fieldset>

      <fieldset>
        <legend>Connection details</legend>

        <div class="field-row">
          <label for="field-hostname">Hostname or IP</label>
          <input type="text" id="field-hostname" name="hostname" placeholder="example.com or 192.168.1.1" required>
        </div>

        <div class="field-row">
          <label for="field-port">Port</label>
          <input type="text" id="field-port" name="port" value="22" inputmode="numeric">
        </div>

        <div class="field-row">
          <label for="field-user">Username</label>
          <input type="text" id="field-user" name="user" placeholder="root">
        </div>

        <div class="field-row">
          <label for="field-key">Private key file</label>
          <select id="field-key" name="key_file">
            <option value="">— none / try all keys —</option>
          </select>
        </div>

        <div class="field-row">
          <label for="field-password">Password (optional, only if not using a key)</label>
          <input type="password" id="field-password" name="password" autocomplete="off">
        </div>
      </fieldset>

      <fieldset>
        <legend>Sudo</legend>
        <div class="field-row">
          <label for="field-sudo">Sudo password (leave blank if passwordless sudo)</label>
          <input type="password" id="field-sudo" name="sudo_pass" autocomplete="off">
        </div>
      </fieldset>

      <div class="button-row">
        <button type="button" id="btn-open-settings">Settings</button>
        <button type="button" id="btn-save-host">Save this host</button>
        <button type="button" id="btn-manage-hosts">Manage saved hosts</button>
        <button type="submit" id="btn-connect" class="primary">Connect</button>
      </div>
    </form>
  </section>

  <!-- ===================== SESSION SCREEN ===================== -->
  <section id="screen-session" hidden aria-labelledby="session-heading">
    <h2 id="session-heading" class="visually-hidden">Server session</h2>

    <div class="session-toolbar">
      <span id="session-target" class="session-target"></span>
      <button type="button" id="btn-disconnect">Disconnect</button>
    </div>

    <nav aria-label="Server management sections" class="tab-list" role="tablist">
      <button role="tab" id="tab-services" aria-controls="panel-services" aria-selected="true" class="tab-btn">Services</button>
      <button role="tab" id="tab-cron" aria-controls="panel-cron" aria-selected="false" class="tab-btn">Crontab</button>
      <button role="tab" id="tab-files" aria-controls="panel-files" aria-selected="false" class="tab-btn">Files</button>
      <button role="tab" id="tab-resources" aria-controls="panel-resources" aria-selected="false" class="tab-btn">Resources</button>
      <button role="tab" id="tab-apt" aria-controls="panel-apt" aria-selected="false" class="tab-btn">Updates</button>
      <button role="tab" id="tab-terminal" aria-controls="panel-terminal" aria-selected="false" class="tab-btn">Terminal</button>
    </nav>

    <!-- ---- Services panel ---- -->
    <div role="tabpanel" id="panel-services" aria-labelledby="tab-services" class="tab-panel">
      <h3 class="visually-hidden">Systemd services</h3>

      <div class="toolbar">
        <button type="button" id="svc-refresh">Refresh list</button>
        <button type="button" id="svc-start">Start</button>
        <button type="button" id="svc-stop">Stop</button>
        <button type="button" id="svc-restart">Restart</button>
        <button type="button" id="svc-enable">Enable</button>
        <button type="button" id="svc-disable">Disable</button>
        <button type="button" id="svc-logs">View logs</button>
        <button type="button" id="svc-status">View status</button>
        <label class="checkbox-label"><input type="checkbox" id="svc-sudo" checked> Use sudo</label>
      </div>

      <label for="svc-filter">Filter services by name</label>
      <input type="text" id="svc-filter" placeholder="Type to filter…">

      <div class="table-wrap">
        <table id="svc-table" aria-label="Systemd services">
          <caption class="visually-hidden">List of systemd services with their active state</caption>
          <thead>
            <tr>
              <th scope="col"><button type="button" class="sort-btn" data-col="0">Service</button></th>
              <th scope="col"><button type="button" class="sort-btn" data-col="1">Active</button></th>
              <th scope="col"><button type="button" class="sort-btn" data-col="2">State</button></th>
              <th scope="col">Description</th>
            </tr>
          </thead>
          <tbody id="svc-tbody">
          </tbody>
        </table>
      </div>

      <h4 id="svc-output-heading">Output / logs</h4>
      <textarea id="svc-output" aria-labelledby="svc-output-heading" readonly rows="14"></textarea>
    </div>

    <!-- ---- Crontab panel ---- -->
    <div role="tabpanel" id="panel-cron" aria-labelledby="tab-cron" class="tab-panel" hidden>
      <h3 class="visually-hidden">Scheduled tasks (crontab)</h3>

      <div class="toolbar">
        <label for="cron-user">User (blank = current user)</label>
        <input type="text" id="cron-user" placeholder="current user">
        <button type="button" id="cron-load">Load schedule</button>
        <label class="checkbox-label"><input type="checkbox" id="cron-sudo"> Use sudo</label>
      </div>

      <p id="cron-status" role="status" aria-live="polite"></p>

      <!-- ---- Structured list of entries (default view) ---- -->
      <div id="cron-entries-view">
        <div class="toolbar">
          <button type="button" id="cron-add-entry" class="primary">Add a new scheduled task</button>
          <button type="button" id="cron-save-entries" class="primary">Save all changes</button>
          <button type="button" id="cron-switch-raw">Switch to raw text editor</button>
        </div>

        <ul id="cron-entry-list" class="cron-entry-list" aria-label="Scheduled tasks"></ul>
        <p id="cron-empty-msg" hidden>No scheduled tasks yet. Use "Add a new scheduled task" to create one.</p>
      </div>

      <!-- ---- Raw text fallback (advanced mode) ---- -->
      <div id="cron-raw-view" hidden>
        <p id="cron-raw-help">
          Each line is one scheduled task: five time fields (minute, hour, day of month,
          month, day of week) followed by the command. A star means "any value". Lines
          starting with # are comments and are kept as-is.
        </p>
        <label for="cron-editor" class="visually-hidden">Crontab contents, one entry per line</label>
        <textarea id="cron-editor" aria-describedby="cron-raw-help" rows="20" spellcheck="false"></textarea>
        <div class="toolbar">
          <button type="button" id="cron-save-raw" class="primary">Save raw crontab</button>
          <button type="button" id="cron-switch-entries">Switch to structured editor</button>
        </div>
      </div>
    </div>

    <!-- ---- Cron entry edit dialog ---- -->
    <dialog id="dlg-cron-entry" aria-labelledby="dlg-cron-entry-title" class="dlg-large">
      <h2 id="dlg-cron-entry-title">Scheduled task</h2>
      <form id="cron-entry-form">

        <fieldset>
          <legend>When should this run?</legend>
          <div class="field-row">
            <label for="cron-preset">Quick choice</label>
            <select id="cron-preset">
              <option value="custom">Custom (set fields below)</option>
              <option value="reboot">When the server starts (@reboot)</option>
              <option value="0 * * * *">Every hour, on the hour</option>
              <option value="*/15 * * * *">Every 15 minutes</option>
              <option value="*/30 * * * *">Every 30 minutes</option>
              <option value="0 0 * * *">Every day at midnight</option>
              <option value="0 H * * *">Every day at a time I choose</option>
              <option value="0 H * * 1">Every Monday at a time I choose</option>
              <option value="0 H 1 * *">First day of every month at a time I choose</option>
            </select>
          </div>
        </fieldset>

        <fieldset id="cron-time-fields">
          <legend>Time of day</legend>
          <div class="field-row inline-row">
            <div>
              <label for="cron-hour">Hour (0–23, or * for every hour)</label>
              <input type="text" id="cron-hour" value="0" inputmode="numeric">
            </div>
            <div>
              <label for="cron-minute">Minute (0–59, or * for every minute)</label>
              <input type="text" id="cron-minute" value="0" inputmode="numeric">
            </div>
          </div>
        </fieldset>

        <fieldset id="cron-date-fields">
          <legend>Which days?</legend>
          <div class="field-row">
            <label for="cron-dom">Day of month (1–31, or * for every day)</label>
            <input type="text" id="cron-dom" value="*">
          </div>
          <div class="field-row">
            <label for="cron-month">Month (1–12, or * for every month)</label>
            <input type="text" id="cron-month" value="*">
          </div>
          <div class="field-row">
            <legend class="sub-legend">Day of week</legend>
            <div class="weekday-checks" role="group" aria-label="Day of week">
              <label class="checkbox-label"><input type="checkbox" class="cron-dow" value="1"> Mon</label>
              <label class="checkbox-label"><input type="checkbox" class="cron-dow" value="2"> Tue</label>
              <label class="checkbox-label"><input type="checkbox" class="cron-dow" value="3"> Wed</label>
              <label class="checkbox-label"><input type="checkbox" class="cron-dow" value="4"> Thu</label>
              <label class="checkbox-label"><input type="checkbox" class="cron-dow" value="5"> Fri</label>
              <label class="checkbox-label"><input type="checkbox" class="cron-dow" value="6"> Sat</label>
              <label class="checkbox-label"><input type="checkbox" class="cron-dow" value="0"> Sun</label>
            </div>
            <p class="hint">Leave all days unchecked to mean "every day".</p>
          </div>
        </fieldset>

        <fieldset>
          <legend>What should run?</legend>
          <div class="field-row">
            <label for="cron-command">Command</label>
            <input type="text" id="cron-command" placeholder="/usr/bin/backup.sh" required>
          </div>
          <div class="field-row">
            <label for="cron-comment">Description (optional, saved as a comment above the task)</label>
            <input type="text" id="cron-comment" placeholder="e.g. Nightly database backup">
          </div>
        </fieldset>

        <fieldset>
          <legend>Preview</legend>
          <p id="cron-preview" class="cron-preview" role="status" aria-live="polite"></p>
        </fieldset>

        <div class="button-row">
          <button type="button" data-close-dialog="dlg-cron-entry">Cancel</button>
          <button type="button" id="cron-entry-delete" class="danger" hidden>Delete this task</button>
          <button type="submit" class="primary">Save task</button>
        </div>
      </form>
    </dialog>

    <!-- ---- Files panel ---- -->
    <div role="tabpanel" id="panel-files" aria-labelledby="tab-files" class="tab-panel" hidden>
      <h3 class="visually-hidden">File manager</h3>

      <div class="toolbar">
        <button type="button" id="files-up">Up</button>
        <button type="button" id="files-home">Home</button>
        <button type="button" id="files-root">Root /</button>
        <label for="files-path">Current path</label>
        <input type="text" id="files-path" value="/">
        <button type="button" id="files-go">Go</button>
        <label class="checkbox-label"><input type="checkbox" id="files-sudo"> Use sudo</label>
      </div>

      <div class="toolbar">
        <button type="button" id="files-edit">Edit file</button>
        <button type="button" id="files-chmod">chmod…</button>
        <button type="button" id="files-chown">chown…</button>
        <button type="button" id="files-disk">Disk usage</button>
        <button type="button" id="files-refresh">Refresh</button>
      </div>

      <div class="table-wrap">
        <table id="files-table" aria-label="Files and directories in current path">
          <caption class="visually-hidden">Directory listing</caption>
          <thead>
            <tr>
              <th scope="col">Name</th>
              <th scope="col">Permissions</th>
              <th scope="col">Owner</th>
              <th scope="col">Group</th>
              <th scope="col">Size</th>
              <th scope="col">Modified</th>
            </tr>
          </thead>
          <tbody id="files-tbody">
          </tbody>
        </table>
      </div>

      <p id="files-status" role="status" aria-live="polite"></p>
    </div>

    <!-- ---- Resources panel ---- -->
    <div role="tabpanel" id="panel-resources" aria-labelledby="tab-resources" class="tab-panel" hidden>
      <h3 class="visually-hidden">Server resources</h3>

      <div class="toolbar">
        <button type="button" id="res-refresh">Refresh</button>
        <label class="checkbox-label"><input type="checkbox" id="res-auto"> Auto-refresh every 5 seconds</label>
      </div>

      <h4>Summary</h4>
      <dl id="res-summary" class="res-summary"></dl>

      <h4 id="res-proc-heading">Top processes by CPU</h4>
      <textarea id="res-processes" aria-labelledby="res-proc-heading" readonly rows="16"></textarea>

      <h4 id="res-disk-heading">Disk usage</h4>
      <textarea id="res-disk" aria-labelledby="res-disk-heading" readonly rows="8"></textarea>
    </div>

    <!-- ---- Apt / updates panel ---- -->
    <div role="tabpanel" id="panel-apt" aria-labelledby="tab-apt" class="tab-panel" hidden>
      <h3 class="visually-hidden">System updates</h3>

      <fieldset>
        <legend>Options</legend>
        <div class="field-row">
          <label for="apt-config-action">When config files conflict</label>
          <select id="apt-config-action">
            <option value="keep" selected>Keep old config (safe default)</option>
            <option value="new">Use new package config</option>
            <option value="default">Use package default</option>
          </select>
        </div>
        <div class="field-row">
          <label for="apt-upgrade-type">Upgrade type</label>
          <select id="apt-upgrade-type">
            <option value="upgrade" selected>upgrade — never removes packages</option>
            <option value="dist-upgrade">dist-upgrade — may add or remove packages</option>
          </select>
        </div>
      </fieldset>

      <div class="toolbar">
        <button type="button" id="apt-update">1. Update package list</button>
        <button type="button" id="apt-upgrade">2. Upgrade packages</button>
        <button type="button" id="apt-both">Update + Upgrade</button>
        <button type="button" id="apt-cancel" disabled>Cancel / Kill</button>
        <button type="button" id="apt-clear">Clear output</button>
      </div>

      <p id="apt-status" role="status" aria-live="polite">Ready.</p>

      <label for="apt-progress" id="apt-progress-label">Progress</label>
      <progress id="apt-progress" max="100" value="0"></progress>
      <span id="apt-progress-text">Idle</span>

      <h4 id="apt-output-heading">Live output</h4>
      <textarea id="apt-output" aria-labelledby="apt-output-heading" readonly rows="18"></textarea>
    </div>

    <!-- ---- Terminal panel ---- -->
    <div role="tabpanel" id="panel-terminal" aria-labelledby="tab-terminal" class="tab-panel" hidden>
      <h3 class="visually-hidden">Terminal</h3>
      <p>Run arbitrary commands on the server. Use the sudo checkbox for privileged commands. This connects via SSH, no extra software needed on the server.</p>

      <h4 id="term-output-heading">Output</h4>
      <textarea id="term-output" aria-labelledby="term-output-heading" readonly rows="18"></textarea>

      <form id="term-form" class="toolbar">
        <label for="term-cmd">Command</label>
        <input type="text" id="term-cmd" autocomplete="off">
        <label class="checkbox-label"><input type="checkbox" id="term-sudo"> sudo</label>
        <button type="submit">Run</button>
        <button type="button" id="term-clear">Clear</button>
      </form>
    </div>

  </section>

</main>

<!-- ===================== DIALOGS ===================== -->

<dialog id="dlg-settings" aria-labelledby="dlg-settings-title">
  <h2 id="dlg-settings-title">Settings</h2>
  <form id="settings-form">
    <div class="field-row">
      <label for="settings-sudo">Default sudo password (used for new connections)</label>
      <input type="password" id="settings-sudo" autocomplete="off">
    </div>
    <div class="field-row">
      <label for="settings-lines">Default log lines to fetch</label>
      <input type="text" id="settings-lines" inputmode="numeric">
    </div>
    <div class="button-row">
      <button type="button" data-close-dialog="dlg-settings">Cancel</button>
      <button type="submit" class="primary">Save</button>
    </div>
  </form>
</dialog>

<dialog id="dlg-save-host" aria-labelledby="dlg-save-host-title">
  <h2 id="dlg-save-host-title">Save host</h2>
  <form id="save-host-form">
    <div class="field-row">
      <label for="save-host-name">Name for this saved host</label>
      <input type="text" id="save-host-name" required>
    </div>
    <div class="button-row">
      <button type="button" data-close-dialog="dlg-save-host">Cancel</button>
      <button type="submit" class="primary">Save</button>
    </div>
  </form>
</dialog>

<dialog id="dlg-manage-hosts" aria-labelledby="dlg-manage-hosts-title">
  <h2 id="dlg-manage-hosts-title">Saved hosts</h2>
  <ul id="saved-hosts-list" class="saved-hosts-list"></ul>
  <div class="button-row">
    <button type="button" data-close-dialog="dlg-manage-hosts">Close</button>
  </div>
</dialog>

<dialog id="dlg-file-editor" aria-labelledby="dlg-file-editor-title" class="dlg-large">
  <h2 id="dlg-file-editor-title">Edit file</h2>
  <p id="file-editor-path"></p>
  <p id="file-editor-status" role="status" aria-live="polite"></p>
  <label for="file-editor-sudo" class="checkbox-label"><input type="checkbox" id="file-editor-sudo"> Use sudo for this file</label>
  <label for="file-editor-textarea" class="visually-hidden">File contents</label>
  <textarea id="file-editor-textarea" rows="24" spellcheck="false"></textarea>
  <div class="button-row">
    <button type="button" data-close-dialog="dlg-file-editor">Close</button>
    <button type="button" id="file-editor-save" class="primary">Save</button>
  </div>
</dialog>

<dialog id="dlg-chmod" aria-labelledby="dlg-chmod-title">
  <h2 id="dlg-chmod-title">Change permissions</h2>
  <p id="chmod-path"></p>
  <form id="chmod-form">
    <div class="field-row">
      <label for="chmod-mode">Mode (e.g. 755 or u+x)</label>
      <input type="text" id="chmod-mode" value="644" required>
    </div>
    <label class="checkbox-label"><input type="checkbox" id="chmod-recursive"> Recursive</label>
    <label class="checkbox-label"><input type="checkbox" id="chmod-sudo"> Use sudo</label>
    <div class="button-row">
      <button type="button" data-close-dialog="dlg-chmod">Cancel</button>
      <button type="submit" class="primary">Apply</button>
    </div>
  </form>
</dialog>

<dialog id="dlg-chown" aria-labelledby="dlg-chown-title">
  <h2 id="dlg-chown-title">Change owner</h2>
  <p id="chown-path"></p>
  <form id="chown-form">
    <div class="field-row">
      <label for="chown-owner">Owner</label>
      <input type="text" id="chown-owner" required>
    </div>
    <div class="field-row">
      <label for="chown-group">Group (optional)</label>
      <input type="text" id="chown-group">
    </div>
    <label class="checkbox-label"><input type="checkbox" id="chown-recursive"> Recursive</label>
    <label class="checkbox-label"><input type="checkbox" id="chown-sudo"> Use sudo</label>
    <div class="button-row">
      <button type="button" data-close-dialog="dlg-chown">Cancel</button>
      <button type="submit" class="primary">Apply</button>
    </div>
  </form>
</dialog>

<dialog id="dlg-interactive" aria-labelledby="dlg-interactive-title" class="dlg-large">
  <h2 id="dlg-interactive-title">Interactive terminal</h2>
  <p>This command needs interactive answers. Type a response below and press Enter to send it.</p>
  <label for="interactive-output" class="visually-hidden">Command output</label>
  <textarea id="interactive-output" readonly rows="18"></textarea>
  <form id="interactive-form" class="toolbar">
    <label for="interactive-input">Your response</label>
    <input type="text" id="interactive-input" autocomplete="off">
    <button type="submit">Send</button>
  </form>
  <div class="button-row">
    <button type="button" data-close-dialog="dlg-interactive">Close</button>
  </div>
</dialog>

<script src="/static/app.js"></script>
</body>
</html>
`
