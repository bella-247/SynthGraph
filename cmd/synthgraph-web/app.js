var parsedModel = null;
var lastResult = null;
var lastJobData = null;
var resultTab = 'preview';

var TEMPLATES = {
  ecommerce: [
    "CREATE TABLE categories (",
    "  id SERIAL PRIMARY KEY,",
    "  name VARCHAR(100) NOT NULL,",
    "  slug VARCHAR(100) UNIQUE NOT NULL,",
    "  description TEXT,",
    "  parent_id INT REFERENCES categories(id)",
    ");",
    "",
    "CREATE TABLE products (",
    "  id SERIAL PRIMARY KEY,",
    "  name VARCHAR(200) NOT NULL,",
    "  sku VARCHAR(50) UNIQUE NOT NULL,",
    "  price DECIMAL(10,2) NOT NULL CHECK (price > 0),",
    "  category_id INT NOT NULL REFERENCES categories(id),",
    "  stock INT NOT NULL DEFAULT 0,",
    "  active BOOLEAN NOT NULL DEFAULT true,",
    "  created_at TIMESTAMP DEFAULT NOW()",
    ");",
    "",
    "CREATE TABLE customers (",
    "  id SERIAL PRIMARY KEY,",
    "  first_name VARCHAR(100) NOT NULL,",
    "  last_name VARCHAR(100) NOT NULL,",
    "  email VARCHAR(255) UNIQUE NOT NULL,",
    "  city VARCHAR(100),",
    "  state VARCHAR(50),",
    "  zip VARCHAR(20),",
    "  registered_at TIMESTAMP DEFAULT NOW()",
    ");",
    "",
    "CREATE TABLE orders (",
    "  id SERIAL PRIMARY KEY,",
    "  customer_id INT NOT NULL REFERENCES customers(id),",
    "  order_date TIMESTAMP DEFAULT NOW(),",
    "  total DECIMAL(12,2) NOT NULL,",
    "  status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','shipped','delivered','cancelled')),",
    "  shipping_city VARCHAR(100),",
    "  shipping_state VARCHAR(50),",
    "  shipping_zip VARCHAR(20)",
    ");",
    "",
    "CREATE TABLE order_items (",
    "  id SERIAL PRIMARY KEY,",
    "  order_id INT NOT NULL REFERENCES orders(id),",
    "  product_id INT NOT NULL REFERENCES products(id),",
    "  quantity INT NOT NULL CHECK (quantity > 0),",
    "  unit_price DECIMAL(10,2) NOT NULL,",
    "  UNIQUE(order_id, product_id)",
    ");"
  ].join('\n'),
  blog: [
    "CREATE TABLE authors (",
    "  id SERIAL PRIMARY KEY,",
    "  name VARCHAR(200) NOT NULL,",
    "  email VARCHAR(255) UNIQUE NOT NULL,",
    "  bio TEXT,",
    "  avatar_url VARCHAR(500),",
    "  created_at TIMESTAMP DEFAULT NOW()",
    ");",
    "",
    "CREATE TABLE posts (",
    "  id SERIAL PRIMARY KEY,",
    "  author_id INT NOT NULL REFERENCES authors(id),",
    "  title VARCHAR(300) NOT NULL,",
    "  slug VARCHAR(300) UNIQUE NOT NULL,",
    "  body TEXT NOT NULL,",
    "  published BOOLEAN NOT NULL DEFAULT false,",
    "  views INT DEFAULT 0,",
    "  created_at TIMESTAMP DEFAULT NOW(),",
    "  updated_at TIMESTAMP DEFAULT NOW()",
    ");",
    "",
    "CREATE TABLE tags (",
    "  id SERIAL PRIMARY KEY,",
    "  name VARCHAR(50) UNIQUE NOT NULL,",
    "  color VARCHAR(7) DEFAULT '#3b82f6'",
    ");",
    "",
    "CREATE TABLE post_tags (",
    "  post_id INT NOT NULL REFERENCES posts(id),",
    "  tag_id INT NOT NULL REFERENCES tags(id),",
    "  PRIMARY KEY (post_id, tag_id)",
    ");",
    "",
    "CREATE TABLE comments (",
    "  id SERIAL PRIMARY KEY,",
    "  post_id INT NOT NULL REFERENCES posts(id),",
    "  author_name VARCHAR(100) NOT NULL,",
    "  author_email VARCHAR(255),",
    "  body TEXT NOT NULL,",
    "  approved BOOLEAN NOT NULL DEFAULT false,",
    "  created_at TIMESTAMP DEFAULT NOW()",
    ");"
  ].join('\n'),
  saas: [
    "CREATE TABLE plans (",
    "  id SERIAL PRIMARY KEY,",
    "  name VARCHAR(100) NOT NULL,",
    "  code VARCHAR(50) UNIQUE NOT NULL,",
    "  price_cents INT NOT NULL CHECK (price_cents >= 0),",
    "  max_users INT NOT NULL,",
    "  features JSONB,",
    "  active BOOLEAN NOT NULL DEFAULT true",
    ");",
    "",
    "CREATE TABLE organizations (",
    "  id SERIAL PRIMARY KEY,",
    "  name VARCHAR(200) NOT NULL,",
    "  slug VARCHAR(200) UNIQUE NOT NULL,",
    "  plan_id INT NOT NULL REFERENCES plans(id),",
    "  created_at TIMESTAMP DEFAULT NOW(),",
    "  trial_ends_at TIMESTAMP",
    ");",
    "",
    "CREATE TABLE users (",
    "  id SERIAL PRIMARY KEY,",
    "  organization_id INT NOT NULL REFERENCES organizations(id),",
    "  email VARCHAR(255) UNIQUE NOT NULL,",
    "  name VARCHAR(200) NOT NULL,",
    "  role VARCHAR(20) NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member','viewer')),",
    "  active BOOLEAN NOT NULL DEFAULT true,",
    "  last_login TIMESTAMP,",
    "  created_at TIMESTAMP DEFAULT NOW()",
    ");",
    "",
    "CREATE TABLE subscriptions (",
    "  id SERIAL PRIMARY KEY,",
    "  organization_id INT NOT NULL REFERENCES organizations(id),",
    "  plan_id INT NOT NULL REFERENCES plans(id),",
    "  start_date TIMESTAMP NOT NULL DEFAULT NOW(),",
    "  end_date TIMESTAMP,",
    "  status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','cancelled','expired')),",
    "  auto_renew BOOLEAN NOT NULL DEFAULT true",
    ");",
    "",
    "CREATE TABLE invoices (",
    "  id SERIAL PRIMARY KEY,",
    "  organization_id INT NOT NULL REFERENCES organizations(id),",
    "  amount_cents INT NOT NULL,",
    "  currency VARCHAR(3) NOT NULL DEFAULT 'USD',",
    "  status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','overdue','cancelled')),",
    "  due_date DATE NOT NULL,",
    "  paid_at TIMESTAMP,",
    "  created_at TIMESTAMP DEFAULT NOW()",
    ");"
  ].join('\n')
};

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
  // Update the page title for bookmarks and tab identification
  var titles = { schema: 'Schema', graph: 'Graph', generate: 'Generate', history: 'History' };
  document.title = (titles[name] || '') + ' — SynthGraph';
  if (name === 'history') loadHistory();
  if (name === 'graph') {
    if (parsedModel) renderGraph();
    else document.getElementById('cy').innerHTML = '<div class="loading-block" style="display:flex"><div style="font-size:14px;font-weight:600;color:var(--text2)">No schema loaded yet</div><div style="color:var(--text3);font-size:12px;margin-top:4px">Go to the Schema tab and paste SQL or pick a template to visualize your database structure.</div><button onclick="showPage(\'schema\')" style="margin-top:12px;padding:6px 16px">Go to Schema</button></div>';
  }
}

