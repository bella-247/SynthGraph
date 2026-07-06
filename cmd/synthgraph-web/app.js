// State
let parsedModel = null;
let lastResult = null;
let lastJobData = null;
let resultTab = 'preview';

// Navigation
function showPage(name) {
  document.querySelectorAll('.page').forEach(function(p) { p.classList.remove('active'); });
  var page = document.getElementById('page-' + name);
  if (page) page.classList.add('active');
  document.querySelectorAll('nav a').forEach(function(a) {
    a.classList.remove('active');
    a.setAttribute('aria-selected', 'false');
    a.setAttribute('tabindex', '-1');
  });
  var el = document.getElementById('nav-' + name);
  if (el) {
    el.classList.add('active');
    el.setAttribute('aria-selected', 'true');
    el.setAttribute('tabindex', '0');
  }
  if (name === 'history') loadHistory();
  if (name === 'graph') {
    if (parsedModel) renderGraph();
    else document.getElementById('cy').innerHTML = '<div class="loading-block" style="display:flex"><div style="color:var(--text3);font-size:13px">Parse a schema first to see the graph</div></div>';
  }
}

// Keyboard nav
document.querySelector('nav').addEventListener('keydown', function(e) {
  if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return;
  var tabs = Array.from(this.querySelectorAll('[role="tab"]'));
  var current = tabs.findIndex(function(a) { return a.classList.contains('active'); });
  var next = e.key === 'ArrowRight' ? (current + 1) % tabs.length : (current - 1 + tabs.length) % tabs.length;
  showPage(tabs[next].id.replace('nav-', ''));
  tabs[next].focus();
});

// Toast
function toast(msg, type) {
  type = type || 'info';
  var container = document.getElementById('toast-container');
  var t = document.createElement('div');
  t.className = 'toast ' + type;
  var icon = type === 'success' ? '\u2713' : type === 'error' ? '\u2717' : '\u24D8';
  t.innerHTML = '<span>' + icon + '</span><span>' + escapeHTML(msg) + '</span>';
  container.appendChild(t);
  setTimeout(function() { if (t.parentNode) t.remove(); }, 4000);
}

// File upload
function loadFile(e) {
  var file = e.target.files[0];
  if (!file) return;
  var reader = new FileReader();
  reader.onload = function(ev) {
    document.getElementById('sql-input').value = ev.target.result;
    clearFieldError('sql-input-group');
    toast('File loaded: ' + file.name, 'info');
  };
  reader.readAsText(file);
}

function clearFieldError(groupId) {
  var group = document.getElementById(groupId);
  if (group) group.classList.remove('invalid');
}

function showFieldError(groupId, message) {
  var group = document.getElementById(groupId);
  if (!group) return;
  group.classList.add('invalid');
  var errorEl = group.querySelector('.field-error');
  if (errorEl) errorEl.textContent = message;
}

function setLoading(btnId, loading) {
  var btn = document.getElementById(btnId);
  if (!btn) return;
  btn.disabled = loading;
  if (loading) {
    btn.dataset.originalText = btn.textContent;
    btn.innerHTML = '<span class="loading-spinner loading-spinner-sm"></span> ' + btn.textContent.trim();
  } else {
    btn.innerHTML = btn.dataset.originalText || btn.textContent;
  }
}

