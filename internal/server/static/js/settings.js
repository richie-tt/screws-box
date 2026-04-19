(function() {
  'use strict';

  // --- CSRF Token ---

  function getCSRFToken() {
    var match = document.cookie.match(/(?:^|; )screwsbox_csrf=([^;]*)/);
    return match ? match[1] : '';
  }

  // --- Password Validation ---

  function validatePassword(pw) {
    if (pw.length < 12) return 'Password must be at least 12 characters';
    if (!/[A-Z]/.test(pw)) return 'Password must contain an uppercase letter';
    if (!/[a-z]/.test(pw)) return 'Password must contain a lowercase letter';
    if (!/[0-9]/.test(pw)) return 'Password must contain a digit';
    if (!/[^A-Za-z0-9]/.test(pw)) return 'Password must contain a special character';
    return '';
  }

  // --- Feedback Helpers ---

  function showFeedback(el, message, type) {
    el.textContent = message;
    el.className = 'settings-form-feedback ' + type;
    if (type === 'success') {
      setTimeout(function() {
        el.textContent = '';
        el.className = 'settings-form-feedback';
      }, 2000);
    }
  }

  function clearFeedback(el) {
    el.textContent = '';
    el.className = 'settings-form-feedback';
  }

  function setBusy(btn, busy) {
    if (busy) {
      btn.setAttribute('aria-busy', 'true');
      btn.style.opacity = '0.7';
      btn.style.pointerEvents = 'none';
    } else {
      btn.removeAttribute('aria-busy');
      btn.style.opacity = '';
      btn.style.pointerEvents = '';
    }
  }

  // --- Shelf Name Form ---

  var shelfNameForm = document.getElementById('shelf-name-form');
  if (shelfNameForm) {
    var nameInput = shelfNameForm.querySelector('#shelf-name-input');
    var nameBtn = shelfNameForm.querySelector('button[type="submit"]');
    var nameFeedback = shelfNameForm.querySelector('.settings-form-feedback');
    var resizeRowsInput = document.getElementById('resize-rows');
    var resizeColsInput = document.getElementById('resize-cols');

    shelfNameForm.addEventListener('submit', function(e) {
      e.preventDefault();
      clearFeedback(nameFeedback);
      setBusy(nameBtn, true);

      var currentRows = parseInt(resizeRowsInput.value, 10);
      var currentCols = parseInt(resizeColsInput.value, 10);

      fetch('/api/shelf/resize', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken()
        },
        body: JSON.stringify({
          rows: currentRows,
          cols: currentCols,
          name: nameInput.value
        })
      })
      .then(function(resp) {
        return resp.json().then(function(data) {
          return { ok: resp.ok, status: resp.status, data: data };
        });
      })
      .then(function(result) {
        setBusy(nameBtn, false);
        if (result.ok) {
          showFeedback(nameFeedback, 'Saved', 'success');
        } else {
          showFeedback(nameFeedback, (result.data && result.data.error) || 'Failed to save settings. Check your connection and try again.', 'error');
        }
      })
      .catch(function() {
        setBusy(nameBtn, false);
        showFeedback(nameFeedback, 'Failed to save settings. Check your connection and try again.', 'error');
      });
    });
  }

  // --- Grid Resize Form ---

  var resizeForm = document.getElementById('resize-form');
  if (resizeForm) {
    var rowsInput = resizeForm.querySelector('#resize-rows');
    var colsInput = resizeForm.querySelector('#resize-cols');
    var resizeBtn = resizeForm.querySelector('button[type="submit"]');
    var resizeFeedback = resizeForm.querySelector('.settings-form-feedback');
    var currentText = document.getElementById('resize-current');

    var previousRows = parseInt(rowsInput.value, 10);
    var previousCols = parseInt(colsInput.value, 10);

    // Pending resize data for force confirmation
    var pendingResize = null;

    resizeForm.addEventListener('submit', function(e) {
      e.preventDefault();
      clearFeedback(resizeFeedback);
      setBusy(resizeBtn, true);

      var newRows = parseInt(rowsInput.value, 10);
      var newCols = parseInt(colsInput.value, 10);

      submitResize(newRows, newCols, false);
    });

    function submitResize(rows, cols, force) {
      var payload = { rows: rows, cols: cols };
      if (force) {
        payload.force = true;
      }

      fetch('/api/shelf/resize', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken()
        },
        body: JSON.stringify(payload)
      })
      .then(function(resp) {
        return resp.json().then(function(data) {
          return { ok: resp.ok, status: resp.status, data: data };
        });
      })
      .then(function(result) {
        setBusy(resizeBtn, false);

        if (result.status === 200) {
          previousRows = rows;
          previousCols = cols;
          if (currentText) {
            currentText.textContent = 'Currently ' + rows + ' x ' + cols;
          }
          showFeedback(resizeFeedback, 'Saved', 'success');
          return;
        }

        if (result.status === 409 && result.data && result.data.affected) {
          pendingResize = { rows: rows, cols: cols };
          showResizeModal(result.data.affected);
          return;
        }

        showFeedback(resizeFeedback, (result.data && result.data.error) || 'Failed to resize grid. Check your connection and try again.', 'error');
      })
      .catch(function() {
        setBusy(resizeBtn, false);
        showFeedback(resizeFeedback, 'Failed to resize grid. Check your connection and try again.', 'error');
      });
    }

    // --- Resize Confirmation Modal ---

    var modalOverlay = document.getElementById('resize-modal-overlay');
    var modalBody = document.getElementById('resize-modal-body');
    var modalConfirm = document.getElementById('resize-modal-confirm');
    var modalCancel = document.getElementById('resize-modal-cancel');

    function showResizeModal(affected) {
      // Build modal content using safe DOM methods
      while (modalBody.firstChild) {
        modalBody.removeChild(modalBody.firstChild);
      }

      var p = document.createElement('p');
      p.textContent = affected.length + ' container' + (affected.length !== 1 ? 's have' : ' has') + ' items that will be lost:';
      modalBody.appendChild(p);

      var ul = document.createElement('ul');
      for (var i = 0; i < affected.length; i++) {
        var c = affected[i];
        var li = document.createElement('li');
        var strong = document.createElement('strong');
        strong.textContent = c.label;
        li.appendChild(strong);
        var detail = ': ' + c.item_count + ' item' + (c.item_count !== 1 ? 's' : '');
        if (c.items && c.items.length > 0) {
          detail += ' (' + c.items.join(', ') + ')';
        }
        li.appendChild(document.createTextNode(detail));
        ul.appendChild(li);
      }
      modalBody.appendChild(ul);

      modalOverlay.classList.add('visible');
      modalConfirm.focus();
    }

    function hideResizeModal() {
      modalOverlay.classList.remove('visible');
      pendingResize = null;
      resizeBtn.focus();
    }

    if (modalConfirm) {
      modalConfirm.addEventListener('click', function() {
        if (!pendingResize) return;
        modalOverlay.classList.remove('visible');
        setBusy(resizeBtn, true);
        submitResize(pendingResize.rows, pendingResize.cols, true);
        pendingResize = null;
      });
    }

    if (modalCancel) {
      modalCancel.addEventListener('click', function() {
        // Restore previous values
        rowsInput.value = previousRows;
        colsInput.value = previousCols;
        hideResizeModal();
      });
    }

    // Close modal on Escape
    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape' && modalOverlay && modalOverlay.classList.contains('visible')) {
        rowsInput.value = previousRows;
        colsInput.value = previousCols;
        hideResizeModal();
      }
    });

    // Focus trap in modal
    if (modalOverlay) {
      modalOverlay.addEventListener('keydown', function(e) {
        if (e.key !== 'Tab') return;
        var focusable = [modalCancel, modalConfirm];
        var first = focusable[0];
        var last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      });
    }
  }

  // --- Auth Settings Form ---

  var authForm = document.getElementById('auth-form');
  if (authForm) {
    var authToggle = document.getElementById('auth-enabled');
    var authFields = authForm.querySelector('.auth-fields');
    var authUsername = document.getElementById('auth-username');
    var authPassword = document.getElementById('auth-password');
    var authBtn = authForm.querySelector('button[type="submit"]');
    var authFeedback = authForm.querySelector('.settings-form-feedback');

    // Toggle auth fields visibility
    if (authToggle) {
      authToggle.addEventListener('change', function() {
        if (authFields) {
          authFields.hidden = !authToggle.checked;
        }
      });
    }

    authForm.addEventListener('submit', function(e) {
      e.preventDefault();
      clearFeedback(authFeedback);

      var enabled = authToggle.checked;
      var username = authUsername ? authUsername.value.trim() : '';
      var password = authPassword ? authPassword.value : '';

      // Client-side password validation
      if (password) {
        var pwErr = validatePassword(password);
        if (pwErr) {
          showFeedback(authFeedback, pwErr, 'error');
          if (authPassword) authPassword.focus();
          return;
        }
      }

      setBusy(authBtn, true);

      fetch('/api/shelf/auth', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken()
        },
        body: JSON.stringify({
          enabled: enabled,
          username: username,
          password: password
        })
      })
      .then(function(resp) {
        return resp.json().then(function(data) {
          return { ok: resp.ok, status: resp.status, data: data };
        });
      })
      .then(function(result) {
        setBusy(authBtn, false);
        if (result.ok) {
          showFeedback(authFeedback, 'Saved', 'success');
          if (authPassword) authPassword.value = '';
          // If auth was just enabled, redirect to login
          if (enabled) {
            window.location.href = '/login';
          }
        } else {
          showFeedback(authFeedback, (result.data && result.data.error) || 'Failed to save settings. Check your connection and try again.', 'error');
        }
      })
      .catch(function() {
        setBusy(authBtn, false);
        showFeedback(authFeedback, 'Failed to save settings. Check your connection and try again.', 'error');
      });
    });
  }

  // --- OIDC Config ---

  var oidcForm = document.getElementById('oidc-form');
  var oidcEnabled = document.getElementById('oidc-enabled');
  var oidcFields = oidcForm ? oidcForm.querySelector('.oidc-fields') : null;
  var oidcSaveBtn = document.getElementById('oidc-save-btn');

  // Toggle OIDC fields visibility
  if (oidcEnabled && oidcFields) {
    oidcEnabled.addEventListener('change', function() {
      if (this.checked) {
        oidcFields.hidden = false;
      } else {
        oidcFields.hidden = true;
      }
      updateOIDCSaveLabel();
    });
  }

  // Track if OIDC was originally enabled (for disable confirmation)
  var oidcWasEnabled = oidcEnabled ? oidcEnabled.checked : false;

  function updateOIDCSaveLabel() {
    if (!oidcSaveBtn || !oidcEnabled) return;
    if (oidcWasEnabled && !oidcEnabled.checked) {
      oidcSaveBtn.textContent = 'Disable OIDC and revoke sessions';
    } else {
      oidcSaveBtn.textContent = 'Save OIDC Settings';
    }
  }

  // Secret toggle
  var secretToggle = document.querySelector('.secret-toggle');
  if (secretToggle) {
    secretToggle.addEventListener('click', function() {
      var input = document.getElementById('oidc-client-secret');
      if (input.type === 'password') {
        input.type = 'text';
        this.setAttribute('aria-label', 'Hide secret');
      } else {
        input.type = 'password';
        this.setAttribute('aria-label', 'Show secret');
      }
    });
  }

  // OIDC form submit
  if (oidcForm) {
    oidcForm.addEventListener('submit', function(e) {
      e.preventDefault();
      var feedback = oidcForm.querySelector('.settings-form-feedback');
      var btn = oidcSaveBtn;
      setBusy(btn, true);
      clearFeedback(feedback);

      var payload = {
        enabled: oidcEnabled.checked,
        display_name: document.getElementById('oidc-display-name').value,
        issuer_url: document.getElementById('oidc-issuer').value,
        client_id: document.getElementById('oidc-client-id').value,
        client_secret: document.getElementById('oidc-client-secret').value
      };

      fetch('/api/oidc/config', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken()
        },
        body: JSON.stringify(payload)
      })
      .then(function(res) { return res.json().then(function(data) { return { ok: res.ok, data: data }; }); })
      .then(function(result) {
        setBusy(btn, false);
        if (result.ok) {
          if (!payload.enabled && oidcWasEnabled) {
            showFeedback(feedback, 'OIDC disabled. Active SSO sessions have been revoked.', 'success');
          } else {
            showFeedback(feedback, 'OIDC settings saved.', 'success');
          }
          oidcWasEnabled = payload.enabled;
          // Clear secret field after successful save
          document.getElementById('oidc-client-secret').value = '';
          if (payload.client_secret) {
            document.getElementById('oidc-client-secret').placeholder = '********';
            var helper = oidcForm.querySelector('.secret-field + .text-muted');
            if (!helper) {
              helper = document.createElement('small');
              helper.className = 'text-muted';
              var secretField = oidcForm.querySelector('.secret-field');
              secretField.parentNode.insertBefore(helper, secretField.nextSibling);
            }
            helper.textContent = 'Secret is configured. Enter a new value to replace it.';
          }
          updateOIDCSaveLabel();
        } else {
          showFeedback(feedback, result.data.error || 'Failed to save OIDC settings.', 'error');
        }
      })
      .catch(function() {
        setBusy(btn, false);
        showFeedback(feedback, 'Network error. Please try again.', 'error');
      });
    });
  }

  // --- Sidebar Navigation ---

  var navItems = document.querySelectorAll('.settings-nav-item:not(.disabled)');
  navItems.forEach(function(item) {
    item.addEventListener('click', function(e) {
      var href = item.getAttribute('href');
      if (href && href.startsWith('#')) {
        e.preventDefault();
        var target = document.querySelector(href);
        if (target) {
          target.scrollIntoView({ behavior: 'smooth' });
        }
        // Update active state
        navItems.forEach(function(n) { n.classList.remove('active'); });
        item.classList.add('active');
      }
    });
  });

  // IntersectionObserver for scroll-based active state
  var sections = document.querySelectorAll('.settings-card[id]');
  if (sections.length > 0 && 'IntersectionObserver' in window) {
    var observer = new IntersectionObserver(function(entries) {
      entries.forEach(function(entry) {
        if (entry.isIntersecting) {
          var id = entry.target.id;
          navItems.forEach(function(n) {
            if (n.getAttribute('href') === '#' + id) {
              n.classList.add('active');
            } else {
              n.classList.remove('active');
            }
          });
          if (id === 'housekeeping') {
            loadDuplicates();
          }
        }
      });
    }, { threshold: 0.3 });

    sections.forEach(function(section) {
      observer.observe(section);
    });
  }

  // --- Data Export/Import Section ---

  var exportBtn = document.getElementById('export-btn');
  var exportFeedback = document.getElementById('export-feedback');
  var importFile = document.getElementById('import-file');
  var validateBtn = document.getElementById('validate-btn');
  var importFeedback = document.getElementById('import-feedback');
  var importUploadArea = document.getElementById('import-upload-area');
  var importSummaryArea = document.getElementById('import-summary-area');
  var importErrorArea = document.getElementById('import-error-area');
  var importBackBtn = document.getElementById('import-back-btn');
  var importConfirmBtn = document.getElementById('import-confirm-btn');
  var importRetryBtn = document.getElementById('import-retry-btn');
  var currentImportToken = null;

  if (exportBtn) {
    exportBtn.addEventListener('click', function() {
      exportBtn.textContent = 'Exporting...';
      exportBtn.disabled = true;
      exportFeedback.textContent = '';
      exportFeedback.className = 'settings-form-feedback';
      // Trigger browser download via navigation per D-17
      window.location.href = '/api/export';
      // Reset button after short delay (download starts async)
      setTimeout(function() {
        exportBtn.textContent = 'Export JSON';
        exportBtn.disabled = false;
      }, 2000);
    });
  }

  if (importFile) {
    importFile.addEventListener('change', function() {
      if (importFile.files.length > 0) {
        validateBtn.disabled = false;
        validateBtn.className = 'btn';
      } else {
        validateBtn.disabled = true;
        validateBtn.className = 'btn secondary';
      }
    });
  }

  function showImportUpload() {
    importUploadArea.hidden = false;
    importSummaryArea.hidden = true;
    importErrorArea.hidden = true;
    importFeedback.textContent = '';
    importFeedback.className = 'settings-form-feedback';
    importFile.value = '';
    validateBtn.disabled = true;
    validateBtn.className = 'btn secondary';
    currentImportToken = null;
  }

  if (validateBtn) {
    validateBtn.addEventListener('click', async function() {
      if (!importFile.files.length) return;
      validateBtn.textContent = 'Validating...';
      validateBtn.disabled = true;
      importFeedback.textContent = '';

      var formData = new FormData();
      formData.append('file', importFile.files[0]);

      try {
        var resp = await fetch('/api/import/validate', {
          method: 'POST',
          headers: { 'X-CSRF-Token': getCSRFToken() },
          body: formData
        });
        var result = await resp.json();

        if (resp.ok) {
          // Show summary per D-09
          currentImportToken = result.token;
          document.getElementById('summary-shelf').textContent =
            '"' + result.summary.shelf_name + '" (' + result.summary.rows + ' x ' + result.summary.cols + ')';
          document.getElementById('summary-containers').textContent = result.summary.containers;
          document.getElementById('summary-items').textContent = result.summary.items;
          document.getElementById('summary-tags').textContent = result.summary.tags;
          importUploadArea.hidden = true;
          importSummaryArea.hidden = false;
          importErrorArea.hidden = true;
        } else {
          // Show validation errors using safe DOM methods (no innerHTML)
          var errorsList = document.getElementById('validation-errors-list');
          while (errorsList.firstChild) {
            errorsList.removeChild(errorsList.firstChild);
          }
          (result.errors || []).forEach(function(errMsg) {
            var li = document.createElement('li');
            li.textContent = errMsg;
            errorsList.appendChild(li);
          });
          importUploadArea.hidden = true;
          importSummaryArea.hidden = true;
          importErrorArea.hidden = false;
        }
      } catch (e) {
        importFeedback.textContent = 'Validation request failed.';
        importFeedback.className = 'settings-form-feedback error';
      }

      validateBtn.textContent = 'Validate File';
      validateBtn.disabled = false;
    });
  }

  if (importBackBtn) {
    importBackBtn.addEventListener('click', showImportUpload);
  }

  if (importRetryBtn) {
    importRetryBtn.addEventListener('click', showImportUpload);
  }

  if (importConfirmBtn) {
    importConfirmBtn.addEventListener('click', async function() {
      if (!currentImportToken) return;
      importConfirmBtn.textContent = 'Importing...';
      importConfirmBtn.disabled = true;

      try {
        var resp = await fetch('/api/import/confirm', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCSRFToken()
          },
          body: JSON.stringify({ token: currentImportToken })
        });
        var result = await resp.json();

        if (resp.ok) {
          // Show success and reload per UI spec
          importSummaryArea.hidden = true;
          importUploadArea.hidden = false;
          importFeedback.textContent = result.message || 'Import complete. Data restored successfully.';
          importFeedback.className = 'settings-form-feedback success';
          currentImportToken = null;
          setTimeout(function() { location.reload(); }, 2000);
        } else {
          importConfirmBtn.textContent = 'Replace All Data';
          importConfirmBtn.disabled = false;
          importFeedback.textContent = result.error || 'Import failed. Existing data was not modified.';
          importFeedback.className = 'settings-form-feedback error';
          importSummaryArea.hidden = true;
          importUploadArea.hidden = false;
        }
      } catch (e) {
        importConfirmBtn.textContent = 'Replace All Data';
        importConfirmBtn.disabled = false;
        importFeedback.textContent = 'Import request failed.';
        importFeedback.className = 'settings-form-feedback error';
        importSummaryArea.hidden = true;
        importUploadArea.hidden = false;
      }
    });
  }

  // --- Sessions Section ---

  var sessionsRefresh = document.getElementById('sessions-refresh');
  var sessionsTbody = document.getElementById('sessions-tbody');
  var sessionsFeedback = document.getElementById('sessions-feedback');
  var revokeAllBtn = document.getElementById('revoke-all-btn');
  var revokeOverlay = document.getElementById('revoke-modal-overlay');
  var revokeTitle = document.getElementById('revoke-modal-title');
  var revokeMessage = document.getElementById('revoke-modal-message');
  var revokeConfirmBtn = document.getElementById('revoke-modal-confirm');
  var revokeCancelBtn = document.getElementById('revoke-modal-cancel');
  var pendingRevoke = null;

  function refreshSessions() {
    if (!sessionsRefresh) return;
    setBusy(sessionsRefresh, true);
    fetch('/api/sessions', {
      headers: { 'X-CSRF-Token': getCSRFToken() }
    })
    .then(function(resp) { return resp.json(); })
    .then(function(sessions) {
      setBusy(sessionsRefresh, false);
      renderSessionsTable(sessions);
      updateSessionBadge(sessions.length);
    })
    .catch(function() {
      setBusy(sessionsRefresh, false);
      if (sessionsFeedback) {
        showFeedback(sessionsFeedback, 'Failed to load sessions. Check server logs and try again.', 'error');
      }
    });
  }

  function renderSessionsTable(sessions) {
    if (!sessionsTbody) return;
    var wrap = document.getElementById('sessions-table-wrap');
    var empty = document.getElementById('sessions-empty');
    var bulk = document.getElementById('sessions-bulk-actions');

    if (sessions.length === 0) {
      if (wrap) wrap.hidden = true;
      if (bulk) bulk.hidden = true;
      if (empty) {
        empty.hidden = false;
      } else {
        var div = document.createElement('div');
        div.className = 'sessions-empty';
        div.id = 'sessions-empty';
        var heading = document.createElement('p');
        heading.textContent = 'No active sessions';
        div.appendChild(heading);
        var storeLabel = document.createElement('p');
        storeLabel.className = 'text-muted';
        var indicator = document.querySelector('.store-indicator');
        storeLabel.textContent = 'Session store: ' + (indicator ? indicator.textContent : 'Unknown');
        div.appendChild(storeLabel);
        var sessionsCard = document.getElementById('sessions');
        if (sessionsCard) sessionsCard.appendChild(div);
      }
      return;
    }

    if (wrap) wrap.hidden = false;
    if (empty) empty.hidden = true;
    if (bulk) {
      var hasOthers = sessions.some(function(s) { return !s.is_current; });
      bulk.hidden = !hasOthers;
    }

    while (sessionsTbody.firstChild) {
      sessionsTbody.removeChild(sessionsTbody.firstChild);
    }

    sessions.forEach(function(s) {
      var tr = document.createElement('tr');
      tr.setAttribute('data-session-id', s.id);
      if (s.is_current) {
        tr.setAttribute('aria-label', 'Your current session');
      }

      var tdUser = document.createElement('td');
      tdUser.textContent = s.username + (s.display_name ? ' (' + s.display_name + ')' : '');
      tr.appendChild(tdUser);

      var tdMethod = document.createElement('td');
      tdMethod.textContent = s.auth_method;
      tr.appendChild(tdMethod);

      var tdCreated = document.createElement('td');
      tdCreated.className = 'sessions-timestamp';
      tdCreated.textContent = s.created_at;
      tr.appendChild(tdCreated);

      var tdActive = document.createElement('td');
      tdActive.className = 'sessions-timestamp';
      tdActive.textContent = s.last_activity;
      tr.appendChild(tdActive);

      var tdExpires = document.createElement('td');
      tdExpires.className = 'sessions-col-expires';
      tdExpires.textContent = s.expires_in;
      tr.appendChild(tdExpires);

      var tdActions = document.createElement('td');
      tdActions.className = 'sessions-col-actions';
      if (s.is_current) {
        var badge = document.createElement('span');
        badge.className = 'session-badge-own';
        badge.textContent = 'Your session';
        tdActions.appendChild(badge);
      } else {
        var btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'ghost danger-ghost session-revoke-btn';
        btn.setAttribute('data-session-id', s.id);
        btn.setAttribute('data-username', s.username);
        btn.setAttribute('aria-label', 'Revoke session for ' + s.username);
        btn.textContent = 'Revoke';
        tdActions.appendChild(btn);
      }
      tr.appendChild(tdActions);

      sessionsTbody.appendChild(tr);
    });
  }

  function updateSessionBadge(count) {
    var badge = document.querySelector('.nav-badge');
    if (badge) {
      badge.textContent = count;
    }
  }

  var sessionsCard = document.getElementById('sessions');
  if (sessionsCard) {
    sessionsCard.addEventListener('click', function(e) {
      var btn = e.target.closest('.session-revoke-btn');
      if (!btn) return;
      var sessionId = btn.getAttribute('data-session-id');
      var username = btn.getAttribute('data-username');
      showRevokeModal('single', sessionId, username);
    });
  }

  if (revokeAllBtn) {
    revokeAllBtn.addEventListener('click', function() {
      var rows = sessionsTbody ? sessionsTbody.querySelectorAll('tr') : [];
      var otherCount = 0;
      for (var i = 0; i < rows.length; i++) {
        if (!rows[i].getAttribute('aria-label')) otherCount++;
      }
      showRevokeModal('bulk', null, null, otherCount);
    });
  }

  function showRevokeModal(type, sessionId, username, count) {
    if (!revokeOverlay) return;
    pendingRevoke = { type: type, sessionId: sessionId };

    if (type === 'single') {
      revokeTitle.textContent = 'Revoke Session';
      revokeMessage.textContent = 'Revoke session for ' + username + '? They will be forced to log in again.';
      revokeConfirmBtn.textContent = 'Revoke Session';
      revokeCancelBtn.textContent = 'Keep Session';
    } else {
      revokeTitle.textContent = 'Revoke All Other Sessions';
      revokeMessage.textContent = 'Revoke ' + count + ' session' + (count !== 1 ? 's' : '') + '? All other users will be forced to log in again.';
      revokeConfirmBtn.textContent = 'Revoke All';
      revokeCancelBtn.textContent = 'Keep Sessions';
    }

    revokeOverlay.classList.add('visible');
    revokeConfirmBtn.focus();
  }

  function hideRevokeModal() {
    if (revokeOverlay) revokeOverlay.classList.remove('visible');
    pendingRevoke = null;
  }

  if (revokeConfirmBtn) {
    revokeConfirmBtn.addEventListener('click', function() {
      if (!pendingRevoke) return;

      if (pendingRevoke.type === 'single') {
        var sid = pendingRevoke.sessionId;
        hideRevokeModal();
        fetch('/api/sessions/' + sid, {
          method: 'DELETE',
          headers: { 'X-CSRF-Token': getCSRFToken() }
        })
        .then(function(resp) {
          if (resp.ok) {
            var row = sessionsTbody.querySelector('tr[data-session-id="' + sid + '"]');
            if (row) {
              row.classList.add('session-row-removing');
              setTimeout(function() {
                row.parentNode.removeChild(row);
                var remaining = sessionsTbody.querySelectorAll('tr').length;
                updateSessionBadge(remaining);
                var hasOthers = sessionsTbody.querySelector('.session-revoke-btn');
                var bulkEl = document.getElementById('sessions-bulk-actions');
                if (!hasOthers && bulkEl) bulkEl.hidden = true;
              }, 300);
            }
          } else {
            resp.json().then(function(data) {
              if (sessionsFeedback) showFeedback(sessionsFeedback, data.error || 'Failed to revoke session. Try again.', 'error');
            });
          }
        })
        .catch(function() {
          if (sessionsFeedback) showFeedback(sessionsFeedback, 'Failed to revoke session. Try again.', 'error');
        });
      } else {
        hideRevokeModal();
        fetch('/api/sessions', {
          method: 'DELETE',
          headers: { 'X-CSRF-Token': getCSRFToken() }
        })
        .then(function(resp) {
          if (resp.ok) {
            refreshSessions();
          } else {
            resp.json().then(function(data) {
              if (sessionsFeedback) showFeedback(sessionsFeedback, data.error || 'Failed to revoke sessions.', 'error');
            });
          }
        })
        .catch(function() {
          if (sessionsFeedback) showFeedback(sessionsFeedback, 'Failed to revoke sessions.', 'error');
        });
      }
    });
  }

  if (revokeCancelBtn) {
    revokeCancelBtn.addEventListener('click', hideRevokeModal);
  }

  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape' && revokeOverlay && revokeOverlay.classList.contains('visible')) {
      hideRevokeModal();
    }
  });

  if (revokeOverlay) {
    revokeOverlay.addEventListener('keydown', function(e) {
      if (e.key !== 'Tab') return;
      var focusable = [revokeCancelBtn, revokeConfirmBtn];
      var first = focusable[0];
      var last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    });
  }

  if (sessionsRefresh) {
    sessionsRefresh.addEventListener('click', refreshSessions);
  }

  // --- Tag Management Section ---

  var TAGS_INITIAL = 7;
  var TAGS_BATCH = 15;
  var tagsLimit = TAGS_INITIAL;

  var tagsData = [];
  var tagsSortCol = 'name';
  var tagsSortAsc = true;
  var tagsFilterText = '';
  var tagsEditingId = null;
  var tagsFilterTimer = null;

  var tagsTbody = document.getElementById('tags-tbody');
  var tagsEmpty = document.getElementById('tags-empty');
  var tagsTableWrap = document.getElementById('tags-table-wrap');
  var tagsFilterInput = document.getElementById('tag-filter-input');
  var tagsFeedback = document.getElementById('tags-feedback');

  function createSVGIcon(paths) {
    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('width', '14');
    svg.setAttribute('height', '14');
    svg.setAttribute('viewBox', '0 0 14 14');
    svg.setAttribute('fill', 'none');
    svg.setAttribute('stroke', 'currentColor');
    svg.setAttribute('stroke-width', '1.5');
    svg.setAttribute('stroke-linecap', 'round');
    svg.setAttribute('stroke-linejoin', 'round');
    if (!Array.isArray(paths)) {
      paths = [paths];
    }
    for (var i = 0; i < paths.length; i++) {
      var p = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      p.setAttribute('d', paths[i]);
      svg.appendChild(p);
    }
    return svg;
  }

  function fetchTags() {
    var url = '/api/tags';
    if (tagsFilterText) {
      url += '?q=' + encodeURIComponent(tagsFilterText);
    }
    fetch(url, { credentials: 'same-origin' })
      .then(function(resp) { return resp.json(); })
      .then(function(data) {
        tagsData = data || [];
        tagsLimit = TAGS_INITIAL;
        if (tagsData.length === 0 && !tagsFilterText) {
          tagsEmpty.hidden = false;
          tagsTableWrap.hidden = true;
        } else {
          tagsEmpty.hidden = true;
          tagsTableWrap.hidden = false;
        }
        sortAndRender();
      })
      .catch(function() {
        showFeedback(tagsFeedback, 'Failed to load tags. Please try again.', 'error');
      });
  }

  function sortAndRender() {
    var sorted = tagsData.slice();
    sorted.sort(function(a, b) {
      var result;
      if (tagsSortCol === 'name') {
        result = a.name.localeCompare(b.name);
      } else {
        result = a.item_count - b.item_count;
        if (result === 0) {
          result = a.name.localeCompare(b.name);
        }
      }
      return tagsSortAsc ? result : -result;
    });

    while (tagsTbody.firstChild) {
      tagsTbody.removeChild(tagsTbody.firstChild);
    }

    if (sorted.length === 0 && tagsFilterText) {
      var emptyRow = document.createElement('tr');
      var emptyCell = document.createElement('td');
      emptyCell.setAttribute('colspan', '3');
      emptyCell.style.textAlign = 'center';
      emptyCell.style.color = 'var(--text-muted)';
      emptyCell.style.padding = 'var(--space-lg)';
      emptyCell.textContent = 'No tags matching "' + tagsFilterText + '"';
      emptyRow.appendChild(emptyCell);
      tagsTbody.appendChild(emptyRow);
    } else {
      // Show all when filtering, paginate otherwise
      var limit = tagsFilterText ? sorted.length : tagsLimit;
      var visible = sorted.slice(0, limit);
      for (var i = 0; i < visible.length; i++) {
        tagsTbody.appendChild(renderTagRow(visible[i]));
      }

      // Manage load-more button
      var existingLoadMore = document.getElementById('tags-load-more');
      if (existingLoadMore) existingLoadMore.remove();

      if (!tagsFilterText) {
        var remaining = sorted.length - limit;
        if (remaining > 0) {
          var wrap = document.createElement('div');
          wrap.id = 'tags-load-more';
          wrap.style.cssText = 'margin-top:12px; text-align:center; padding-top:12px; border-top:1px solid var(--border)';
          var loadBtn = document.createElement('button');
          loadBtn.className = 'secondary';
          loadBtn.textContent = 'Show more tags (' + remaining + ' remaining)';
          loadBtn.addEventListener('click', function() {
            tagsLimit += TAGS_BATCH;
            sortAndRender();
          });
          wrap.appendChild(loadBtn);
          tagsTableWrap.parentNode.insertBefore(wrap, tagsTableWrap.nextSibling);
        }
      }
    }

    // Update sort indicators
    var headers = document.querySelectorAll('.tags-table th.sortable');
    for (var h = 0; h < headers.length; h++) {
      var th = headers[h];
      var col = th.getAttribute('data-sort');
      var indicator = th.querySelector('.sort-indicator');
      if (col === tagsSortCol) {
        th.setAttribute('aria-sort', tagsSortAsc ? 'ascending' : 'descending');
        if (indicator) indicator.textContent = tagsSortAsc ? '\u25B2' : '\u25BC';
      } else {
        th.removeAttribute('aria-sort');
        if (indicator) indicator.textContent = '';
      }
    }
  }

  function renderTagRow(tag) {
    var tr = document.createElement('tr');
    tr.setAttribute('data-tag-id', tag.id);

    // Name cell
    var tdName = document.createElement('td');
    tdName.textContent = tag.name;
    tr.appendChild(tdName);

    // Count cell
    var tdCount = document.createElement('td');
    tdCount.textContent = tag.item_count;
    if (tag.item_count === 0) {
      tdCount.style.color = 'var(--text-muted)';
    }
    tr.appendChild(tdCount);

    // Actions cell
    var tdActions = document.createElement('td');
    tdActions.className = 'tags-col-actions';

    var renameBtn = document.createElement('button');
    renameBtn.type = 'button';
    renameBtn.className = 'ghost';
    renameBtn.setAttribute('aria-label', 'Rename tag ' + tag.name);
    renameBtn.appendChild(createSVGIcon('M10 1.5l2.5 2.5L4.5 12H2v-2.5L10 1.5z'));
    renameBtn.addEventListener('click', function() { startEdit(tag); });
    tdActions.appendChild(renameBtn);

    if (tag.item_count === 0) {
      var deleteBtn = document.createElement('button');
      deleteBtn.type = 'button';
      deleteBtn.className = 'ghost danger-ghost';
      deleteBtn.setAttribute('aria-label', 'Delete tag ' + tag.name);
      deleteBtn.appendChild(createSVGIcon([
        'M2 3.5h10',
        'M5 3.5V2a1 1 0 011-1h2a1 1 0 011 1v1.5',
        'M11 3.5l-.5 8a1 1 0 01-1 1h-5a1 1 0 01-1-1L3 3.5'
      ]));
      deleteBtn.addEventListener('click', function() { startDelete(tag); });
      tdActions.appendChild(deleteBtn);
    }

    tr.appendChild(tdActions);

    // If another tag is being edited, disable actions on this row
    if (tagsEditingId !== null && tagsEditingId !== tag.id) {
      tdActions.classList.add('tags-actions-disabled');
    }

    return tr;
  }

  // Sort header click handlers
  var sortHeaders = document.querySelectorAll('.tags-table th.sortable');
  for (var si = 0; si < sortHeaders.length; si++) {
    (function(th) {
      th.addEventListener('click', function() {
        var col = th.getAttribute('data-sort');
        if (col === tagsSortCol) {
          tagsSortAsc = !tagsSortAsc;
        } else {
          tagsSortCol = col;
          tagsSortAsc = true;
        }
        cancelEdit();
        sortAndRender();
      });
    })(sortHeaders[si]);
  }

  // Filter input handler with debounce
  if (tagsFilterInput) {
    tagsFilterInput.addEventListener('input', function() {
      if (tagsFilterTimer) clearTimeout(tagsFilterTimer);
      tagsFilterTimer = setTimeout(function() {
        tagsFilterText = tagsFilterInput.value.trim().toLowerCase();
        if (tagsEditingId !== null) cancelEdit();
        fetchTags();
      }, 300);
    });
  }

  function startEdit(tag) {
    if (tagsEditingId !== null) cancelEdit();
    tagsEditingId = tag.id;

    var row = tagsTbody.querySelector('tr[data-tag-id="' + tag.id + '"]');
    if (!row) return;

    var cells = row.querySelectorAll('td');
    var nameCell = cells[0];
    var actionsCell = cells[2];

    // Replace name cell with input
    while (nameCell.firstChild) nameCell.removeChild(nameCell.firstChild);
    var input = document.createElement('input');
    input.type = 'text';
    input.className = 'tag-rename-input';
    input.value = tag.name;
    nameCell.appendChild(input);
    input.focus();
    input.select();

    // Keyboard handlers
    input.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        saveRename(tag);
      } else if (e.key === 'Escape') {
        e.preventDefault();
        cancelEdit();
      }
    });

    // Replace actions with save/cancel
    while (actionsCell.firstChild) actionsCell.removeChild(actionsCell.firstChild);

    var saveBtn = document.createElement('button');
    saveBtn.type = 'button';
    saveBtn.className = 'ghost';
    saveBtn.setAttribute('aria-label', 'Save');
    saveBtn.appendChild(createSVGIcon('M2 7l3.5 3.5L12 3'));
    saveBtn.addEventListener('click', function() { saveRename(tag); });
    actionsCell.appendChild(saveBtn);

    var cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.className = 'ghost';
    cancelBtn.setAttribute('aria-label', 'Cancel');
    cancelBtn.appendChild(createSVGIcon(['M3 3l8 8', 'M11 3l-8 8']));
    cancelBtn.addEventListener('click', function() { cancelEdit(); });
    actionsCell.appendChild(cancelBtn);

    // Disable other rows' actions
    var allRows = tagsTbody.querySelectorAll('tr');
    for (var r = 0; r < allRows.length; r++) {
      if (allRows[r].getAttribute('data-tag-id') !== String(tag.id)) {
        var acts = allRows[r].querySelector('.tags-col-actions');
        if (acts) acts.classList.add('tags-actions-disabled');
      }
    }
  }

  function cancelEdit() {
    tagsEditingId = null;
    sortAndRender();
  }

  function saveRename(tag) {
    var row = tagsTbody.querySelector('tr[data-tag-id="' + tag.id + '"]');
    if (!row) return;
    var input = row.querySelector('.tag-rename-input');
    if (!input) return;

    var newName = input.value.trim().toLowerCase();

    // Client-side validation
    if (!newName) {
      showFeedback(tagsFeedback, 'Tag name cannot be empty', 'error');
      return;
    }
    if (newName.length > 50) {
      showFeedback(tagsFeedback, 'Tag name must be 50 characters or fewer', 'error');
      return;
    }
    if (newName === tag.name) {
      showFeedback(tagsFeedback, 'Tag name unchanged', 'error');
      cancelEdit();
      return;
    }

    var saveBtn = row.querySelector('button[aria-label="Save"]');
    if (saveBtn) setBusy(saveBtn, true);
    clearFeedback(tagsFeedback);

    fetch('/api/tags/' + tag.id, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': getCSRFToken()
      },
      credentials: 'same-origin',
      body: JSON.stringify({ name: newName })
    })
    .then(function(resp) {
      return resp.json().then(function(data) {
        return { ok: resp.ok, status: resp.status, data: data };
      });
    })
    .then(function(result) {
      if (saveBtn) setBusy(saveBtn, false);

      if (result.status === 200 && result.data.ok) {
        // Success - flash row green
        showFeedback(tagsFeedback, 'Tag renamed', 'success');
        tagsEditingId = null;
        fetchTags();
        // Flash after re-render
        setTimeout(function() {
          var updatedRow = tagsTbody.querySelector('tr[data-tag-id="' + tag.id + '"]');
          if (updatedRow) {
            updatedRow.classList.add('tag-row-success');
            setTimeout(function() {
              updatedRow.classList.remove('tag-row-success');
            }, 1500);
          }
        }, 100);
      } else if (result.status === 409 && result.data.merge_needed) {
        showMergeConfirm(tag, result.data);
      } else if (result.status === 400) {
        showFeedback(tagsFeedback, result.data.error || 'Invalid tag name', 'error');
      } else {
        showFeedback(tagsFeedback, 'Something went wrong. Please try again.', 'error');
      }
    })
    .catch(function() {
      if (saveBtn) setBusy(saveBtn, false);
      showFeedback(tagsFeedback, 'Something went wrong. Please try again.', 'error');
    });
  }

  function showMergeConfirm(tag, responseData) {
    var row = tagsTbody.querySelector('tr[data-tag-id="' + tag.id + '"]');
    if (!row) return;

    var cells = row.querySelectorAll('td');
    var nameCell = cells[0];
    var actionsCell = cells[2];

    // Replace name cell with merge prompt
    while (nameCell.firstChild) nameCell.removeChild(nameCell.firstChild);
    var prompt = document.createElement('p');
    prompt.className = 'tags-merge-prompt';
    prompt.textContent = "Tag '" + responseData.target.name + "' already exists (" +
      responseData.target.item_count + " items). Merge '" + responseData.source.name +
      "' (" + responseData.source.item_count + " items) into '" + responseData.target.name + "'?";
    nameCell.appendChild(prompt);

    // Replace actions with cancel/merge buttons
    while (actionsCell.firstChild) actionsCell.removeChild(actionsCell.firstChild);

    var cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.className = 'ghost';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.addEventListener('click', function() { cancelEdit(); });
    actionsCell.appendChild(cancelBtn);

    var mergeBtn = document.createElement('button');
    mergeBtn.type = 'button';
    mergeBtn.className = 'btn danger';
    mergeBtn.style.fontSize = '13px';
    mergeBtn.textContent = 'Merge';
    mergeBtn.addEventListener('click', function() { doMerge(tag.id, responseData.target.name); });
    actionsCell.appendChild(mergeBtn);
  }

  function doMerge(tagId, targetName) {
    var row = tagsTbody.querySelector('tr[data-tag-id="' + tagId + '"]');
    var mergeBtn = row ? row.querySelector('.btn.danger') : null;
    if (mergeBtn) setBusy(mergeBtn, true);
    clearFeedback(tagsFeedback);

    fetch('/api/tags/' + tagId, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': getCSRFToken()
      },
      credentials: 'same-origin',
      body: JSON.stringify({ name: targetName, force_merge: true })
    })
    .then(function(resp) {
      return resp.json().then(function(data) {
        return { ok: resp.ok, status: resp.status, data: data };
      });
    })
    .then(function(result) {
      if (mergeBtn) setBusy(mergeBtn, false);
      if (result.ok) {
        showFeedback(tagsFeedback, 'Tags merged', 'success');
        tagsEditingId = null;
        fetchTags();
      } else {
        showFeedback(tagsFeedback, result.data.error || 'Merge failed. Please try again.', 'error');
      }
    })
    .catch(function() {
      if (mergeBtn) setBusy(mergeBtn, false);
      showFeedback(tagsFeedback, 'Something went wrong. Please try again.', 'error');
    });
  }

  function startDelete(tag) {
    if (tagsEditingId !== null) cancelEdit();
    tagsEditingId = tag.id;

    var row = tagsTbody.querySelector('tr[data-tag-id="' + tag.id + '"]');
    if (!row) return;

    var cells = row.querySelectorAll('td');
    var nameCell = cells[0];
    var countCell = cells[1];
    var actionsCell = cells[2];

    // Replace name cell with confirmation text
    while (nameCell.firstChild) nameCell.removeChild(nameCell.firstChild);
    var confirmText = document.createElement('span');
    confirmText.textContent = "Delete tag '" + tag.name + "'?";
    nameCell.appendChild(confirmText);

    // Clear count cell
    while (countCell.firstChild) countCell.removeChild(countCell.firstChild);

    // Replace actions with cancel/delete buttons
    while (actionsCell.firstChild) actionsCell.removeChild(actionsCell.firstChild);

    var cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.className = 'ghost';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.addEventListener('click', function() { cancelEdit(); });
    actionsCell.appendChild(cancelBtn);

    var deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className = 'btn danger';
    deleteBtn.style.fontSize = '13px';
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', function() { doDelete(tag); });
    actionsCell.appendChild(deleteBtn);

    // Disable other rows' actions
    var allRows = tagsTbody.querySelectorAll('tr');
    for (var r = 0; r < allRows.length; r++) {
      if (allRows[r].getAttribute('data-tag-id') !== String(tag.id)) {
        var acts = allRows[r].querySelector('.tags-col-actions');
        if (acts) acts.classList.add('tags-actions-disabled');
      }
    }
  }

  function doDelete(tag) {
    var row = tagsTbody.querySelector('tr[data-tag-id="' + tag.id + '"]');
    var deleteBtn = row ? row.querySelector('.btn.danger') : null;
    if (deleteBtn) setBusy(deleteBtn, true);
    clearFeedback(tagsFeedback);

    fetch('/api/tags/' + tag.id, {
      method: 'DELETE',
      headers: { 'X-CSRF-Token': getCSRFToken() },
      credentials: 'same-origin'
    })
    .then(function(resp) {
      if (resp.status === 204) {
        // Success - fade out and remove
        if (row) {
          row.classList.add('tag-row-removing');
          setTimeout(function() {
            if (row.parentNode) row.parentNode.removeChild(row);
          }, 300);
        }
        showFeedback(tagsFeedback, 'Tag deleted', 'success');
        tagsEditingId = null;
        setTimeout(function() { fetchTags(); }, 400);
      } else if (resp.status === 409) {
        resp.json().then(function(data) {
          showFeedback(tagsFeedback, data.error || 'Tag is in use', 'error');
          if (deleteBtn) setBusy(deleteBtn, false);
          cancelEdit();
        });
      } else {
        showFeedback(tagsFeedback, 'Something went wrong. Please try again.', 'error');
        if (deleteBtn) setBusy(deleteBtn, false);
        cancelEdit();
      }
    })
    .catch(function() {
      if (deleteBtn) setBusy(deleteBtn, false);
      showFeedback(tagsFeedback, 'Something went wrong. Please try again.', 'error');
    });
  }

  // Initialize tags section
  if (tagsTbody) {
    fetchTags();
  }

  // --- Housekeeping: Duplicate Detection ---

  var housekeepingLoaded = false;

  function loadDuplicates() {
    if (housekeepingLoaded) return;
    var content = document.getElementById('housekeeping-content');
    if (!content) return;

    var csrf = getCSRFToken();
    fetch('/api/duplicates', {
      headers: { 'X-CSRF-Token': csrf }
    })
    .then(function(res) {
      if (!res.ok) throw new Error('HTTP ' + res.status);
      return res.json();
    })
    .then(function(groups) {
      housekeepingLoaded = true;
      while (content.firstChild) content.removeChild(content.firstChild);

      if (!groups || groups.length === 0) {
        var empty = document.createElement('div');
        empty.className = 'housekeeping-empty';
        var p1 = document.createElement('p');
        p1.textContent = 'No duplicates found';
        var p2 = document.createElement('p');
        p2.className = 'text-muted';
        p2.textContent = 'All items are unique across containers.';
        empty.appendChild(p1);
        empty.appendChild(p2);
        content.appendChild(empty);
        return;
      }

      groups.forEach(function(group) {
        content.appendChild(renderDuplicateCard(group));
      });
    })
    .catch(function() {
      while (content.firstChild) content.removeChild(content.firstChild);
      var errP = document.createElement('p');
      errP.className = 'text-muted';
      errP.textContent = 'Could not load duplicates. Try refreshing the page.';
      content.appendChild(errP);
    });
  }

  function renderDuplicateCard(group) {
    var card = document.createElement('div');
    card.className = 'duplicate-group';

    var header = document.createElement('div');
    header.className = 'duplicate-header';

    var name = document.createElement('strong');
    name.className = 'duplicate-name';
    name.textContent = group.name;

    var count = document.createElement('span');
    count.className = 'duplicate-count';
    count.textContent = group.count + ' copies';

    header.appendChild(name);
    header.appendChild(count);
    card.appendChild(header);

    if (group.tags && group.tags.length > 0) {
      var tagsDiv = document.createElement('div');
      tagsDiv.className = 'duplicate-tags';
      group.tags.forEach(function(tag) {
        var chip = document.createElement('span');
        chip.className = 'tag-chip';
        chip.textContent = tag;
        tagsDiv.appendChild(chip);
      });
      card.appendChild(tagsDiv);
    } else {
      var noTags = document.createElement('div');
      noTags.className = 'duplicate-tags-none';
      noTags.textContent = 'No tags';
      card.appendChild(noTags);
    }

    var locations = document.createElement('div');
    locations.className = 'duplicate-locations';

    var prefix = document.createElement('span');
    prefix.textContent = 'Found in: ';
    locations.appendChild(prefix);

    group.containers.forEach(function(loc, idx) {
      if (idx > 0) {
        locations.appendChild(document.createTextNode(', '));
      }
      var link = document.createElement('a');
      link.className = 'duplicate-container-link';
      link.href = '/?cell=' + encodeURIComponent(loc.label);
      link.textContent = loc.label;
      locations.appendChild(link);
    });

    card.appendChild(locations);
    return card;
  }

  if (window.location.hash === '#housekeeping') {
    loadDuplicates();
  }

  // --- Avatar Pill: Initials + Dropdown ---

  (function initAvatarDropdown() {
    var pill = document.getElementById('avatar-pill');
    if (!pill) return;

    var initialsEl = pill.querySelector('.avatar-initials');
    if (initialsEl) {
      var name = initialsEl.getAttribute('data-display-name') || '';
      var parts = name.trim().split(/\s+/);
      var initials;
      if (parts.length >= 2) {
        initials = parts[0][0] + parts[parts.length - 1][0];
      } else if (parts[0] && parts[0].length >= 2) {
        initials = parts[0].substring(0, 2);
      } else {
        initials = parts[0] ? parts[0][0] : '?';
      }
      initialsEl.textContent = initials.toUpperCase();
    }

    var nameEl = pill.querySelector('.avatar-name');
    if (nameEl) {
      var fullName = nameEl.textContent.trim();
      var firstName = fullName.split(/\s+/)[0];
      nameEl.textContent = firstName;
    }

    var dropdown = document.getElementById('avatar-dropdown');
    if (!dropdown) return;

    pill.addEventListener('click', function(e) {
      e.stopPropagation();
      var isOpen = dropdown.getAttribute('data-open') === 'true';
      dropdown.setAttribute('data-open', isOpen ? 'false' : 'true');
      pill.setAttribute('aria-expanded', isOpen ? 'false' : 'true');
    });

    document.addEventListener('click', function(e) {
      if (!pill.contains(e.target) && !dropdown.contains(e.target)) {
        dropdown.setAttribute('data-open', 'false');
        pill.setAttribute('aria-expanded', 'false');
      }
    });

    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape' && dropdown.getAttribute('data-open') === 'true') {
        dropdown.setAttribute('data-open', 'false');
        pill.setAttribute('aria-expanded', 'false');
        pill.focus();
      }
    });
  })();

})();