document.querySelector('nav').addEventListener('keydown', function(e) {
  if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return;
  var tabs = Array.from(this.querySelectorAll('[role="tab"]'));
  var current = tabs.findIndex(function(a) { return a.classList.contains('active'); });
  var next = e.key === 'ArrowRight' ? (current + 1) % tabs.length : (current - 1 + tabs.length) % tabs.length;
  showPage(tabs[next].id.replace('nav-', ''));
  tabs[next].focus();
});

document.addEventListener('keydown', function(e) {
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
  if (e.key === '?') { toggleShortcuts(); return; }
  if (e.key === 'Escape') {
    if (document.getElementById('shortcuts-modal').style.display !== 'none') { toggleShortcuts(); return; }
    if (document.getElementById('detail-panel').classList.contains('open')) { closePanel(); return; }
    return;
  }
  var map = {s:'schema', g:'graph', d:'generate', h:'history'};
  if (map[e.key]) { showPage(map[e.key]); e.preventDefault(); }
});

document.addEventListener('keydown', function(e) {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    var activePage = document.querySelector('.page.active');
    if (!activePage) return;
    if (activePage.id === 'page-schema') { parseSchema(); e.preventDefault(); }
    else if (activePage.id === 'page-generate') { runGeneration(); e.preventDefault(); }
  }
});

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