// Parse Schema
async function parseSchema() {
  var sql = document.getElementById('sql-input').value.trim();
  clearFieldError('sql-input-group');
  if (!sql) { showFieldError('sql-input-group', 'Enter SQL DDL first'); return; }
  setLoading('parse-btn', true);
  try {
    var res = await fetch('/api/parse', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({sql: sql})
    });
    var data = await res.json();
    if (!res.ok) {
      toast(extractError(data), 'error');
      var summaryEl = document.getElementById('parse-result');
      document.getElementById('parse-summary').style.display = 'block';
      summaryEl.innerHTML = '<div style="color:var(--red);display:flex;align-items:center;gap:6px"><span>\u2717</span><span>' + escapeHTML(extractError(data)) + '</span></div>';
      return;
    }
    parsedModel = data.model;
    document.getElementById('gen-sql').value = sql;

    // Summary
    var warnHtml = '';
    if (data.warnings && data.warnings.length) {
      warnHtml = '<div style="margin-top:8px"><div class="flex gap-4 text-sm text-2 mb-8" style="color:var(--yellow)"><span>\u26A0</span> Warnings</div>';
      data.warnings.forEach(function(w) { warnHtml += '<div class="text-sm" style="color:var(--text2);padding:2px 0;padding-left:16px)">' + escapeHTML(w) + '</div>'; });
      warnHtml += '</div>';
    }
    var summaryDiv = document.getElementById('parse-result');
    document.getElementById('parse-summary').style.display = 'block';
    summaryDiv.innerHTML = '<div class="flex gap-12 text-sm"><span class="badge badge-blue">' + data.tables + ' tables</span><span class="badge badge-gray">' + data.enums + ' enums</span></div>' + warnHtml;
    toast('Parsed ' + data.tables + ' tables', 'success');

    // Detailed schema view
    renderSchemaDetails(data.model);
    document.getElementById('parse-detail').style.display = 'block';
    document.getElementById('table-count-badge').textContent = data.tables + ' tables';
  } catch(err) {
    toast('Network error: ' + err.message, 'error');
  } finally {
    setLoading('parse-btn', false);
  }
}

function renderSchemaDetails(model) {
  var container = document.getElementById('schema-tables');
  container.innerHTML = '';
  if (!model || !model.tables) {
    container.innerHTML = '<div class="text-sm text-2">No tables found in schema.</div>';
    return;
  }
  model.tables.forEach(function(table, idx) {
    var cols = table.columns || [];
    var fkCount = (table.foreign_keys || []).length;
    var checkCount = (table.checks || []).length;
    var colCount = cols.length;

    // Build FK column set from table-level foreign key definitions
    var fkCols = {};
    (table.foreign_keys || []).forEach(function(fk) {
      (fk.columns || []).forEach(function(c) { fkCols[c] = true; });
    });
    // Build unique column set from table-level unique constraints
    var uniqueCols = {};
    (table.unique || []).forEach(function(group) {
      (group || []).forEach(function(c) { uniqueCols[c] = true; });
    });

    var headerBadges = '<span>' + colCount + ' col' + (colCount !== 1 ? 's' : '') + '</span>';
    if (fkCount) headerBadges += '<span class="badge badge-blue small" style="font-size:10px;padding:0 5px">' + fkCount + ' FK</span>';
    if (checkCount) headerBadges += '<span class="badge badge-orange small" style="font-size:10px;padding:0 5px">' + checkCount + ' CHECK</span>';

    var card = document.createElement('div');
    card.className = 'schema-table';
    card.innerHTML =
      '<div class="schema-table-header collapsed" onclick="toggleSchemaTable(this)" role="button" tabindex="0" aria-expanded="false">' +
        '<div class="table-name">' +
          '<span class="expand-icon">\u25BC</span>' +
          '<span>' + escapeHTML(table.name) + '</span>' +
        '</div>' +
        '<div class="table-meta">' + headerBadges + '</div>' +
      '</div>' +
      '<div class="schema-table-body collapsed">' +
        '<table><thead><tr><th>Column</th><th>Type</th><th>Constraints</th><th>Default</th></tr></thead><tbody id="schema-body-' + idx + '"></tbody></table>' +
      '</div>';
    container.appendChild(card);

    // Build rows
    var tbody = document.getElementById('schema-body-' + idx);
    cols.forEach(function(col) {
      var constraints = [];
      if (col.is_primary_key) constraints.push('<span class="col-pk">PK</span>');
      if (fkCols[col.name]) constraints.push('<span class="col-fk">FK</span>');
      if (uniqueCols[col.name]) constraints.push('<span class="col-unique">UNIQUE</span>');
      if (!col.nullable) constraints.push('<span class="col-not-null">NOT NULL</span>');

      var typeDisplay = escapeHTML(col.type);
      if (col.length > 0) typeDisplay += '(' + col.length + (col.precision > 0 ? ',' + col.precision : '') + ')';

      var semTag = '';
      if (col.semantic) semTag = '<span class="col-semantic">' + escapeHTML(col.semantic) + '</span>';

      var defaultVal = col.default || '';
      if (defaultVal) defaultVal = '<span class="col-default">' + escapeHTML(defaultVal) + '</span>';

      var tr = document.createElement('tr');
      tr.innerHTML = '<td style="font-weight:500">' + escapeHTML(col.name) + semTag + '</td>' +
        '<td style="color:var(--text2);font-family:monospace;font-size:12px">' + typeDisplay + '</td>' +
        '<td>' + constraints.join(' ') + '</td>' +
        '<td>' + defaultVal + '</td>';
      tbody.appendChild(tr);
    });

    // CHECK constraints section
    if (checkCount > 0) {
      var checkRow = document.createElement('tr');
      var checkHtml = '<td colspan="4" style="padding:6px 14px;background:var(--bg3)">';
      checkHtml += '<div style="font-size:11px;color:var(--orange);margin-bottom:4px"><span>\u2691</span> CHECK constraints</div>';
      (table.checks || []).forEach(function(chk) {
        checkHtml += '<div style="font-size:11px;color:var(--text2);font-family:monospace;padding:1px 0">' +
          (chk.name ? escapeHTML(chk.name) + ': ' : '') + escapeHTML(chk.expression) + '</div>';
      });
      checkHtml += '</td>';
      checkRow.innerHTML = checkHtml;
      tbody.appendChild(checkRow);
    }
  });
}

