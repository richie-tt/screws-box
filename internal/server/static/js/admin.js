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
    el.className = 'admin-form-feedback ' + type;
    if (type === 'success') {
      setTimeout(function() {
        el.textContent = '';
        el.className = 'admin-form-feedback';
      }, 2000);
    }
  }

  function clearFeedback(el) {
    el.textContent = '';
    el.className = 'admin-form-feedback';
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
    var nameFeedback = shelfNameForm.querySelector('.admin-form-feedback');
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
    var resizeFeedback = resizeForm.querySelector('.admin-form-feedback');
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
    var authFeedback = authForm.querySelector('.admin-form-feedback');

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
      var feedback = oidcForm.querySelector('.admin-form-feedback');
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

  var navItems = document.querySelectorAll('.admin-nav-item:not(.disabled)');
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
  var sections = document.querySelectorAll('.admin-card[id]');
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
      exportFeedback.className = 'admin-form-feedback';
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
    importFeedback.className = 'admin-form-feedback';
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
        importFeedback.className = 'admin-form-feedback error';
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
          importFeedback.className = 'admin-form-feedback success';
          currentImportToken = null;
          setTimeout(function() { location.reload(); }, 2000);
        } else {
          importConfirmBtn.textContent = 'Replace All Data';
          importConfirmBtn.disabled = false;
          importFeedback.textContent = result.error || 'Import failed. Existing data was not modified.';
          importFeedback.className = 'admin-form-feedback error';
          importSummaryArea.hidden = true;
          importUploadArea.hidden = false;
        }
      } catch (e) {
        importConfirmBtn.textContent = 'Replace All Data';
        importConfirmBtn.disabled = false;
        importFeedback.textContent = 'Import request failed.';
        importFeedback.className = 'admin-form-feedback error';
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

})();