function loadFile(e) {
  var file = e.target.files[0];
  if (!file) return;
  var reader = new FileReader();
  reader.onload = function(ev) {
    document.getElementById('sql-input').value = ev.target.result;
    clearFieldError('sql-input-group');
    document.getElementById('template-select').value = '';
    hideEmptyState();
    toast('File loaded: ' + file.name, 'info');
  };
  reader.readAsText(file);
}

function loadTemplate(value) {
  if (!value || !TEMPLATES[value]) return;
  document.getElementById('sql-input').value = TEMPLATES[value];
  document.getElementById('template-select').value = value;
  clearFieldError('sql-input-group');
  hideEmptyState();
  toast('Template loaded', 'info');
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

function toggleEmptyState() {
  var el = document.getElementById('schema-empty');
  if (!el) return;
  var sql = document.getElementById('sql-input').value.trim();
  el.style.display = sql ? 'none' : 'flex';
}
function hideEmptyState() {
  var el = document.getElementById('schema-empty');
  if (el) el.style.display = 'none';
}
function toggleShortcuts() {
  var modal = document.getElementById('shortcuts-modal');
  modal.style.display = modal.style.display === 'none' ? 'flex' : 'none';
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
    var uniqueCount = (table.unique || []).length;
    var pkCount = (table.primary_key || []).length;
    var colCount = cols.length;

    // Build lookup: FK column name \u2192 FK details
    var fkMap = {};
    (table.foreign_keys || []).forEach(function(fk) {
      (fk.columns || []).forEach(function(colName) {
        fkMap[colName] = fk;
      });
    });
    var uniqueCols = {};
    (table.unique || []).forEach(function(group) {
      (group || []).forEach(function(c) { uniqueCols[c] = true; });
    });

    var headerBadges = '<span>' + colCount + ' col' + (colCount !== 1 ? 's' : '') + '</span>';
    if (pkCount) headerBadges += '<span class="badge badge-yellow small" style="font-size:10px;padding:0 5px">PK</span>';
    if (fkCount) headerBadges += '<span class="badge badge-blue small" style="font-size:10px;padding:0 5px">' + fkCount + ' FK</span>';
    if (uniqueCount) headerBadges += '<span class="badge badge-purple small" style="font-size:10px;padding:0 5px">' + uniqueCount + ' UQ</span>';
    if (checkCount) headerBadges += '<span class="badge badge-orange small" style="font-size:10px;padding:0 5px">' + checkCount + ' CHK</span>';

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

    var tbody = document.getElementById('schema-body-' + idx);
    cols.forEach(function(col) {
      var constraints = [];

      // Primary key
      if (col.is_primary_key) constraints.push('<span class="col-pk">PK</span>');

      // Foreign key \u2013 show referenced table + columns + actions
      var fk = fkMap[col.name];
      if (fk) {
        var fkRef = escapeHTML(fk.ref_table);
        if (fk.ref_columns && fk.ref_columns.length) {
          fkRef += '(' + fk.ref_columns.map(function(c) { return escapeHTML(c); }).join(', ') + ')';
        }
        var fkActions = '';
        if (fk.on_delete && fk.on_delete !== 'NO ACTION') fkActions += ' DEL:' + escapeHTML(fk.on_delete);
        if (fk.on_update && fk.on_update !== 'NO ACTION') fkActions += ' UPD:' + escapeHTML(fk.on_update);
        constraints.push(
          '<span class="col-fk" title="' + (fkActions ? 'ON' + fkActions : '') + '">FK</span>' +
          '<span style="color:var(--accent);font-size:9px;margin-left:2px;opacity:0.8">\u2192 ' + fkRef + '</span>'
        );
      }

      // UNIQUE
      if (uniqueCols[col.name] && !col.is_primary_key) {
        constraints.push('<span class="col-unique">UNIQUE</span>');
      }

      // Nullability — always show one so no column looks blank
      if (col.nullable === false || col.is_primary_key) {
        constraints.push('<span class="col-not-null">NOT NULL</span>');
      } else if (col.nullable === true) {
        constraints.push('<span class="col-nullable">NULL</span>');
      }

      // Type with length/precision
      var typeDisplay = escapeHTML(col.type);
      if (col.length > 0) typeDisplay += '(' + col.length + (col.precision > 0 ? ',' + col.precision : '') + ')';

      var semTag = '';
      if (col.semantic) semTag = '<span class="col-semantic">' + escapeHTML(col.semantic) + '</span>';

      var defaultVal = col.default || '';
      if (defaultVal) defaultVal = '<span class="col-default">' + escapeHTML(defaultVal) + '</span>';

      var tr = document.createElement('tr');
      tr.innerHTML = '<td style="font-weight:500">' + escapeHTML(col.name) + semTag + '</td>' +
        '<td style="color:var(--text2);font-family:monospace;font-size:12px">' + typeDisplay + '</td>' +
        '<td style="line-height:1.7">' + constraints.join(' ') + '</td>' +
        '<td>' + defaultVal + '</td>';
      tbody.appendChild(tr);
    });

    // Foreign Key details section
    var foreignKeys = table.foreign_keys || [];
    if (foreignKeys.length > 0) {
      var fkSectionRow = document.createElement('tr');
      var fkHtml = '<td colspan="4" style="padding:8px 14px;background:var(--bg3)">';
      fkHtml += '<div style="font-size:11px;color:var(--accent);margin-bottom:6px"><span>\u2192</span> Foreign Keys</div>';
      foreignKeys.forEach(function(fk) {
        var colList = (fk.columns || []).map(function(c) { return escapeHTML(c); }).join(', ');
        var refColList = (fk.ref_columns || []).map(function(c) { return escapeHTML(c); }).join(', ');
        fkHtml += '<div style="font-size:11px;color:var(--text2);font-family:monospace;padding:2px 0;display:flex;gap:8px;align-items:baseline">';
        fkHtml += '<span style="color:var(--accent);min-width:120px">' + colList + '</span>';
        fkHtml += '<span style="color:var(--text3)">\u2192</span>';
        fkHtml += '<span style="color:var(--text)">' + escapeHTML(fk.ref_table) + '(' + refColList + ')</span>';
        var actions = [];
        if (fk.on_delete && fk.on_delete !== 'NO ACTION') actions.push('ON DELETE ' + escapeHTML(fk.on_delete));
        if (fk.on_update && fk.on_update !== 'NO ACTION') actions.push('ON UPDATE ' + escapeHTML(fk.on_update));
        if (actions.length) {
          fkHtml += '<span style="color:var(--orange);font-size:10px;margin-left:auto">' + actions.join(' ') + '</span>';
        }
        fkHtml += '</div>';
      });
      fkHtml += '</td>';
      fkSectionRow.innerHTML = fkHtml;
      tbody.appendChild(fkSectionRow);
    }

    // Unique constraints section
    var uniqueGroups = table.unique || [];
    if (uniqueGroups.length > 0) {
      var uqSectionRow = document.createElement('tr');
      var uqHtml = '<td colspan="4" style="padding:8px 14px;background:var(--bg3);border-top:1px solid var(--border)">';
      uqHtml += '<div style="font-size:11px;color:#c084fc;margin-bottom:4px"><span>\u2630</span> Unique Constraints</div>';
      uniqueGroups.forEach(function(group) {
        var colList = (group || []).map(function(c) { return escapeHTML(c); }).join(', ');
        uqHtml += '<div style="font-size:11px;color:var(--text2);font-family:monospace;padding:1px 6px">(' + colList + ')</div>';
      });
      uqHtml += '</td>';
      uqSectionRow.innerHTML = uqHtml;
      tbody.appendChild(uqSectionRow);
    }

    // CHECK constraints section
    var checks = table.checks || [];
    if (checks.length > 0) {
      var checkRow = document.createElement('tr');
      var checkHtml = '<td colspan="4" style="padding:8px 14px;background:var(--bg3);border-top:1px solid var(--border)">';
      checkHtml += '<div style="font-size:11px;color:var(--orange);margin-bottom:4px"><span>\u2691</span> CHECK Constraints</div>';
      checks.forEach(function(chk) {
        var label = chk.name ? escapeHTML(chk.name) + ': ' : '';
        checkHtml += '<div style="font-size:11px;color:var(--text2);font-family:monospace;padding:1px 6px">' +
          label + escapeHTML(chk.expression) + '</div>';
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
      if (roles.indexOf('hierarchical') !== -1) return '#3b82f6';
      if (roles.indexOf('transactional') !== -1) return '#22c55e';
      if (roles.indexOf('lookup') !== -1) return '#eab308';
      return '#64748b';
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
          isJunction: n.is_junction,
          color: nodeColor(roles)
        }
      });
    });
    gData.edges.forEach(function(e) {
      elements.push({
        data: {id: e.id, source: e.source, target: e.target, label: e.label, nullable: e.nullable}
      });
    });
    window.cyInstance = cytoscape({
      container: document.getElementById('cy'),
      elements: elements,
      style: [
        { selector: 'node', style: {
          'label': 'data(label)', 'font-size': '11px',
          'text-valign': 'bottom', 'text-halign': 'center',
          'color': '#e2e8f0', 'text-wrap': 'wrap', 'text-max-width': '80px',
          'background-color': 'data(color)', 'width': 60, 'height': 60,
          'border-width': 2, 'border-color': '#263040',
          'transition-property': 'background-color, border-color, opacity',
          'transition-duration': '0.2s'
        }},
        { selector: 'node:selected', style: {
          'border-color': '#fff', 'border-width': 3
        }},
        { selector: 'edge', style: {
          'width': 2, 'line-color': '#3b82f6',
          'target-arrow-color': '#3b82f6', 'target-arrow-shape': 'triangle',
          'curve-style': 'bezier', 'label': 'data(label)', 'font-size': '9px',
          'color': '#94a3b8', 'text-rotation': 'autorotate',
          'line-style': 'solid'
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

function filterGraph(query) {
  if (!window.cyInstance) return;
  var q = query.trim().toLowerCase();
  window.cyInstance.nodes().forEach(function(n) {
    var match = !q || n.data('label').toLowerCase().indexOf(q) !== -1;
    n.style('opacity', match ? 1 : 0.15);
    n.style('border-color', match ? '#263040' : '#18222e');
  });
  window.cyInstance.edges().forEach(function(e) {
    var src = e.source();
    var tgt = e.target();
    var match = src.style('opacity') == 1 && tgt.style('opacity') == 1;
    e.style('opacity', match ? 1 : 0.08);
  });
}

function exportGraph() {
  if (!window.cyInstance) { toast('No graph to export', 'error'); return; }
  var png = window.cyInstance.png({full: true, scale: 2, bg: '#0a0e14'});
  var a = document.createElement('a');
  a.href = png;
  a.download = 'synthgraph-schema.png';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  toast('Graph exported as PNG', 'success');
}

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

  var doneEl = document.getElementById('gen-done');
  if (!doneEl) {
    doneEl = document.createElement('div');
    doneEl.id = 'gen-done';
    doneEl.className = 'gen-done';
    var container = resultEl.querySelector('.card-header');
    if (container) container.parentNode.insertBefore(doneEl, container);
  }
  doneEl.style.display = 'flex';
  var hasWarnings = data.errors && data.errors.length;
  doneEl.className = 'gen-done' + (hasWarnings ? ' gen-done-warn' : '');
  doneEl.innerHTML = '<div class="gen-done-icon">' + (hasWarnings ? '&#9888;' : '&#10003;') + '</div>' +
    '<div class="gen-done-text"><div class="gen-done-title">' + (hasWarnings ? 'Generated with warnings' : 'Generation complete') + '</div>' +
    '<div class="gen-done-desc">' + data.tables + ' table(s) \u00B7 ' + (data.format || '').toUpperCase() + (hasWarnings ? ' \u00B7 ' + data.errors.length + ' warning(s)' : '') + '</div></div>' +
    '<div class="gen-done-actions">' +
    '<button onclick="downloadResult()" class="primary small">Download</button>' +
    '<button onclick="showPage(\'history\')" class="small">View in History</button>' +
    '<button onclick="resetGeneration()" class="small ghost">Generate More</button></div>';

  var metaEl = document.getElementById('result-meta');
  metaEl.innerHTML = '<span class="badge badge-blue">Job #' + data.job_id + '</span><span>' + data.tables + ' tables</span><span>' + (data.format || '').toUpperCase() + '</span>';
  if (data.errors && data.errors.length) {
    metaEl.innerHTML += '<span style="color:var(--yellow)">' + data.errors.length + ' warning(s)</span>';
  }

  var warningsHtml = '';
  if (data.errors && data.errors.length) {
    warningsHtml = '<div style="margin:8px 0;padding:8px 12px;background:var(--yellow-bg);border-radius:var(--radius-sm);border:1px solid rgba(234,179,8,0.2)">';
    data.errors.forEach(function(e) { warningsHtml += '<div style="font-size:12px;color:var(--yellow)"><span>\u26A0</span> ' + escapeHTML(e) + '</div>'; });
    warningsHtml += '</div>';
  }

  var isSQL = (data.format === 'sql');
  var previewHtml = warningsHtml;
  var lines = data.data.split('\n').filter(function(l) { return l.trim(); });

  if (isSQL) {
    // SQL output: show the first N INSERT statements as code blocks
    var sqlPreviewLines = lines.slice(0, 10);
    previewHtml += '<div class="data-preview"><pre style="padding:12px;margin:0;font-size:11px;line-height:1.6;color:var(--text);white-space:pre-wrap;word-break:break-word">';
    sqlPreviewLines.forEach(function(l) { previewHtml += escapeHTML(l) + '\n'; });
    if (lines.length > 10) previewHtml += '<span style="color:var(--text3)">... and ' + (lines.length - 10) + ' more lines</span>\n';
    previewHtml += '</pre></div>';
  } else {
    // CSV output: parse and show as a preview table
    var previewLines = lines.slice(0, 21);
    var csvLines = previewLines.map(function(l) { return parseCSVLine(l); });
    if (csvLines.length > 0) {
      previewHtml += '<div class="data-preview"><table><thead><tr>';
      (csvLines[0] || []).forEach(function(h) { previewHtml += '<th>' + escapeHTML(h) + '</th>'; });
      previewHtml += '</tr></thead><tbody>';
      for (var i = 1; i < csvLines.length; i++) {
        previewHtml += '<tr>';
        (csvLines[i] || []).forEach(function(c) { previewHtml += '<td onclick="copyCell(this)" title="Click to copy">' + escapeHTML(c) + '</td>'; });
        previewHtml += '</tr>';
      }
      previewHtml += '</tbody></table></div>';
    }
  }
  document.getElementById('result-preview').innerHTML = previewHtml;

  document.getElementById('result-raw-text').value = data.data;

  if (isSQL) {
    // SQL format: default to Raw Data tab since CSV-style preview doesn't apply
    resultTab = 'raw';
    document.querySelectorAll('#gen-result .tab').forEach(function(t) { t.classList.remove('active'); t.setAttribute('aria-selected', 'false'); });
    document.querySelectorAll('#gen-result .tab')[1].classList.add('active');
    document.querySelectorAll('#gen-result .tab')[1].setAttribute('aria-selected', 'true');
    document.getElementById('result-preview').style.display = 'none';
    document.getElementById('result-raw').style.display = 'block';
    document.getElementById('result-profiling').style.display = 'none';
  } else {
    resultTab = 'preview';
    document.querySelectorAll('#gen-result .tab').forEach(function(t) { t.classList.remove('active'); t.setAttribute('aria-selected', 'false'); });
    document.querySelector('#gen-result .tab').classList.add('active');
    document.querySelector('#gen-result .tab').setAttribute('aria-selected', 'true');
    document.getElementById('result-preview').style.display = 'block';
    document.getElementById('result-raw').style.display = 'none';
    showProfiling(data.data, (lines.slice(0, 1).length ? lines.slice(0, 1).map(function(l) { return parseCSVLine(l); })[0] : []));
  }
}

function showProfiling(rawData, headers) {
  var container = document.getElementById('result-profiling');
  var lines = rawData.split('\n').filter(function(l) { return l.trim(); });
  if (lines.length < 2) { container.style.display = 'none'; return; }

  var dataLines = lines.slice(1);
  var colCount = headers.length;
  if (colCount === 0) { container.style.display = 'none'; return; }

  var cols = [];
  for (var i = 0; i < colCount; i++) {
    var vals = [];
    var nulls = 0;
    for (var j = 0; j < dataLines.length; j++) {
      var row = parseCSVLine(dataLines[j]);
      if (i < row.length) {
        var v = row[i].trim();
        if (v === '' || v === 'NULL' || v === 'null') { nulls++; continue; }
        vals.push(v);
      } else {
        nulls++;
      }
    }
    var type = 'text';
    var allNumbers = vals.length > 0 && vals.every(function(v) { return !isNaN(parseFloat(v)) && isFinite(v); });
    var allInts = allNumbers && vals.every(function(v) { return v.indexOf('.') === -1; });
    if (allInts) type = 'integer';
    else if (allNumbers) type = 'decimal';
    var samples = vals.slice(0, 3);
    cols.push({name: headers[i], type: type, nulls: nulls, total: dataLines.length, samples: samples, distinct: new Set(vals).size});
  }

  var html = '<div class="card-header" style="margin-bottom:8px"><h2>Data Profile <span class="text-3 text-sm">' + dataLines.length + ' rows</span></h2></div>';
  html += '<div class="profiling-grid">';
  cols.forEach(function(c) {
    var typeBadge = c.type === 'integer' ? 'badge-blue' : c.type === 'decimal' ? 'badge-purple' : 'badge-gray';
    html += '<div class="profiling-card">' +
      '<div class="prof-col-name">' + escapeHTML(c.name) + '</div>' +
      '<div><span class="badge ' + typeBadge + '" style="font-size:9px;padding:0 5px">' + c.type + '</span></div>' +
      '<div class="prof-stat"><span>Nulls</span><span class="prof-val">' + c.nulls + '/' + c.total + '</span></div>' +
      '<div class="prof-stat"><span>Distinct</span><span class="prof-val">' + c.distinct + '</span></div>';
    if (c.samples.length) {
      html += '<div class="prof-stat" style="flex-wrap:wrap"><span>Samples</span><span class="prof-val" style="font-size:10px">' + c.samples.map(function(s) { return escapeHTML(s.length > 20 ? s.slice(0,20) + '...' : s); }).join(', ') + '</span></div>';
    }
    html += '</div>';
  });
  html += '</div>';
  container.innerHTML = html;
  container.style.display = 'block';
}

function copyCell(el) {
  var text = el.textContent;
  if (!text) return;
  navigator.clipboard.writeText(text).then(function() {
    var orig = el.style.background;
    el.style.background = 'rgba(59,130,246,0.15)';
    setTimeout(function() { el.style.background = orig; }, 300);
  }).catch(function() {});
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

function resetGeneration() {
  document.getElementById('gen-result').style.display = 'none';
  document.getElementById('gen-progress').style.display = 'none';
  document.getElementById('gen-pipeline').style.display = 'none';
  var doneEl = document.getElementById('gen-done');
  if (doneEl) doneEl.style.display = 'none';
  document.getElementById('generate-btn').disabled = false;
  document.getElementById('generate-btn').textContent = 'Generate';
}
function closePanel() {
  document.getElementById('detail-panel').classList.remove('open');
}

(function() {
  var activePage = document.querySelector('.page.active');
  if (activePage && activePage.id === 'page-history') loadHistory();
})();

document.getElementById('sql-input').addEventListener('input', toggleEmptyState);