function toggleSchemaTable(header) {
  header.classList.toggle('collapsed');
  var body = header.nextElementSibling;
  body.classList.toggle('collapsed');
  var expanded = !body.classList.contains('collapsed');
  header.setAttribute('aria-expanded', expanded);
}

// Graph
async function renderGraph() {
  var cyEl = document.getElementById('cy');
  var loadingEl = document.getElementById('graph-loading');
  if (!parsedModel) {
    cyEl.innerHTML = '<div class="loading-block" style="display:flex"><div style="color:var(--text3);font-size:13px">Parse a schema first</div></div>';
    return;
  }
  if (window.cyInstance) {
    window.cyInstance.destroy();
    window.cyInstance = null;
  }
  loadingEl.style.display = 'flex';
  cyEl.innerHTML = '';
  try {
    var res = await fetch('/api/graph', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({model: parsedModel})
    });
    var gData = await res.json();
    if (!res.ok) { toast(extractError(gData), 'error'); loadingEl.style.display = 'none'; return; }
    var semRes = await fetch('/api/semantic', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({model: parsedModel})
    });
    var semData = await semRes.json();
    loadingEl.style.display = 'none';
    var roleMap = {};
    if (semData.nodes) {
      semData.nodes.forEach(function(n) { roleMap[n.id] = n.roles || []; });
    }
    function nodeColor(roles) {
      if (roles.indexOf('junction') !== -1) return '#d4761a';
      if (roles.indexOf('hierarchical') !== -1) return '#4c9aff';
      if (roles.indexOf('transactional') !== -1) return '#3ecf8e';
      if (roles.indexOf('lookup') !== -1) return '#e5b83c';
      return '#6b7d96';
    }
    var elements = [];
    gData.nodes.forEach(function(n) {
      var roles = roleMap[n.table] || [];
      elements.push({
        data: {
          id: n.id,
          label: n.table,
          roles: roles.join(', '),
          columns: n.columns,
          isJunction: n.is_junction
        },
        style: {'background-color': nodeColor(roles), width: 60, height: 60}
      });
    });
    gData.edges.forEach(function(e) {
      elements.push({
        data: {id: e.id, source: e.source, target: e.target, label: e.label},
        style: {'line-style': e.nullable ? 'dashed' : 'solid'}
      });
    });
    window.cyInstance = cytoscape({
      container: document.getElementById('cy'),
      elements: elements,
      style: [
        { selector: 'node', style: {
          'label': 'data(label)', 'font-size': '11px',
          'text-valign': 'bottom', 'text-halign': 'center',
          'color': '#dfe6f0', 'text-wrap': 'wrap', 'text-max-width': '80px',
          'border-width': 2, 'border-color': '#2a3442',
          'transition-property': 'background-color, border-color',
          'transition-duration': '0.2s'
        }},
        { selector: 'node:selected', style: {
          'border-color': '#fff', 'border-width': 3
        }},
        { selector: 'edge', style: {
          'width': 2, 'line-color': '#4c9aff',
          'target-arrow-color': '#4c9aff', 'target-arrow-shape': 'triangle',
          'curve-style': 'bezier', 'label': 'data(label)', 'font-size': '9px',
          'color': '#9aa8be', 'text-rotation': 'autorotate'
        }},
        { selector: 'edge[?nullable]', style: {
          'line-style': 'dashed'
        }}
      ],
      layout: { name: 'cose', animate: false, nodeSeparation: 120, gravity: 0.8 }
    });
    window.cyInstance.on('tap', 'node', function(evt) {
      var node = evt.target;
      var nData = gData.nodes.find(function(n) { return n.id === node.id(); });
      if (!nData) return;
      var roles = roleMap[nData.table] || [];
      var edges = gData.edges.filter(function(e) { return e.source === node.id() || e.target === node.id(); });
      var html = '<div class="panel-section"><h4>Table</h4><div class="detail-table">' +
        '<table><tr><td>Name</td><td style="font-weight:600">' + escapeHTML(nData.table) + '</td></tr>' +
        '<tr><td>Columns</td><td>' + nData.columns + '</td></tr>' +
        '<tr><td>Junction</td><td>' + (nData.is_junction ? '<span class="badge badge-orange">Yes</span>' : '<span class="badge badge-gray">No</span>') + '</td></tr>' +
        '<tr><td>Roles</td><td>' + (roles.length ? '<span class="badge ' + roleBadgeClass(roles[0]) + '">' + escapeHTML(roles.join(', ')) + '</span>' : '<span class="text-3">&mdash;</span>') + '</td></tr>' +
        '</table></div>';
      if (edges.length) {
        html += '<div class="panel-section" style="margin-top:16px"><h4>Relationships (' + edges.length + ')</h4><div class="detail-table"><table>';
        edges.forEach(function(e) {
          var src = gData.nodes.find(function(n) { return n.id === e.source; });
          var tgt = gData.nodes.find(function(n) { return n.id === e.target; });
          html += '<tr><td style="font-size:12px">' + (src ? escapeHTML(src.table) : e.source) + '</td><td style="font-size:12px;color:var(--text3)">\u2192</td><td style="font-size:12px">' + (tgt ? escapeHTML(tgt.table) : e.target) + '</td></tr>';
        });
        html += '</table></div>';
      }
      html += '<div class="panel-section" style="margin-top:16px">';
      if (nData.columns && nData.columns.length) {
        html += '<h4>Columns</h4><div class="detail-table"><table>';
        nData.columns.forEach(function(c) {
          html += '<tr><td style="font-size:12px">' + escapeHTML(c.name || c) + '</td><td style="font-size:12px;color:var(--text2)">' + escapeHTML(c.type || '') + '</td></tr>';
        });
        html += '</table></div>';
      }
      html += '</div>';
      document.getElementById('detail-content').innerHTML = html;
      document.getElementById('detail-panel').classList.add('open');
    });
    // Semantic roles
    if (semData.nodes && semData.nodes.length) {
      var semEl = document.getElementById('semantic-content');
      var sHtml = '<div class="table-wrap"><table><thead><tr><th>Table</th><th>Role</th><th>Columns</th></tr></thead><tbody>';
      semData.nodes.forEach(function(n) {
        if (n.roles && n.roles.length) {
          var rc = roleBadgeClass(n.roles[0]);
          sHtml += '<tr><td style="font-weight:500">' + escapeHTML(n.id) + '</td><td>' + n.roles.map(function(r) { return '<span class="badge ' + roleBadgeClass(r) + '">' + escapeHTML(r) + '</span>'; }).join(' ') + '</td><td style="color:var(--text2)">' + (n.columns || '') + '</td></tr>';
        }
      });
      sHtml += '</tbody></table></div>';
      if (semData.edges && semData.edges.length) {
        sHtml += '<h4 style="margin-top:16px;font-size:13px;color:var(--text2);margin-bottom:8px">Relationships</h4><div class="table-wrap"><table><thead><tr><th>From</th><th>To</th><th>Kind</th></tr></thead><tbody>';
        semData.edges.forEach(function(e) {
          sHtml += '<tr><td style="font-weight:500">' + escapeHTML(e.from) + '</td><td style="font-weight:500">' + escapeHTML(e.to) + '</td><td style="color:var(--text2)">' + escapeHTML(e.kind) + '</td></tr>';
        });
        sHtml += '</tbody></table></div>';
      }
      semEl.innerHTML = sHtml;
      document.getElementById('semantic-card').style.display = 'block';
    }
  } catch(err) {
    loadingEl.style.display = 'none';
    cyEl.innerHTML = '<div class="loading-block" style="display:flex;color:var(--red)"><span>\u2717</span>Error: ' + escapeHTML(err.message) + '</div>';
    toast('Graph failed to load', 'error');
  }
}

