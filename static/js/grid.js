// grid.js -- Item CRUD dialog logic
// Phase 06 Plan 02: Full dialog, CRUD, and DOM update logic
(function () {
  'use strict';

  // --- Section 1: DOM references and state ---

  const dialog = document.getElementById('item-dialog');
  const dialogCoord = document.getElementById('dialog-coord');
  const dialogBody = document.getElementById('dialog-body');
  const dialogFooter = document.getElementById('dialog-footer');
  const dialogClose = document.getElementById('dialog-close');
  const gridContainer = document.querySelector('.grid-container');

  let currentContainerId = null;

  // --- Section 2: Dialog open/close ---

  // Event delegation: single click listener on grid container
  gridContainer.addEventListener('click', function (e) {
    var cell = e.target.closest('.grid-cell');
    if (!cell) return;
    var coord = cell.dataset.coord;
    var containerId = cell.dataset.containerId;
    openDialog(coord, containerId);
  });

  async function openDialog(coord, containerId) {
    dialogCoord.textContent = coord;
    currentContainerId = containerId;
    dialogBody.textContent = '';
    dialogFooter.textContent = '';

    try {
      var resp = await fetch('/api/containers/' + containerId + '/items');
      if (!resp.ok) {
        throw new Error('Nie udalo sie zaladowac elementow');
      }
      var data = await resp.json();
      if (data.items && data.items.length > 0) {
        renderItemList(data.items);
      } else {
        renderAddForm();
      }
    } catch (err) {
      dialogBody.textContent = err.message || 'Blad ladowania';
    }

    dialog.showModal();
  }

  // Close button
  dialogClose.addEventListener('click', function () {
    dialog.close();
  });

  // Backdrop click to close (Pitfall 4: native dialog does NOT close on backdrop click)
  dialog.addEventListener('click', function (e) {
    if (e.target === dialog) {
      dialog.close();
    }
  });

  // --- Section 3: Render item list (D-03, D-15) ---

  function renderItemList(items) {
    dialogBody.textContent = '';
    dialogFooter.textContent = '';

    var ul = document.createElement('ul');
    ul.style.listStyle = 'none';
    ul.style.padding = '0';

    items.forEach(function (item) {
      var li = document.createElement('li');
      li.style.marginBottom = '0.75rem';
      li.style.paddingBottom = '0.75rem';
      li.style.borderBottom = '1px solid var(--pico-muted-border-color)';

      // Item name
      var nameEl = document.createElement('strong');
      nameEl.textContent = item.name;
      li.appendChild(nameEl);

      // Tag badges
      if (item.tags && item.tags.length > 0) {
        var tagsDiv = document.createElement('div');
        tagsDiv.className = 'item-tags';
        item.tags.forEach(function (tag) {
          var badge = document.createElement('span');
          badge.className = 'tag-badge';
          badge.textContent = tag;
          tagsDiv.appendChild(badge);
        });
        li.appendChild(tagsDiv);
      }

      // Action buttons container
      var actions = document.createElement('div');
      actions.style.marginTop = '0.5rem';
      actions.style.display = 'flex';
      actions.style.gap = '0.5rem';

      // Edit button
      var editBtn = document.createElement('button');
      editBtn.className = 'outline secondary';
      editBtn.style.padding = '0.25rem 0.5rem';
      editBtn.style.fontSize = '0.8rem';
      editBtn.textContent = 'Edytuj';
      editBtn.type = 'button';
      editBtn.addEventListener('click', function () {
        renderEditForm(item);
      });
      actions.appendChild(editBtn);

      // Delete button
      var deleteBtn = document.createElement('button');
      deleteBtn.className = 'outline';
      deleteBtn.style.padding = '0.25rem 0.5rem';
      deleteBtn.style.fontSize = '0.8rem';
      deleteBtn.textContent = 'Usun';
      deleteBtn.type = 'button';
      deleteBtn.addEventListener('click', function () {
        handleDelete(deleteBtn, item);
      });
      actions.appendChild(deleteBtn);

      li.appendChild(actions);
      ul.appendChild(li);
    });

    dialogBody.appendChild(ul);

    // "Dodaj element" button in footer
    var addBtn = document.createElement('button');
    addBtn.textContent = 'Dodaj element';
    addBtn.type = 'button';
    addBtn.addEventListener('click', function () {
      renderAddForm();
    });
    dialogFooter.appendChild(addBtn);
  }

  // --- Section 4: Add form (D-02, D-05, D-06, D-07, D-08, D-09) ---

  function renderAddForm() {
    dialogBody.textContent = '';
    dialogFooter.textContent = '';

    var form = document.createElement('form');
    form.id = 'item-form';

    // Name field
    var nameLabel = document.createElement('label');
    nameLabel.textContent = 'Nazwa ';
    var nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.name = 'name';
    nameInput.required = true;
    nameInput.placeholder = 'np. Sruba M6x20';
    nameLabel.appendChild(nameInput);
    form.appendChild(nameLabel);

    // Tags field
    var tagsLabel = document.createElement('label');
    tagsLabel.textContent = 'Tagi (po przecinku) ';
    var tagsInput = document.createElement('input');
    tagsInput.type = 'text';
    tagsInput.name = 'tags';
    tagsInput.required = true;
    tagsInput.placeholder = 'np. m6, din933, hex';
    tagsLabel.appendChild(tagsInput);
    form.appendChild(tagsLabel);

    // Error area
    var errorSmall = document.createElement('small');
    errorSmall.id = 'form-error';
    errorSmall.setAttribute('aria-live', 'polite');
    errorSmall.style.color = 'var(--pico-del-color)';
    form.appendChild(errorSmall);

    // Submit button
    var submitBtn = document.createElement('button');
    submitBtn.type = 'submit';
    submitBtn.textContent = 'Dodaj';
    form.appendChild(submitBtn);

    form.addEventListener('submit', async function (e) {
      e.preventDefault();
      errorSmall.textContent = '';
      nameInput.removeAttribute('aria-invalid');
      tagsInput.removeAttribute('aria-invalid');

      var name = nameInput.value.trim();
      if (!name) {
        nameInput.setAttribute('aria-invalid', 'true');
        errorSmall.textContent = 'Nazwa jest wymagana';
        return;
      }

      var tags = tagsInput.value.split(',')
        .map(function (s) { return s.trim().toLowerCase(); })
        .filter(Boolean);
      tags = Array.from(new Set(tags));

      if (tags.length === 0) {
        tagsInput.setAttribute('aria-invalid', 'true');
        errorSmall.textContent = 'Wymagany co najmniej jeden tag';
        return;
      }

      try {
        submitBtn.setAttribute('aria-busy', 'true');
        var resp = await fetch('/api/items', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: name,
            container_id: parseInt(currentContainerId, 10),
            tags: tags
          })
        });

        if (!resp.ok) {
          var errData = await resp.json();
          errorSmall.textContent = errData.error || 'Blad tworzenia elementu';
          return;
        }

        updateCellCount(currentContainerId, +1);
        dialog.close();
      } catch (err) {
        errorSmall.textContent = err.message || 'Blad sieci';
      } finally {
        submitBtn.removeAttribute('aria-busy');
      }
    });

    dialogBody.appendChild(form);
  }

  // --- Section 5: Edit form (D-05, D-17, D-13) ---

  function renderEditForm(item) {
    dialogBody.textContent = '';
    dialogFooter.textContent = '';

    var form = document.createElement('form');
    form.id = 'item-form';

    // Name field (pre-filled)
    var nameLabel = document.createElement('label');
    nameLabel.textContent = 'Nazwa ';
    var nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.name = 'name';
    nameInput.required = true;
    nameInput.value = item.name;
    nameLabel.appendChild(nameInput);
    form.appendChild(nameLabel);

    // Tags field (pre-filled)
    var tagsLabel = document.createElement('label');
    tagsLabel.textContent = 'Tagi (po przecinku) ';
    var tagsInput = document.createElement('input');
    tagsInput.type = 'text';
    tagsInput.name = 'tags';
    tagsInput.required = true;
    tagsInput.value = (item.tags || []).join(', ');
    tagsLabel.appendChild(tagsInput);
    form.appendChild(tagsLabel);

    // Error area
    var errorSmall = document.createElement('small');
    errorSmall.id = 'form-error';
    errorSmall.setAttribute('aria-live', 'polite');
    errorSmall.style.color = 'var(--pico-del-color)';
    form.appendChild(errorSmall);

    // Button row
    var btnRow = document.createElement('div');
    btnRow.style.display = 'flex';
    btnRow.style.gap = '0.5rem';

    var submitBtn = document.createElement('button');
    submitBtn.type = 'submit';
    submitBtn.textContent = 'Zapisz';
    btnRow.appendChild(submitBtn);

    var cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.className = 'secondary';
    cancelBtn.textContent = 'Anuluj';
    cancelBtn.addEventListener('click', async function () {
      // Re-fetch and re-render item list
      try {
        var resp = await fetch('/api/containers/' + currentContainerId + '/items');
        if (!resp.ok) throw new Error('Blad ladowania');
        var data = await resp.json();
        if (data.items && data.items.length > 0) {
          renderItemList(data.items);
        } else {
          renderAddForm();
        }
      } catch (err) {
        dialogBody.textContent = err.message;
      }
    });
    btnRow.appendChild(cancelBtn);

    form.appendChild(btnRow);

    form.addEventListener('submit', async function (e) {
      e.preventDefault();
      errorSmall.textContent = '';
      nameInput.removeAttribute('aria-invalid');
      tagsInput.removeAttribute('aria-invalid');

      var name = nameInput.value.trim();
      if (!name) {
        nameInput.setAttribute('aria-invalid', 'true');
        errorSmall.textContent = 'Nazwa jest wymagana';
        return;
      }

      var newTags = tagsInput.value.split(',')
        .map(function (s) { return s.trim().toLowerCase(); })
        .filter(Boolean);
      newTags = Array.from(new Set(newTags));

      if (newTags.length === 0) {
        tagsInput.setAttribute('aria-invalid', 'true');
        errorSmall.textContent = 'Wymagany co najmniej jeden tag';
        return;
      }

      try {
        submitBtn.setAttribute('aria-busy', 'true');

        // PUT to update name/container (does NOT update tags)
        var putResp = await fetch('/api/items/' + item.id, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: name,
            container_id: parseInt(currentContainerId, 10),
            description: null
          })
        });

        if (!putResp.ok) {
          var errData = await putResp.json();
          errorSmall.textContent = errData.error || 'Blad aktualizacji';
          return;
        }

        // Tag diff: compute adds and removes
        var oldTags = item.tags || [];
        var toAdd = newTags.filter(function (t) { return oldTags.indexOf(t) === -1; });
        var toRemove = oldTags.filter(function (t) { return newTags.indexOf(t) === -1; });

        // Remove tags
        for (var i = 0; i < toRemove.length; i++) {
          var delResp = await fetch('/api/items/' + item.id + '/tags/' + encodeURIComponent(toRemove[i]), {
            method: 'DELETE'
          });
          if (!delResp.ok && delResp.status !== 204) {
            var delErr = await delResp.json().catch(function () { return { error: 'Blad usuwania tagu' }; });
            errorSmall.textContent = delErr.error || 'Blad usuwania tagu';
            return;
          }
        }

        // Add tags
        for (var j = 0; j < toAdd.length; j++) {
          var addResp = await fetch('/api/items/' + item.id + '/tags', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: toAdd[j] })
          });
          if (!addResp.ok) {
            var addErr = await addResp.json().catch(function () { return { error: 'Blad dodawania tagu' }; });
            errorSmall.textContent = addErr.error || 'Blad dodawania tagu';
            return;
          }
        }

        dialog.close();
      } catch (err) {
        errorSmall.textContent = err.message || 'Blad sieci';
      } finally {
        submitBtn.removeAttribute('aria-busy');
      }
    });

    dialogBody.appendChild(form);
  }

  // --- Section 6: Delete with confirmation (D-16) ---

  function handleDelete(btn, item) {
    if (btn.dataset.confirm === 'true') {
      // Second click: actually delete
      performDelete(item);
    } else {
      // First click: show confirmation
      btn.dataset.confirm = 'true';
      btn.textContent = 'Na pewno?';
      btn.classList.add('contrast');
      btn.classList.remove('outline');

      // Reset after 3 seconds
      var resetTimer = setTimeout(function () {
        btn.dataset.confirm = '';
        btn.textContent = 'Usun';
        btn.classList.remove('contrast');
        btn.classList.add('outline');
      }, 3000);

      // Store timer so we can clear if deleted
      btn._resetTimer = resetTimer;
    }
  }

  async function performDelete(item) {
    try {
      var resp = await fetch('/api/items/' + item.id, {
        method: 'DELETE'
      });

      if (!resp.ok && resp.status !== 204) {
        var errData = await resp.json().catch(function () { return { error: 'Blad usuwania' }; });
        alert(errData.error || 'Blad usuwania elementu');
        return;
      }

      updateCellCount(currentContainerId, -1);

      // Re-fetch items to see if any remain
      var listResp = await fetch('/api/containers/' + currentContainerId + '/items');
      if (listResp.ok) {
        var data = await listResp.json();
        if (data.items && data.items.length > 0) {
          renderItemList(data.items);
        } else {
          dialog.close();
        }
      } else {
        dialog.close();
      }
    } catch (err) {
      alert(err.message || 'Blad sieci');
    }
  }

  // --- Section 7: Cell count update (D-10, D-11, D-12) ---

  function updateCellCount(containerId, delta) {
    var cell = document.querySelector('.grid-cell[data-container-id="' + containerId + '"]');
    if (!cell) return;

    var currentCount = parseInt(cell.dataset.count, 10) || 0;
    var newCount = currentCount + delta;
    if (newCount < 0) newCount = 0;

    // Update data attribute (reliable source of truth -- Pitfall 6)
    cell.dataset.count = newCount;

    // Update visible count text
    var countSpan = cell.querySelector('.cell-count');
    if (countSpan) {
      if (newCount <= 0) {
        countSpan.innerHTML = '&mdash;';
        cell.classList.add('cell-empty');
      } else {
        countSpan.textContent = newCount + ' elem.';
        cell.classList.remove('cell-empty');
      }
    }
  }

})();
