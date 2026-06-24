package ui

const appJS = `
'use strict';

// All request/response calls go straight through glaze's Bind bridge:
// window.goshell_<method>(args...) returns a Promise, backed directly by a
// Go method on Service (see internal/ui/service.go). There is no fetch/JSON
// route layer to maintain for these.
//
// Only two things still go over a real WebSocket, because Bind has no
// server-push primitive: live apt output/progress, and the interactive
// terminal (stdin keeps flowing while output keeps streaming).

// ============== Utilities ==============

function $(id) { return document.getElementById(id); }

function announce(msg, urgent) {
  var el = urgent ? $('alert-live') : $('status-live');
  el.textContent = '';
  window.setTimeout(function () { el.textContent = msg; }, 30);
}

function setStatus(msg) {
  $('connection-status').textContent = msg;
}

function wsURL(path) {
  return 'ws://' + window.location.host + path;
}

function escapeHTML(s) {
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

async function call(fnName) {
  const fn = window['goshell_' + fnName];
  if (!fn) throw new Error('not available: ' + fnName);
  const args = Array.prototype.slice.call(arguments, 1);
  return await fn.apply(null, args);
}

// ============== Dialog handling ==============

document.addEventListener('click', function (e) {
  const closeId = e.target.getAttribute && e.target.getAttribute('data-close-dialog');
  if (closeId) $(closeId).close();
});

function openDialog(id) {
  const dlg = $(id);
  dlg.showModal();
  const firstInput = dlg.querySelector('input, select, textarea, button');
  if (firstInput) firstInput.focus();
}

// ============== Connect screen ==============

let sshConfigHosts = [];
let savedHosts = [];

async function loadConnectScreenData() {
  try {
    sshConfigHosts = await call('ssh_hosts') || [];
    const keys = await call('ssh_keys') || [];
    savedHosts = await call('config_hosts') || [];

    const sel = $('known-hosts-select');
    sel.innerHTML = '<option value="">— manual entry —</option>';
    savedHosts.forEach(function (h) {
      const opt = document.createElement('option');
      opt.value = 'saved:' + h.name;
      opt.textContent = 'Saved: ' + h.name;
      sel.appendChild(opt);
    });
    sshConfigHosts.forEach(function (h) {
      const opt = document.createElement('option');
      opt.value = 'config:' + h.Name;
      opt.textContent = h.Name;
      sel.appendChild(opt);
    });

    const keySel = $('field-key');
    keySel.innerHTML = '<option value="">— none / try all keys —</option>';
    keys.forEach(function (k) {
      const opt = document.createElement('option');
      opt.value = k;
      opt.textContent = k.split('/').pop() + '  (' + k + ')';
      keySel.appendChild(opt);
    });
  } catch (err) {
    announce('Error loading connection data: ' + err.message, true);
  }
}

$('known-hosts-select').addEventListener('change', function () {
  const val = this.value;
  if (!val) return;
  if (val.indexOf('saved:') === 0) {
    const h = savedHosts.find(function (x) { return x.name === val.slice(6); });
    if (h) {
      $('field-hostname').value = h.hostname || '';
      $('field-port').value = h.port || '22';
      $('field-user').value = h.user || '';
      if (h.sudo_pass) $('field-sudo').value = h.sudo_pass;
      if (h.key_file) selectKeyByPath(h.key_file);
    }
  } else if (val.indexOf('config:') === 0) {
    const h = sshConfigHosts.find(function (x) { return x.Name === val.slice(7); });
    if (h) {
      if (h.Hostname) $('field-hostname').value = h.Hostname;
      if (h.Port) $('field-port').value = h.Port;
      if (h.User) $('field-user').value = h.User;
      if (h.IdentityFile) selectKeyByPath(h.IdentityFile);
    }
  }
  announce('Filled in connection details for ' + this.options[this.selectedIndex].textContent);
});

function selectKeyByPath(path) {
  const sel = $('field-key');
  const base = path.split('/').pop();
  for (let i = 0; i < sel.options.length; i++) {
    if (sel.options[i].value === path || sel.options[i].textContent.indexOf(base) >= 0) {
      sel.selectedIndex = i;
      return;
    }
  }
}

$('connect-form').addEventListener('submit', async function (e) {
  e.preventDefault();
  const hostname = $('field-hostname').value.trim();
  if (!hostname) {
    announce('Hostname is required', true);
    $('field-hostname').focus();
    return;
  }
  const params = {
    Host: hostname,
    Port: $('field-port').value.trim() || '22',
    User: $('field-user').value.trim() || 'root',
    KeyFile: $('field-key').value,
    Password: $('field-password').value,
    SudoPass: $('field-sudo').value
  };
  const btn = $('btn-connect');
  btn.disabled = true;
  setStatus('Connecting to ' + params.User + '@' + params.Host + ':' + params.Port + ' …');
  announce('Connecting…');
  try {
    const data = await call('connect', params);
    setStatus('Connected to ' + data.user + '@' + data.host);
    announce('Connected to ' + data.host);
    $('session-target').textContent = data.user + '@' + data.host;
    $('screen-connect').hidden = true;
    $('screen-session').hidden = false;
    initSessionScreen();
  } catch (err) {
    setStatus('Connection failed');
    announce('Connection failed: ' + err.message, true);
  } finally {
    btn.disabled = false;
  }
});

$('btn-disconnect').addEventListener('click', async function () {
  try { await call('disconnect'); } catch (e) {}
  setStatus('Not connected');
  announce('Disconnected');
  $('screen-session').hidden = true;
  $('screen-connect').hidden = false;
  $('field-hostname').focus();
});

$('btn-open-settings').addEventListener('click', async function () {
  try {
    const cfg = await call('get_settings');
    $('settings-sudo').value = cfg.GlobalSudo || '';
    $('settings-lines').value = cfg.DefaultLines || 200;
  } catch (e) {}
  openDialog('dlg-settings');
});

$('settings-form').addEventListener('submit', async function (e) {
  e.preventDefault();
  try {
    await call('save_settings', {
      GlobalSudo: $('settings-sudo').value,
      DefaultLines: parseInt($('settings-lines').value, 10) || 200
    });
    announce('Settings saved');
    $('dlg-settings').close();
  } catch (err) {
    announce('Error saving settings: ' + err.message, true);
  }
});

$('btn-save-host').addEventListener('click', function () {
  const hostname = $('field-hostname').value.trim();
  if (!hostname) {
    announce('Enter a hostname first', true);
    $('field-hostname').focus();
    return;
  }
  $('save-host-name').value = hostname;
  openDialog('dlg-save-host');
});

$('save-host-form').addEventListener('submit', async function (e) {
  e.preventDefault();
  const name = $('save-host-name').value.trim();
  if (!name) return;
  try {
    await call('save_host', {
      name: name,
      hostname: $('field-hostname').value.trim(),
      port: $('field-port').value.trim() || '22',
      user: $('field-user').value.trim(),
      key_file: $('field-key').value,
      sudo_pass: $('field-sudo').value
    });
    announce('Host "' + name + '" saved');
    $('dlg-save-host').close();
    loadConnectScreenData();
  } catch (err) {
    announce('Error saving host: ' + err.message, true);
  }
});

$('btn-manage-hosts').addEventListener('click', function () {
  renderSavedHostsList();
  openDialog('dlg-manage-hosts');
});

function renderSavedHostsList() {
  const ul = $('saved-hosts-list');
  ul.innerHTML = '';
  if (savedHosts.length === 0) {
    const li = document.createElement('li');
    li.textContent = 'No saved hosts yet.';
    ul.appendChild(li);
    return;
  }
  savedHosts.forEach(function (h) {
    const li = document.createElement('li');
    const span = document.createElement('span');
    span.textContent = h.name + ' (' + h.user + '@' + h.hostname + ':' + h.port + ')';
    li.appendChild(span);
    const delBtn = document.createElement('button');
    delBtn.textContent = 'Delete';
    delBtn.className = 'danger';
    delBtn.setAttribute('aria-label', 'Delete saved host ' + h.name);
    delBtn.addEventListener('click', async function () {
      try {
        await call('delete_host', h.name);
        announce('Deleted host ' + h.name);
        await loadConnectScreenData();
        renderSavedHostsList();
      } catch (err) {
        announce('Error deleting host: ' + err.message, true);
      }
    });
    li.appendChild(delBtn);
    ul.appendChild(li);
  });
}

// ============== Tabs ==============

const tabIds = ['services', 'cron', 'files', 'resources', 'apt', 'terminal'];
let sessionInitialized = false;
let currentTab = 'services';

function initSessionScreen() {
  tabIds.forEach(function (id) {
    $('tab-' + id).addEventListener('click', function () { selectTab(id); });
  });
  document.querySelector('.tab-list').addEventListener('keydown', function (e) {
    const idx = tabIds.indexOf(currentTab);
    if (e.key === 'ArrowRight') {
      const next = tabIds[(idx + 1) % tabIds.length];
      selectTab(next); $('tab-' + next).focus();
    } else if (e.key === 'ArrowLeft') {
      const prev = tabIds[(idx - 1 + tabIds.length) % tabIds.length];
      selectTab(prev); $('tab-' + prev).focus();
    }
  });

  if (!sessionInitialized) {
    sessionInitialized = true;
    initServicesTab();
    initCronTab();
    initFilesTab();
    initResourcesTab();
    initAptTab();
    initTerminalTab();
  }
  selectTab('services');
  refreshServices();
}

function selectTab(id) {
  currentTab = id;
  tabIds.forEach(function (t) {
    const selected = t === id;
    $('tab-' + t).setAttribute('aria-selected', selected ? 'true' : 'false');
    $('panel-' + t).hidden = !selected;
  });
  // No announce() here: role="tab" + aria-selected is already a native
  // state NVDA speaks on its own when a tab is activated. A live-region
  // announcement on top of that is a second, redundant voice for the
  // same event, not new information.
}

// ============== Services tab ==============

let servicesData = [];
let selectedService = null;

function initServicesTab() {
  $('svc-refresh').addEventListener('click', refreshServices);
  $('svc-filter').addEventListener('input', renderServicesTable);
  $('svc-start').addEventListener('click', function () { serviceAction('start'); });
  $('svc-stop').addEventListener('click', function () { serviceAction('stop'); });
  $('svc-restart').addEventListener('click', function () { serviceAction('restart'); });
  $('svc-enable').addEventListener('click', function () { serviceAction('enable'); });
  $('svc-disable').addEventListener('click', function () { serviceAction('disable'); });
  $('svc-logs').addEventListener('click', viewServiceLogs);
  $('svc-status').addEventListener('click', viewServiceStatus);
  $('svc-tbody').addEventListener('click', function (e) {
    const tr = e.target.closest('tr');
    if (tr) selectServiceRow(tr);
  });
}

async function refreshServices() {
  setStatus('Loading services…');
  try {
    servicesData = await call('list_services') || [];
    renderServicesTable();
    setStatus('Connected — ' + servicesData.length + ' services loaded');
    announce(servicesData.length + ' services loaded');
  } catch (err) {
    announce('Error loading services: ' + err.message, true);
  }
}

function renderServicesTable() {
  const filter = $('svc-filter').value.toLowerCase();
  const tbody = $('svc-tbody');
  tbody.innerHTML = '';
  servicesData
    .filter(function (s) { return !filter || s.Name.toLowerCase().indexOf(filter) >= 0; })
    .forEach(function (s) {
      const tr = document.createElement('tr');
      tr.tabIndex = 0;
      tr.dataset.name = s.Name;
      if (s.Active === 'active') tr.className = 'row-active';
      else if (s.Active === 'failed') tr.className = 'row-failed';
      tr.innerHTML =
        '<td>' + escapeHTML(s.Name) + '</td>' +
        '<td>' + escapeHTML(s.Active) + '</td>' +
        '<td>' + escapeHTML(s.Sub) + '</td>' +
        '<td>' + escapeHTML(s.Description || '') + '</td>';
      tbody.appendChild(tr);
    });
}

function selectServiceRow(tr) {
  document.querySelectorAll('#svc-tbody tr').forEach(function (r) { r.removeAttribute('aria-selected'); });
  tr.setAttribute('aria-selected', 'true');
  selectedService = tr.dataset.name;
}

async function serviceAction(action) {
  if (!selectedService) { announce('No service selected', true); return; }
  const useSudo = $('svc-sudo').checked;
  setStatus('Running systemctl ' + action + ' ' + selectedService + ' …');
  try {
    await call('service_action', { name: selectedService, action: action, use_sudo: useSudo });
    announce('systemctl ' + action + ' ' + selectedService + ' succeeded');
    setStatus('OK: systemctl ' + action + ' ' + selectedService);
    refreshServices();
  } catch (err) {
    announce('Error: ' + err.message, true);
    setStatus('Failed: systemctl ' + action + ' ' + selectedService);
  }
}

async function viewServiceLogs() {
  if (!selectedService) { announce('No service selected', true); return; }
  setStatus('Loading logs for ' + selectedService + ' …');
  try {
    const logs = await call('service_logs', selectedService);
    $('svc-output').value = logs;
    announce('Logs loaded for ' + selectedService);
    $('svc-output').focus();
  } catch (err) {
    announce('Error loading logs: ' + err.message, true);
  }
}

async function viewServiceStatus() {
  if (!selectedService) { announce('No service selected', true); return; }
  try {
    $('svc-output').value = await call('service_status_detail', selectedService);
  } catch (err) {
    announce('Error: ' + err.message, true);
  }
}

// ============== Crontab tab ==============
//
// Default view is a structured list of scheduled tasks built from
// CronEntry objects the Go side parsed out of the raw crontab (see
// internal/ui/cron.go) -- no cron syntax is typed by the user. A raw
// text mode stays available for entries the parser can't represent and
// for people who already know the syntax and want it directly.

let cronEntries = [];
let cronEditingId = null; // null = adding a new entry

function initCronTab() {
  $('cron-load').addEventListener('click', loadCronEntries);
  $('cron-add-entry').addEventListener('click', openNewCronEntry);
  $('cron-save-entries').addEventListener('click', saveCronEntries);
  $('cron-switch-raw').addEventListener('click', switchToRawMode);
  $('cron-switch-entries').addEventListener('click', switchToEntriesMode);
  $('cron-save-raw').addEventListener('click', saveCrontabRaw);

  $('cron-entry-list').addEventListener('click', function (e) {
    const btn = e.target.closest('button[data-edit-id]');
    if (btn) openExistingCronEntry(btn.dataset.editId);
  });

  $('cron-preset').addEventListener('change', applyCronPreset);
  ['cron-hour', 'cron-minute', 'cron-dom', 'cron-month'].forEach(function (id) {
    $(id).addEventListener('input', updateCronPreview);
  });
  document.querySelectorAll('.cron-dow').forEach(function (cb) {
    cb.addEventListener('change', updateCronPreview);
  });

  $('cron-entry-form').addEventListener('submit', saveCronEntryFromDialog);
  $('cron-entry-delete').addEventListener('click', deleteCronEntryFromDialog);
}

async function loadCronEntries() {
  const user = $('cron-user').value.trim();
  $('cron-status').textContent = 'Loading…';
  try {
    cronEntries = await call('get_cron_entries', user) || [];
    renderCronEntries();
    $('cron-status').textContent = cronEntries.length + ' scheduled task(s) loaded';
    announce(cronEntries.length + ' scheduled tasks loaded');
    // Keep the raw view in sync too, in case the user switches over.
    const raw = await call('get_crontab', user);
    $('cron-editor').value = raw;
  } catch (err) {
    $('cron-status').textContent = 'Error: ' + err.message;
    announce('Error loading scheduled tasks: ' + err.message, true);
  }
}

function renderCronEntries() {
  const list = $('cron-entry-list');
  list.innerHTML = '';
  const real = cronEntries.filter(function (e) { return !e.raw; });
  $('cron-empty-msg').hidden = real.length > 0;

  cronEntries.forEach(function (e) {
    if (e.raw) return; // raw/unparsed lines aren't shown as editable tasks
    const li = document.createElement('li');
    li.className = 'cron-entry-item';

    const desc = describeScheduleLocally(e);
    const label = (e.comment ? e.comment + ' — ' : '') + e.command;

    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'cron-entry-btn';
    btn.dataset.editId = e.id;
    btn.innerHTML =
      '<span class="cron-entry-title">' + escapeHTML(label) + '</span>' +
      '<span class="cron-entry-schedule">' + escapeHTML(desc) + '</span>';
    li.appendChild(btn);
    list.appendChild(li);
  });
}

// Quick local description so the list renders instantly without a round
// trip per row; the dialog's live preview uses the authoritative Go-side
// describe_cron_schedule call instead.
function describeScheduleLocally(e) {
  if (e.is_reboot) return 'When the server starts';
  if (e.minute === '*' && e.hour === '*') return 'Every minute';
  if (e.dom === '*' && e.month === '*' && e.dow === '*') {
    return 'Every day at ' + pad2(e.hour) + ':' + pad2(e.minute);
  }
  return e.minute + ' ' + e.hour + ' ' + e.dom + ' ' + e.month + ' ' + e.dow;
}

function pad2(s) {
  if (!/^\d+$/.test(s)) return s;
  return s.length < 2 ? '0' + s : s;
}

function openNewCronEntry() {
  cronEditingId = null;
  $('dlg-cron-entry-title').textContent = 'New scheduled task';
  $('cron-entry-delete').hidden = true;
  $('cron-preset').value = 'custom';
  $('cron-hour').value = '0';
  $('cron-minute').value = '0';
  $('cron-dom').value = '*';
  $('cron-month').value = '*';
  document.querySelectorAll('.cron-dow').forEach(function (cb) { cb.checked = false; });
  $('cron-command').value = '';
  $('cron-comment').value = '';
  setCronTimeFieldsVisible(true);
  updateCronPreview();
  openDialog('dlg-cron-entry');
}

function openExistingCronEntry(id) {
  const e = cronEntries.find(function (x) { return x.id === id; });
  if (!e) return;
  cronEditingId = id;
  $('dlg-cron-entry-title').textContent = 'Edit scheduled task';
  $('cron-entry-delete').hidden = false;
  $('cron-preset').value = 'custom';
  $('cron-command').value = e.command || '';
  $('cron-comment').value = e.comment || '';

  if (e.is_reboot) {
    $('cron-preset').value = 'reboot';
    setCronTimeFieldsVisible(false);
  } else {
    $('cron-hour').value = e.hour || '0';
    $('cron-minute').value = e.minute || '0';
    $('cron-dom').value = e.dom || '*';
    $('cron-month').value = e.month || '*';
    document.querySelectorAll('.cron-dow').forEach(function (cb) {
      cb.checked = (e.dow || '').split(',').indexOf(cb.value) >= 0;
    });
    setCronTimeFieldsVisible(true);
  }
  updateCronPreview();
  openDialog('dlg-cron-entry');
}

function applyCronPreset() {
  const val = $('cron-preset').value;
  if (val === 'custom') { setCronTimeFieldsVisible(true); updateCronPreview(); return; }
  if (val === 'reboot') { setCronTimeFieldsVisible(false); updateCronPreview(); return; }

  setCronTimeFieldsVisible(true);
  // Presets use "H" as a placeholder meaning "let the user's current hour field stand".
  const parts = val.split(' ');
  if (parts.length === 5) {
    if (parts[0] !== 'H') $('cron-minute').value = parts[0];
    if (parts[1] !== 'H') $('cron-hour').value = parts[1];
    $('cron-dom').value = parts[2];
    $('cron-month').value = parts[3];
    document.querySelectorAll('.cron-dow').forEach(function (cb) {
      cb.checked = parts[4] !== '*' && parts[4].split(',').indexOf(cb.value) >= 0;
    });
  }
  updateCronPreview();
}

function setCronTimeFieldsVisible(visible) {
  $('cron-time-fields').hidden = !visible;
  $('cron-date-fields').hidden = !visible;
}

function collectCronEntryFromDialog() {
  const isReboot = $('cron-preset').value === 'reboot';
  const checkedDows = Array.prototype.slice.call(document.querySelectorAll('.cron-dow:checked'))
    .map(function (cb) { return cb.value; });
  return {
    id: cronEditingId || '',
    comment: $('cron-comment').value.trim(),
    command: $('cron-command').value.trim(),
    is_reboot: isReboot,
    minute: isReboot ? '' : ($('cron-minute').value.trim() || '*'),
    hour: isReboot ? '' : ($('cron-hour').value.trim() || '*'),
    dom: isReboot ? '' : ($('cron-dom').value.trim() || '*'),
    month: isReboot ? '' : ($('cron-month').value.trim() || '*'),
    dow: isReboot ? '' : (checkedDows.length ? checkedDows.join(',') : '*'),
    raw: ''
  };
}

async function updateCronPreview() {
  const entry = collectCronEntryFromDialog();
  try {
    $('cron-preview').textContent = await call('describe_cron_schedule', entry);
  } catch (err) {
    $('cron-preview').textContent = '';
  }
}

function saveCronEntryFromDialog(e) {
  e.preventDefault();
  const entry = collectCronEntryFromDialog();
  if (!entry.command) {
    announce('A command is required', true);
    $('cron-command').focus();
    return;
  }
  if (cronEditingId) {
    const idx = cronEntries.findIndex(function (x) { return x.id === cronEditingId; });
    if (idx >= 0) cronEntries[idx] = Object.assign({}, cronEntries[idx], entry);
  } else {
    entry.id = 'new-' + Date.now();
    cronEntries.push(entry);
  }
  renderCronEntries();
  $('dlg-cron-entry').close();
  announce('Task saved locally. Click "Save all changes" to write it to the server.');
}

function deleteCronEntryFromDialog() {
  if (!cronEditingId) return;
  cronEntries = cronEntries.filter(function (x) { return x.id !== cronEditingId; });
  renderCronEntries();
  $('dlg-cron-entry').close();
  announce('Task removed locally. Click "Save all changes" to write it to the server.');
}

async function saveCronEntries() {
  $('cron-status').textContent = 'Saving…';
  try {
    await call('set_cron_entries', {
      user: $('cron-user').value.trim(),
      entries: cronEntries,
      use_sudo: $('cron-sudo').checked
    });
    $('cron-status').textContent = 'Scheduled tasks saved successfully';
    announce('Scheduled tasks saved');
  } catch (err) {
    $('cron-status').textContent = 'Error: ' + err.message;
    announce('Error saving scheduled tasks: ' + err.message, true);
  }
}

function switchToRawMode() {
  $('cron-entries-view').hidden = true;
  $('cron-raw-view').hidden = false;
  announce('Switched to raw text editor for scheduled tasks');
  $('cron-editor').focus();
}

function switchToEntriesMode() {
  $('cron-raw-view').hidden = true;
  $('cron-entries-view').hidden = false;
  announce('Switched to structured editor for scheduled tasks');
}

async function saveCrontabRaw() {
  $('cron-status').textContent = 'Saving…';
  try {
    await call('set_crontab', {
      user: $('cron-user').value.trim(),
      content: $('cron-editor').value,
      use_sudo: $('cron-sudo').checked
    });
    $('cron-status').textContent = 'Crontab saved successfully';
    announce('Crontab saved');
    // Re-parse so the structured view reflects the raw edit if the user switches back.
    cronEntries = await call('get_cron_entries', $('cron-user').value.trim()) || [];
    renderCronEntries();
  } catch (err) {
    $('cron-status').textContent = 'Error: ' + err.message;
    announce('Error saving crontab: ' + err.message, true);
  }
}

// ============== Files tab ==============
//
// A real ARIA tree (role="tree" / role="treeitem"), not a table that gets
// fully replaced on every navigation. Folders expand in place: opening one
// fetches and inserts its children as a nested group, without throwing
// away the rest of the tree or moving the screen reader's position back
// to the top. Follows the WAI-ARIA tree view keyboard pattern: Up/Down
// move between visible items, Right expands a folder (or moves into it if
// already expanded), Left collapses a folder (or moves to its parent if
// already collapsed), Enter opens a file or toggles a folder, Home/End
// jump to the first/last visible item. The tree container itself is the
// only tab stop; focus moves between items via roving tabindex.

let treeRootPath = '/';
let treeFocusedPath = null; // path of the item that currently has roving focus
let treeNodesByPath = {};   // path -> { li, rowEl, entry, expanded, loaded }

function initFilesTab() {
  $('files-home').addEventListener('click', function () {
    goToPath('/');
    announce('Use the path field to type your home directory, e.g. /home/yourusername');
  });
  $('files-root').addEventListener('click', function () { goToPath('/'); });
  $('files-go').addEventListener('click', function () { goToPath($('files-path').value); });
  $('files-path').addEventListener('keydown', function (e) {
    if (e.key === 'Enter') goToPath(this.value);
  });
  $('files-refresh').addEventListener('click', function () { goToPath(treeRootPath, true); });

  $('files-tree').addEventListener('keydown', onTreeKeyDown);
  $('files-tree').addEventListener('click', function (e) {
    const row = e.target.closest('.tree-node-row');
    if (!row) return;
    const path = row.dataset.path;
    if (e.target.closest('.tree-twisty')) {
      toggleNode(path);
    } else {
      focusNode(path);
      const node = treeNodesByPath[path];
      if (node && node.entry.IsDir) toggleNode(path); else openFileEditorFor(path);
    }
  });

  $('files-edit').addEventListener('click', function () {
    if (!treeFocusedPath) { announce('No item selected', true); return; }
    const node = treeNodesByPath[treeFocusedPath];
    if (!node || node.entry.IsDir) { announce('Select a file, not a folder, to edit', true); return; }
    openFileEditorFor(treeFocusedPath);
  });
  $('files-chmod').addEventListener('click', function () {
    if (!treeFocusedPath) { announce('No item selected', true); return; }
    $('chmod-path').textContent = 'Path: ' + treeFocusedPath;
    openDialog('dlg-chmod');
  });
  $('files-chown').addEventListener('click', function () {
    if (!treeFocusedPath) { announce('No item selected', true); return; }
    $('chown-path').textContent = 'Path: ' + treeFocusedPath;
    openDialog('dlg-chown');
  });
  $('files-disk').addEventListener('click', showDiskUsage);

  $('chmod-form').addEventListener('submit', async function (e) {
    e.preventDefault();
    try {
      await call('chmod', {
        path: treeFocusedPath,
        mode: $('chmod-mode').value,
        recursive: $('chmod-recursive').checked,
        use_sudo: $('chmod-sudo').checked
      });
      announce('Permissions changed');
      $('dlg-chmod').close();
      refreshNodeDetails(treeFocusedPath);
    } catch (err) {
      announce('chmod error: ' + err.message, true);
    }
  });

  $('chown-form').addEventListener('submit', async function (e) {
    e.preventDefault();
    try {
      await call('chown', {
        path: treeFocusedPath,
        owner: $('chown-owner').value,
        group: $('chown-group').value,
        recursive: $('chown-recursive').checked,
        use_sudo: $('chown-sudo').checked
      });
      announce('Owner changed');
      $('dlg-chown').close();
      refreshNodeDetails(treeFocusedPath);
    } catch (err) {
      announce('chown error: ' + err.message, true);
    }
  });

  $('file-editor-save').addEventListener('click', saveFileEditor);

  goToPath('/');
}

// Rebuilds the tree from scratch rooted at the given path. Used for the
// initial load, Root/Home/Go-to-path, and manual refresh -- not for
// expanding a folder you're already looking at, which uses toggleNode()
// instead and never touches anything else on screen.
async function goToPath(path, isRefresh) {
  path = (path || '/').replace(/\/+$/, '') || '/';
  $('files-status').textContent = 'Loading ' + path + ' …';
  try {
    const data = await call('list_dir', path);
    treeRootPath = data.path;
    treeNodesByPath = {};
    $('files-path').value = treeRootPath;

    const tree = $('files-tree');
    tree.innerHTML = '';
    (data.entries || []).forEach(function (entry) {
      const childPath = joinPath(treeRootPath, entry.Name);
      const li = buildTreeNode(entry, childPath, 1);
      tree.appendChild(li);
    });

    resetDetailsPanel();
    const first = tree.querySelector('.tree-node-row');
    if (first) {
      first.tabIndex = 0;
      treeFocusedPath = first.dataset.path;
    }
    $('files-status').textContent = treeRootPath + (isRefresh ? ' refreshed' : '') + ' — ' + (data.entries || []).length + ' entries';
  } catch (err) {
    $('files-status').textContent = 'Error: ' + err.message;
    announce('Error: ' + err.message, true);
  }
}

function joinPath(dir, name) {
  return dir === '/' ? '/' + name : dir + '/' + name;
}

function buildTreeNode(entry, path, level) {
  const li = document.createElement('li');
  li.setAttribute('role', 'treeitem');
  li.setAttribute('aria-level', String(level));
  if (entry.IsDir) li.setAttribute('aria-expanded', 'false');

  const row = document.createElement('div');
  row.className = 'tree-node-row';
  row.dataset.path = path;
  row.tabIndex = -1; // roving tabindex; only the focused item is 0

  const twisty = document.createElement('span');
  twisty.className = 'tree-twisty';
  twisty.setAttribute('aria-hidden', 'true');
  twisty.textContent = entry.IsDir ? '▸' : '';

  const label = document.createElement('span');
  label.textContent = entry.Name + (entry.IsDir ? '/' : '');

  row.appendChild(twisty);
  row.appendChild(label);
  li.appendChild(row);

  treeNodesByPath[path] = { li: li, rowEl: row, entry: entry, expanded: false, loaded: false };
  return li;
}

async function toggleNode(path) {
  const node = treeNodesByPath[path];
  if (!node || !node.entry.IsDir) return;

  if (node.expanded) {
    const group = node.li.querySelector('ul');
    if (group) group.remove();
    node.expanded = false;
    node.li.setAttribute('aria-expanded', 'false');
    node.rowEl.querySelector('.tree-twisty').textContent = '▸';
    focusNode(path);
    return;
  }

  if (!node.loaded) {
    $('files-status').textContent = 'Loading ' + path + ' …';
    try {
      const data = await call('list_dir', path);
      const level = parseInt(node.li.getAttribute('aria-level'), 10) + 1;
      const group = document.createElement('ul');
      group.setAttribute('role', 'group');
      (data.entries || []).forEach(function (entry) {
        const childPath = joinPath(path, entry.Name);
        group.appendChild(buildTreeNode(entry, childPath, level));
      });
      node.li.appendChild(group);
      node.loaded = true;
      $('files-status').textContent = path + ' — ' + (data.entries || []).length + ' entries';
    } catch (err) {
      $('files-status').textContent = 'Error opening ' + path + ': ' + err.message;
      announce('Error opening folder: ' + err.message, true);
      return;
    }
  }

  node.expanded = true;
  node.li.setAttribute('aria-expanded', 'true');
  node.rowEl.querySelector('.tree-twisty').textContent = '▾';
  focusNode(path);
}

function visibleRows() {
  return Array.prototype.slice.call($('files-tree').querySelectorAll('.tree-node-row'));
}

function focusNode(path) {
  const node = treeNodesByPath[path];
  if (!node) return;
  const rows = visibleRows();
  rows.forEach(function (r) { r.tabIndex = -1; });
  node.rowEl.tabIndex = 0;
  node.rowEl.focus();
  treeFocusedPath = path;
  showNodeDetails(node.entry, path);
}

function onTreeKeyDown(e) {
  if (!treeFocusedPath) return;
  const rows = visibleRows();
  const idx = rows.findIndex(function (r) { return r.dataset.path === treeFocusedPath; });
  if (idx < 0) return;
  const node = treeNodesByPath[treeFocusedPath];

  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault();
      if (idx + 1 < rows.length) focusNode(rows[idx + 1].dataset.path);
      break;
    case 'ArrowUp':
      e.preventDefault();
      if (idx - 1 >= 0) focusNode(rows[idx - 1].dataset.path);
      break;
    case 'ArrowRight':
      e.preventDefault();
      if (node.entry.IsDir && !node.expanded) {
        toggleNode(treeFocusedPath);
      } else if (node.entry.IsDir && node.expanded) {
        if (idx + 1 < rows.length) focusNode(rows[idx + 1].dataset.path);
      }
      break;
    case 'ArrowLeft':
      e.preventDefault();
      if (node.entry.IsDir && node.expanded) {
        toggleNode(treeFocusedPath);
      } else {
        const parentLi = node.li.parentElement.closest('li[role="treeitem"]');
        if (parentLi) {
          const parentRow = parentLi.querySelector('.tree-node-row');
          if (parentRow) focusNode(parentRow.dataset.path);
        }
      }
      break;
    case 'Home':
      e.preventDefault();
      if (rows.length) focusNode(rows[0].dataset.path);
      break;
    case 'End':
      e.preventDefault();
      if (rows.length) focusNode(rows[rows.length - 1].dataset.path);
      break;
    case 'Enter':
      e.preventDefault();
      if (node.entry.IsDir) toggleNode(treeFocusedPath);
      else openFileEditorFor(treeFocusedPath);
      break;
  }
}

function showNodeDetails(entry, path) {
  $('fd-name').textContent = entry.Name;
  $('fd-type').textContent = entry.IsDir ? 'Folder' : 'File';
  $('fd-perms').textContent = entry.Permissions || '—';
  $('fd-owner').textContent = entry.Owner || '—';
  $('fd-group').textContent = entry.Group || '—';
  $('fd-size').textContent = entry.IsDir ? '—' : String(entry.Size);
  $('fd-modified').textContent = entry.Modified || '—';
  $('fd-path').textContent = path;
}

function resetDetailsPanel() {
  ['fd-type', 'fd-perms', 'fd-owner', 'fd-group', 'fd-size', 'fd-modified', 'fd-path'].forEach(function (id) {
    $(id).textContent = '—';
  });
  $('fd-name').textContent = 'None selected';
}

// After chmod/chown, re-fetch just the parent listing so the one row's
// permissions/owner refresh without collapsing or reloading the rest of
// the tree the user has open.
async function refreshNodeDetails(path) {
  const node = treeNodesByPath[path];
  if (!node) return;
  const parentPath = path === treeRootPath ? null : path.slice(0, path.length - ('/' + node.entry.Name).length) || '/';
  try {
    const data = await call('list_dir', parentPath === null ? treeRootPath : parentPath);
    const updated = (data.entries || []).find(function (e) { return e.Name === node.entry.Name; });
    if (updated) {
      node.entry = updated;
      showNodeDetails(updated, path);
    }
  } catch (err) {
    // Non-fatal: the chmod/chown itself already succeeded and was announced.
  }
}

async function openFileEditorFor(path) {
  const node = treeNodesByPath[path];
  if (!node || node.entry.IsDir) {
    announce('Select a file, not a folder, to edit', true);
    return;
  }
  const useSudo = $('files-sudo').checked;
  $('file-editor-path').textContent = path;
  $('file-editor-status').textContent = 'Loading…';
  $('file-editor-sudo').checked = useSudo;
  $('file-editor-textarea').value = '';
  $('file-editor-textarea').dataset.path = path;
  openDialog('dlg-file-editor');
  try {
    const content = await call('read_file', path, useSudo);
    $('file-editor-textarea').value = content;
    $('file-editor-status').textContent = 'Loaded ' + content.length + ' characters';
    $('file-editor-textarea').focus();
  } catch (err) {
    $('file-editor-status').textContent = 'Error loading: ' + err.message;
  }
}

async function saveFileEditor() {
  const path = $('file-editor-textarea').dataset.path;
  const content = $('file-editor-textarea').value;
  const useSudo = $('file-editor-sudo').checked;
  $('file-editor-status').textContent = 'Saving…';
  try {
    await call('write_file', { path: path, content: content, use_sudo: useSudo });
    $('file-editor-status').textContent = 'Saved successfully';
    announce('File saved: ' + path);
  } catch (err) {
    $('file-editor-status').textContent = 'Error saving: ' + err.message;
    announce('Error saving file: ' + err.message, true);
  }
}

function showDiskUsage() {
  selectTab('resources');
  refreshResources();
  announce('Switched to the Resources tab to view disk usage');
}

// ============== Resources tab ==============

let resAutoTimer = null;

function initResourcesTab() {
  $('res-refresh').addEventListener('click', refreshResources);
  $('res-auto').addEventListener('change', function () {
    // Native checkbox semantics already announce checked/unchecked;
    // no announce() needed here (same reasoning as the tab fix above).
    if (this.checked) {
      resAutoTimer = window.setInterval(refreshResources, 5000);
    } else {
      if (resAutoTimer) window.clearInterval(resAutoTimer);
    }
  });
  refreshResources();
}

async function refreshResources() {
  try {
    const ri = await call('get_resources');
    const procs = await call('get_processes');
    const disk = await call('disk_usage');
    const memUsedMB = Math.round(ri.MemUsed / 1024 / 1024);
    const memTotalMB = Math.round(ri.MemTotal / 1024 / 1024);
    const memPct = ri.MemTotal > 0 ? ((ri.MemUsed / ri.MemTotal) * 100).toFixed(1) : '0';
    const swapUsedMB = Math.round(ri.SwapUsed / 1024 / 1024);
    const swapTotalMB = Math.round(ri.SwapTotal / 1024 / 1024);

    $('res-summary').innerHTML =
      '<dt>Uptime</dt><dd>' + escapeHTML(ri.Uptime || '') + '</dd>' +
      '<dt>CPU usage</dt><dd>' + ri.CPUPercent.toFixed(1) + '%</dd>' +
      '<dt>RAM</dt><dd>' + memUsedMB + ' MB used of ' + memTotalMB + ' MB (' + memPct + '%)</dd>' +
      '<dt>Swap</dt><dd>' + swapUsedMB + ' MB used of ' + swapTotalMB + ' MB</dd>';

    renderProcessTable(procs || []);
    renderDiskTable(disk || []);
  } catch (err) {
    announce('Error loading resources: ' + err.message, true);
  }
}

function renderProcessTable(procs) {
  const tbody = $('res-proc-tbody');
  tbody.innerHTML = '';
  procs.forEach(function (p) {
    const tr = document.createElement('tr');
    tr.innerHTML =
      '<td>' + escapeHTML(p.user) + '</td>' +
      '<td>' + escapeHTML(p.pid) + '</td>' +
      '<td>' + p.cpu.toFixed(1) + '</td>' +
      '<td>' + p.mem.toFixed(1) + '</td>' +
      '<td>' + escapeHTML(p.stat) + '</td>' +
      '<td>' + escapeHTML(p.start) + '</td>' +
      '<td>' + escapeHTML(p.time) + '</td>' +
      '<td>' + escapeHTML(p.command) + '</td>';
    tbody.appendChild(tr);
  });
}

function renderDiskTable(disks) {
  const tbody = $('res-disk-tbody');
  tbody.innerHTML = '';
  disks.forEach(function (d) {
    const tr = document.createElement('tr');
    tr.innerHTML =
      '<td>' + escapeHTML(d.filesystem) + '</td>' +
      '<td>' + escapeHTML(d.size) + '</td>' +
      '<td>' + escapeHTML(d.used) + '</td>' +
      '<td>' + escapeHTML(d.avail) + '</td>' +
      '<td>' + escapeHTML(d.use_percent) + '</td>' +
      '<td>' + escapeHTML(d.mounted_on) + '</td>';
    tbody.appendChild(tr);
  });
}

// ============== Apt tab (WebSocket: live progress + output) ==============

let aptSocket = null;
let aptRunning = false;

function initAptTab() {
  $('apt-update').addEventListener('click', function () { runApt('update'); });
  $('apt-upgrade').addEventListener('click', function () { runApt($('apt-upgrade-type').value); });
  $('apt-both').addEventListener('click', function () { runApt('update+upgrade'); });
  $('apt-cancel').addEventListener('click', cancelApt);
  $('apt-clear').addEventListener('click', function () {
    $('apt-output').value = '';
    $('apt-progress').value = 0;
    $('apt-progress-text').textContent = 'Idle';
    $('apt-status').textContent = 'Output cleared';
  });
}

function setAptBusy(busy) {
  aptRunning = busy;
  $('apt-update').disabled = busy;
  $('apt-upgrade').disabled = busy;
  $('apt-both').disabled = busy;
  $('apt-cancel').disabled = !busy;
}

function appendAptOutput(text) {
  const ta = $('apt-output');
  ta.value += text;
  ta.scrollTop = ta.scrollHeight;
  const lines = text.split(String.fromCharCode(10)).map(function (s) { return s.trim(); }).filter(Boolean);
  if (lines.length) {
    const last = lines[lines.length - 1];
    if (last.length < 140) $('apt-status').textContent = last;
  }
}

function runApt(operation) {
  if (aptRunning) return;
  setAptBusy(true);
  $('apt-progress').value = 0;
  $('apt-progress-text').textContent = 'Starting ' + operation + '…';
  appendAptOutput(String.fromCharCode(10) + '=== ' + operation + ' ===' + String.fromCharCode(10));
  announce('Starting apt ' + operation);

  aptSocket = new WebSocket(wsURL('/ws/apt'));
  aptSocket.onopen = function () {
    aptSocket.send(JSON.stringify({ operation: operation, config_action: $('apt-config-action').value }));
  };
  aptSocket.onmessage = function (ev) {
    let msg;
    try { msg = JSON.parse(ev.data); } catch (e) { return; }
    if (msg.type === 'output') {
      appendAptOutput(msg.data);
    } else if (msg.type === 'progress') {
      $('apt-progress').value = msg.pct;
      $('apt-progress-text').textContent = msg.label;
      announce(msg.label);
    } else if (msg.type === 'done') {
      setAptBusy(false);
      if (msg.data) {
        $('apt-status').textContent = 'Failed: ' + msg.data;
        announce('apt operation failed: ' + msg.data, true);
      } else {
        $('apt-progress').value = 100;
        $('apt-progress-text').textContent = 'Complete';
        $('apt-status').textContent = operation + ' completed successfully';
        announce(operation + ' completed successfully');
      }
    }
  };
  aptSocket.onerror = function () {
    setAptBusy(false);
    announce('Connection error during apt operation', true);
  };
  aptSocket.onclose = function () { setAptBusy(false); };
}

function cancelApt() {
  if (aptSocket) aptSocket.close();
  setAptBusy(false);
  appendAptOutput(String.fromCharCode(10) + '[Cancelled by user]' + String.fromCharCode(10));
  announce('apt operation cancelled', true);
}

// ============== Terminal tab (WebSocket: live stdin/stdout) ==============

function initTerminalTab() {
  $('term-form').addEventListener('submit', function (e) {
    e.preventDefault();
    runTerminalCommand();
  });
  $('term-clear').addEventListener('click', function () { $('term-output').value = ''; });
}

function appendTermOutput(text) {
  const ta = $('term-output');
  ta.value += text;
  ta.scrollTop = ta.scrollHeight;
}

function runTerminalCommand() {
  const cmd = $('term-cmd').value.trim();
  if (!cmd) return;
  const useSudo = $('term-sudo').checked;
  $('term-cmd').value = '';
  appendTermOutput(String.fromCharCode(10) + '$ ' + cmd + String.fromCharCode(10));
  setStatus('Running: ' + cmd);

  const sock = new WebSocket(wsURL('/ws/terminal'));
  sock.onopen = function () {
    sock.send(JSON.stringify({ cmd: cmd, use_sudo: useSudo }));
  };
  sock.onmessage = function (ev) {
    let msg;
    try { msg = JSON.parse(ev.data); } catch (e) { return; }
    if (msg.type === 'output') {
      appendTermOutput(msg.data);
    } else if (msg.type === 'done') {
      if (msg.error) {
        appendTermOutput(String.fromCharCode(10) + '[error: ' + msg.error + ']' + String.fromCharCode(10));
        setStatus('Command failed');
      } else {
        setStatus('Command completed');
      }
    }
  };
  sock.onerror = function () { setStatus('Connection error'); };
}

// ============== Init ==============

// Returns the element that should receive keyboard focus right now: the
// currently focused element if there is one and it's still visible/in the
// document, otherwise a sensible default per screen (first field on the
// connect screen, the active tab button in a session).
function defaultFocusTarget() {
  if (!$('screen-session').hidden) {
    const activeTab = document.querySelector('.tab-btn[aria-selected="true"]');
    return activeTab || $('tab-services');
  }
  return $('field-hostname');
}

function refocusContent() {
  const active = document.activeElement;
  // If focus already landed somewhere real inside the page, leave it.
  if (active && active !== document.body && active !== document.documentElement) {
    return;
  }
  const target = defaultFocusTarget();
  if (target) target.focus();
}

document.addEventListener('DOMContentLoaded', function () {
  loadConnectScreenData();
  $('field-hostname').focus();

  // WebView-embedded apps (this one included) can lose keyboard focus on
  // the actual page content when the OS window is minimized, Alt-Tabbed
  // away from, and then back -- the top-level window gets OS focus again,
  // but the document inside the WebView control doesn't automatically
  // get focus restored to a specific element, which leaves NVDA's
  // browse-mode cursor with nothing to anchor to until the user clicks.
  // Re-focusing a sensible default element on these events closes most
  // of that gap from the JS side; main.go's native SetFocus/grab_focus
  // call on window-shown handles getting the WebView *control* itself
  // OS keyboard focus in the first place.
  window.addEventListener('focus', refocusContent);
  document.addEventListener('visibilitychange', function () {
    if (document.visibilityState === 'visible') refocusContent();
  });
});
`