function roleBadgeClass(role) {
  if (role === 'junction') return 'badge-orange';
  if (role === 'hierarchical') return 'badge-blue';
  if (role === 'transactional') return 'badge-green';
  if (role === 'lookup') return 'badge-yellow';
  return 'badge-gray';
}

function fitGraph() {
  if (window.cyInstance) {
    window.cyInstance.fit(undefined, 40);
  }
}

function resetGraphLayout() {
  if (window.cyInstance) {
    window.cyInstance.layout({ name: 'cose', animate: true, animationDuration: 500, nodeSeparation: 120, gravity: 0.8 }).run();
  }
}

// Generate
function runGeneration() {
  var sql = document.getElementById('gen-sql').value.trim();
  clearFieldError('gen-sql-group');
  if (!sql) { showFieldError('gen-sql-group', 'Enter SQL DDL first'); return; }
  var rows = parseInt(document.getElementById('gen-rows').value) || 10;
  if (rows < 1) { rows = 1; document.getElementById('gen-rows').value = 1; }
  if (rows > 100000) { rows = 100000; document.getElementById('gen-rows').value = 100000; }
  var seed = parseInt(document.getElementById('gen-seed').value) || 42;
  var format = document.getElementById('gen-format').value;
  var schemaName = document.getElementById('gen-schema-name').value;
  var progressEl = document.getElementById('gen-progress');
  var barEl = document.getElementById('gen-bar');
  var statusEl = document.getElementById('gen-status');
  var resultEl = document.getElementById('gen-result');
  var stageEl = document.getElementById('gen-stage');
  var genBtn = document.getElementById('generate-btn');
  var pipelineEl = document.getElementById('gen-pipeline');

  progressEl.style.display = 'block';
  resultEl.style.display = 'none';
  barEl.style.width = '0%';
  barEl.className = 'progress-fill';
  stageEl.textContent = 'Starting...';
  statusEl.textContent = '';
  genBtn.disabled = true;
  genBtn.innerHTML = '<span class="loading-spinner loading-spinner-sm"></span> Generating...';

  var params = new URLSearchParams({
    input: sql, rows: rows, seed: seed, format: format, schema_name: schemaName
  });
  var es = new EventSource('/api/generate/stream?' + params.toString());
  var lastStage = '';
  var stages = ['Parsing', 'Analyzing', 'Generating', 'Exporting'];

  // Pipeline visualization
  pipelineEl.style.display = 'flex';
  pipelineEl.innerHTML = stages.map(function(s) {
    return '<div class="pipeline-step" data-stage="' + s.toLowerCase() + '">' +
      '<span class="step-icon">&#9711;</span>' +
      '<span class="step-label">' + s + '</span></div>';
  }).join('');

  es.addEventListener('stage', function(e) {
    var data = JSON.parse(e.data);
    lastStage = data.stage;
    var stageKey = data.stage.toLowerCase();
    if (data.status === 'processing') {
      stageEl.textContent = data.stage.charAt(0).toUpperCase() + data.stage.slice(1) + '...';
      pipelineEl.querySelectorAll('.pipeline-step').forEach(function(s) {
        var stepStage = s.getAttribute('data-stage');
        if (stepStage === stageKey) {
          s.className = 'pipeline-step active';
          s.querySelector('.step-icon').textContent = '\u25CF';
        } else if (stages.indexOf(stageKey) > stages.indexOf(stepStage)) {
          s.className = 'pipeline-step done';
          s.querySelector('.step-icon').textContent = '\u2713';
        }
      });
    } else if (data.status === 'done') {
      stageEl.textContent = data.stage.charAt(0).toUpperCase() + data.stage.slice(1) + ' done';
      pipelineEl.querySelectorAll('.pipeline-step').forEach(function(s) {
        if (s.getAttribute('data-stage') === stageKey) {
          s.className = 'pipeline-step done';
          s.querySelector('.step-icon').textContent = '\u2713';
        }
      });
    }
  });

  es.addEventListener('progress', function(e) {
    var data = JSON.parse(e.data);
    var currentTable = data.current;
    var totalTables = data.total;
    var pct = Math.round((currentTable / totalTables) * 100);
    barEl.style.width = pct + '%';
    statusEl.textContent = 'Generating ' + escapeHTML(data.table) + ' (' + currentTable + '/' + totalTables + ')';
  });

  es.addEventListener('complete', function(e) {
    var data = JSON.parse(e.data);
    es.close();
    lastResult = data;
    lastJobData = data.data;
    barEl.style.width = '100%';
    barEl.className = 'progress-fill success';
    stageEl.textContent = 'Complete';
    statusEl.textContent = 'Generated ' + data.tables + ' table(s)';
    if (data.errors && data.errors.length) {
      statusEl.textContent += ' with ' + data.errors.length + ' warning(s)';
    }
    genBtn.disabled = false;
    genBtn.textContent = 'Generate';
    toast('Generated ' + data.tables + ' table(s)', 'success');

    // Show result
    showGenerationResult(data);
    loadHistory();
  });

  es.addEventListener('error', function(e) {
    if (e.eventPhase === EventSource.CLOSED || e.data) {
      var msg = 'Generation failed';
      try {
        var parsed = JSON.parse(e.data);
        if (parsed && parsed.message) msg = parsed.message;
      } catch(_) {
        if (lastStage) msg = 'Connection lost during ' + lastStage;
      }
      es.close();
      barEl.style.width = '100%';
      barEl.className = 'progress-fill failed';
      stageEl.textContent = 'Failed';
      statusEl.textContent = msg;
      pipelineEl.querySelectorAll('.pipeline-step').forEach(function(s) {
        var stepStage = s.getAttribute('data-stage');
        if (stepStage === lastStage.toLowerCase()) {
          s.className = 'pipeline-step failed';
          s.querySelector('.step-icon').textContent = '\u2717';
        }
      });
      genBtn.disabled = false;
      genBtn.textContent = 'Generate';
      toast(msg, 'error');
    } else {
      stageEl.textContent = 'Reconnecting...';
      statusEl.textContent = 'Connection lost, retrying...';
    }
  });
}

function showGenerationResult(data) {
  var resultEl = document.getElementById('gen-result');
  resultEl.style.display = 'block';

  // Meta
  var metaEl = document.getElementById('result-meta');
  metaEl.innerHTML = '<span class="badge badge-blue">Job #' + data.job_id + '</span><span>' + data.tables + ' tables</span><span>' + (data.format || '').toUpperCase() + '</span>';
  if (data.errors && data.errors.length) {
    metaEl.innerHTML += '<span style="color:var(--yellow)">' + data.errors.length + ' warning(s)</span>';
  }

  // Warnings
  var warningsHtml = '';
  if (data.errors && data.errors.length) {
    warningsHtml = '<div style="margin:8px 0;padding:8px 12px;background:var(--yellow-bg);border-radius:var(--radius-sm);border:1px solid rgba(229,184,60,0.2)">';
    data.errors.forEach(function(e) { warningsHtml += '<div style="font-size:12px;color:var(--yellow)"><span>\u26A0</span> ' + escapeHTML(e) + '</div>'; });
    warningsHtml += '</div>';
  }

  // Preview tab
  var previewHtml = warningsHtml;
  var lines = data.data.split('\n').filter(function(l) { return l.trim(); });
  var previewLines = lines.slice(0, 21);
  var csvLines = previewLines.map(function(l) { return parseCSVLine(l); });
  if (csvLines.length > 0) {
    previewHtml += '<div class="data-preview"><table><thead><tr>';
    (csvLines[0] || []).forEach(function(h) { previewHtml += '<th>' + escapeHTML(h) + '</th>'; });
    previewHtml += '</tr></thead><tbody>';
    for (var i = 1; i < csvLines.length; i++) {
      previewHtml += '<tr>';
      (csvLines[i] || []).forEach(function(c) { previewHtml += '<td>' + escapeHTML(c) + '</td>'; });
      previewHtml += '</tr>';
    }
    previewHtml += '</tbody></table></div>';
  }
  document.getElementById('result-preview').innerHTML = previewHtml;

  // Raw tab
  document.getElementById('result-raw-text').value = data.data;

  // Reset tabs
  resultTab = 'preview';
  document.querySelectorAll('#gen-result .tab').forEach(function(t) { t.classList.remove('active'); t.setAttribute('aria-selected', 'false'); });
  document.querySelector('#gen-result .tab').classList.add('active');
  document.querySelector('#gen-result .tab').setAttribute('aria-selected', 'true');
  document.getElementById('result-preview').style.display = 'block';
  document.getElementById('result-raw').style.display = 'none';
}

function switchResultTab(tab, el) {
  resultTab = tab;
  document.querySelectorAll('#gen-result .tab').forEach(function(t) { t.classList.remove('active'); t.setAttribute('aria-selected', 'false'); });
  el.classList.add('active');
  el.setAttribute('aria-selected', 'true');
  document.getElementById('result-preview').style.display = tab === 'preview' ? 'block' : 'none';
  document.getElementById('result-raw').style.display = tab === 'raw' ? 'block' : 'none';
}

function copyResult() {
  var text = lastJobData || '';
  if (!text) { toast('No data to copy', 'error'); return; }
  navigator.clipboard.writeText(text).then(function() {
    toast('Copied to clipboard', 'success');
  }).catch(function() {
    toast('Failed to copy', 'error');
  });
}

function downloadResult() {
  var data = lastJobData || (lastResult ? lastResult.data : null);
  if (!data) { toast('No data available', 'error'); return; }
  var format = (lastResult && lastResult.format) || 'csv';
  var jobId = (lastResult && lastResult.job_id) || 'download';
  var blob = new Blob([data], {type: 'text/plain'});
  var url = URL.createObjectURL(blob);
  var a = document.createElement('a');
  a.href = url;
  a.download = 'synthgraph-job-' + jobId + '.' + format;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
  toast('Downloaded ' + a.download, 'success');
}

function parseCSVLine(line) {
  var result = [], current = '', inQuotes = false;
  for (var i = 0; i < line.length; i++) {
    var c = line[i];
    if (c === '"') { inQuotes = !inQuotes; continue; }
    if (c === ',' && !inQuotes) { result.push(current); current = ''; continue; }
    current += c;
  }
  result.push(current);
  return result;
}

function escapeHTML(s) {
  var d = document.createElement('div');
  d.textContent = s || '';
  return d.innerHTML;
}

function extractError(data) {
  if (!data) return 'Unknown error';
  if (typeof data.error === 'string') return data.error;
  if (data.error && data.error.message) return data.error.message;
  if (data.message) return data.message;
  return 'Unknown error';
}

// History
async function loadHistory() {
  var loadingEl = document.getElementById('history-loading');
  var tbody = document.getElementById('history-body');
  var empty = document.getElementById('history-empty');
  loadingEl.style.display = 'flex';
  tbody.innerHTML = '';
  try {
    var res = await fetch('/api/jobs');
    var jobs = await res.json();
    loadingEl.style.display = 'none';
    if (!jobs || jobs.length === 0) {
      empty.style.display = 'block';
      return;
    }
    empty.style.display = 'none';
    jobs.forEach(function(j) {
      var statusClass = j.status === 'failed' ? 'badge-red' : (j.errors && j.errors.length ? 'badge-yellow' : 'badge-green');
      var statusLabel = j.status === 'failed' ? 'Failed' : (j.errors && j.errors.length ? 'Partial' : 'Completed');
      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td style="font-weight:500;font-variant-numeric:tabular-nums">#' + j.id + '</td>' +
        '<td style="font-variant-numeric:tabular-nums">' + new Date(j.created).toLocaleString() + '</td>' +
        '<td>' + j.tables + '</td>' +
        '<td>' + (j.format || '').toUpperCase() + '</td>' +
        '<td style="font-variant-numeric:tabular-nums">' + (j.rows || '') + '</td>' +
        '<td><span class="badge ' + statusClass + '">' + statusLabel + '</span></td>' +
        '<td><div class="flex gap-4">' +
          '<button class="small" onclick="viewJob(' + j.id + ')" title="View result">View</button>' +
          '<button class="small" onclick="downloadJob(' + j.id + ')" title="Download data">DL</button>' +
          '<button class="small danger" onclick="deleteJob(' + j.id + ')" title="Delete job">Del</button>' +
        '</div></td>';
      tbody.appendChild(tr);
    });
  } catch(err) {
    loadingEl.style.display = 'none';
    empty.style.display = 'none';
    tbody.innerHTML = '<tr><td colspan="7" style="color:var(--red);text-align:center">Failed to load: ' + escapeHTML(err.message) + '</td></tr>';
    toast('Failed to load history', 'error');
  }
}

function viewJob(jobId) {
  showPage('generate');
  var resultEl = document.getElementById('gen-result');
  var progressEl = document.getElementById('gen-progress');
  var pipelineEl = document.getElementById('gen-pipeline');
  pipelineEl.style.display = 'none';
  progressEl.style.display = 'none';
  resultEl.style.display = 'block';
  resultEl.innerHTML = '<div class="loading-block" style="display:flex"><span class="loading-spinner"></span>Loading job #' + jobId + '...</div>';
  fetch('/api/jobs/' + jobId).then(function(res) {
    if (!res.ok) { throw new Error('Job not found'); }
    return res.json();
  }).then(function(job) {
    lastResult = job;
    lastJobData = job.data;
    showGenerationResult(job);
  }).catch(function(err) {
    resultEl.innerHTML = '<div style="padding:20px;text-align:center;color:var(--red)">Error: ' + escapeHTML(err.message) + '</div>';
    toast('Failed to load job', 'error');
  });
}

function downloadJob(jobId) {
  fetch('/api/jobs/' + jobId).then(function(res) {
    if (!res.ok) throw new Error('Job not found');
    return res.json();
  }).then(function(job) {
    if (!job.data) { toast('No data in job #' + jobId, 'error'); return; }
    var format = job.format || 'csv';
    var blob = new Blob([job.data], {type: 'text/plain'});
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'synthgraph-job-' + jobId + '.' + format;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast('Downloaded job #' + jobId, 'success');
  }).catch(function(err) {
    toast('Error: ' + err.message, 'error');
  });
}

function deleteJob(jobId) {
  if (!confirm('Delete job #' + jobId + '?')) return;
  fetch('/api/jobs/' + jobId, {method: 'DELETE'}).then(function(res) {
    if (!res.ok) throw new Error('Delete failed');
    toast('Deleted job #' + jobId, 'success');
    loadHistory();
  }).catch(function(err) {
    toast('Error: ' + err.message, 'error');
  });
}

function closePanel() {
  document.getElementById('detail-panel').classList.remove('open');
}

// Init: load history if on that tab initially
(function() {
  var activePage = document.querySelector('.page.active');
  if (activePage && activePage.id === 'page-history') loadHistory();
})();
