// grid.js -- Inline cell expansion CRUD logic
// Phase 06 Plan 02: Complete rewrite -- inline expansion, no dialog
(function () {
  'use strict';

  // --- Section 1: State and DOM references ---

  const gridContainer = document.querySelector('.grid-container');
  if (!gridContainer) return;

  let expandedCell = null;

  // --- Section 2: Utility functions ---

  function formatCount(n) {
    if (n === 0) return '\u2014'; // em-dash
    if (n === 1) return '1 item';
    return n + ' items';
  }

  function pulseCell(cell) {
    if (!cell) return;
    cell.classList.remove('pulse');
    // Force reflow so re-adding the class triggers animation
    void cell.offsetWidth;
    cell.classList.add('pulse');
    cell.addEventListener('animationend', function handler() {
      cell.classList.remove('pulse');
      cell.removeEventListener('animationend', handler);
    });
  }

  function updateCellCount(containerId, delta) {
    const cell = findCell(containerId);
    if (!cell) return;

    let current = parseInt(cell.dataset.count, 10) || 0;
    let newCount = current + delta;
    if (newCount < 0) newCount = 0;

    cell.dataset.count = newCount;

    const countSpan = cell.querySelector('.cell-count');
    if (countSpan) {
      countSpan.textContent = formatCount(newCount);
    }

    if (newCount === 0) {
      cell.classList.add('cell-empty');
    } else {
      cell.classList.remove('cell-empty');
    }
  }

  async function apiCall(url, options) {
    try {
      const resp = await fetch(url, options);
      if (resp.status === 204) {
        return { ok: true, status: 204, data: null };
      }
      const data = await resp.json().catch(() => null);
      return { ok: resp.ok, status: resp.status, data: data };
    } catch (e) {
      return { ok: false, status: 0, data: { error: 'Connection error. Please try again.' } };
    }
  }

  // --- Section 3: Expand/Collapse ---

  // Event delegation on grid container for cell clicks
  gridContainer.addEventListener('click', function (e) {
    const cell = e.target.closest('.grid-cell');
    if (!cell) return;
    // Do not re-expand an already expanded cell
    if (cell.classList.contains('expanded')) return;
    expandCell(cell);
  });

  async function expandCell(cell) {
    // Collapse any previously expanded cell first
    if (expandedCell) {
      collapseCell();
    }

    // Save original HTML for restore on collapse
    cell._originalHTML = cell.innerHTML;

    // Add expanded class
    cell.classList.add('expanded');

    // Clear cell contents
    cell.innerHTML = '';

    // Close button
    const closeBtn = document.createElement('button');
    closeBtn.className = 'expanded-close';
    closeBtn.type = 'button';
    closeBtn.textContent = '\u00D7';
    closeBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      collapseCell();
    });

    // Header with coord label
    const header = document.createElement('div');
    header.className = 'expanded-header';
    header.textContent = cell.dataset.coord;

    // Content container
    const content = document.createElement('div');

    cell.appendChild(closeBtn);
    cell.appendChild(header);
    cell.appendChild(content);

    expandedCell = cell;

    // Fetch items for this container
    const containerId = parseInt(cell.dataset.containerId, 10);
    const result = await apiCall('/api/containers/' + containerId + '/items');

    if (result.ok && result.data && result.data.items && result.data.items.length > 0) {
      renderItemList(cell, content, result.data.items, containerId);
    } else {
      renderAddForm(cell, content, containerId);
    }
  }

  function collapseCell() {
    if (!expandedCell) return;
    expandedCell.classList.remove('expanded');
    expandedCell.innerHTML = expandedCell._originalHTML || '';
    expandedCell = null;
  }

  // Click-outside handler
  document.addEventListener('click', function (e) {
    if (expandedCell && !expandedCell.contains(e.target)) {
      collapseCell();
    }
  });

  // Escape key handler
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
      collapseCell();
    }
  });

  // --- Section 4: Render item list ---

  function renderItemList(cell, container, items, containerId) {
    container.innerHTML = '';

    const ul = document.createElement('ul');
    ul.className = 'expanded-items';

    items.forEach(function (item) {
      const li = document.createElement('li');

      const row = document.createElement('div');
      row.className = 'item-row';

      // Item name
      const nameSpan = document.createElement('span');
      nameSpan.className = 'item-name';
      nameSpan.textContent = item.name;
      row.appendChild(nameSpan);

      // Tags as chips (no X button in list view)
      if (item.tags && item.tags.length > 0) {
        item.tags.forEach(function (tag) {
          const chip = document.createElement('kbd');
          chip.className = 'tag-chip';
          chip.textContent = tag;
          row.appendChild(chip);
        });
      }

      // Action buttons
      const actions = document.createElement('div');
      actions.className = 'item-actions';

      const editBtn = document.createElement('button');
      editBtn.className = 'outline secondary';
      editBtn.type = 'button';
      editBtn.textContent = 'Edit';
      editBtn.addEventListener('click', function (e) {
        e.stopPropagation();
        renderInlineEdit(li, item, cell, container, containerId);
      });
      actions.appendChild(editBtn);

      const deleteBtn = document.createElement('button');
      deleteBtn.className = 'outline';
      deleteBtn.type = 'button';
      deleteBtn.textContent = 'Delete';
      deleteBtn.addEventListener('click', function (e) {
        e.stopPropagation();
        handleDelete(deleteBtn, item, cell, container, containerId);
      });
      actions.appendChild(deleteBtn);

      row.appendChild(actions);
      li.appendChild(row);
      ul.appendChild(li);
    });

    // "Add item" button below the list
    const addBtn = document.createElement('button');
    addBtn.type = 'button';
    addBtn.textContent = 'Add item';
    addBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      renderAddForm(cell, container, containerId);
    });

    container.appendChild(ul);
    container.appendChild(addBtn);
  }

  // --- Section 5: Add item form ---

  function renderAddForm(cell, container, containerId) {
    container.innerHTML = '';

    const isEmpty = parseInt(cell.dataset.count, 10) === 0;

    if (isEmpty) {
      const emptyHeading = document.createElement('p');
      emptyHeading.className = 'expanded-empty';
      emptyHeading.innerHTML = '<strong>No items</strong><br>Add the first item to this container.';
      container.appendChild(emptyHeading);
    }

    const form = document.createElement('form');
    form.className = 'expanded-form';

    // Name field
    const nameLabel = document.createElement('label');
    nameLabel.textContent = 'Name';
    const nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.name = 'name';
    nameInput.required = true;
    nameLabel.appendChild(nameInput);
    form.appendChild(nameLabel);

    // Description field
    const descLabel = document.createElement('label');
    descLabel.textContent = 'Description';
    const descInput = document.createElement('textarea');
    descInput.name = 'description';
    descLabel.appendChild(descInput);
    form.appendChild(descLabel);

    // Tags section
    const tagsLabel = document.createElement('label');
    tagsLabel.textContent = 'Tags';
    form.appendChild(tagsLabel);

    const chipsContainer = document.createElement('div');
    chipsContainer.className = 'tag-chips-container';
    form.appendChild(chipsContainer);

    const tagInput = document.createElement('input');
    tagInput.type = 'text';
    tagInput.placeholder = 'Type tag, press Enter';
    form.appendChild(tagInput);

    const tagHint = document.createElement('small');
    tagHint.className = 'tag-hint';
    tagHint.textContent = 'Add at least one tag';
    tagHint.hidden = true;
    form.appendChild(tagHint);

    // Error area
    const errorEl = document.createElement('small');
    errorEl.className = 'form-error';
    errorEl.setAttribute('aria-live', 'polite');
    form.appendChild(errorEl);

    // Button row
    const formActions = document.createElement('div');
    formActions.className = 'form-actions';

    const submitBtn = document.createElement('button');
    submitBtn.type = 'submit';
    submitBtn.textContent = 'Add item';
    submitBtn.disabled = true;
    submitBtn.setAttribute('aria-disabled', 'true');
    formActions.appendChild(submitBtn);

    // Cancel button only if coming from item list (not empty cell)
    if (!isEmpty) {
      const cancelBtn = document.createElement('button');
      cancelBtn.type = 'button';
      cancelBtn.className = 'secondary';
      cancelBtn.textContent = 'Discard changes';
      cancelBtn.addEventListener('click', async function (e) {
        e.stopPropagation();
        // Re-fetch and re-render item list
        const result = await apiCall('/api/containers/' + containerId + '/items');
        if (result.ok && result.data && result.data.items && result.data.items.length > 0) {
          renderItemList(cell, container, result.data.items, containerId);
        } else {
          renderAddForm(cell, container, containerId);
        }
      });
      formActions.appendChild(cancelBtn);
    }

    form.appendChild(formActions);

    // Tag input behavior
    let pendingTags = [];

    function updateSubmitState() {
      const hasName = nameInput.value.trim().length > 0;
      const hasTags = pendingTags.length > 0;
      const enabled = hasName && hasTags;
      submitBtn.disabled = !enabled;
      submitBtn.setAttribute('aria-disabled', String(!enabled));
      if (hasTags) {
        tagHint.hidden = true;
      }
    }

    function renderTagChips() {
      chipsContainer.innerHTML = '';
      pendingTags.forEach(function (tag, index) {
        const chip = document.createElement('kbd');
        chip.className = 'tag-chip';
        chip.textContent = tag;

        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.textContent = '\u00D7';
        removeBtn.addEventListener('click', function (e) {
          e.stopPropagation();
          pendingTags.splice(index, 1);
          renderTagChips();
          updateSubmitState();
        });

        chip.appendChild(removeBtn);
        chipsContainer.appendChild(chip);
      });
    }

    tagInput.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') {
        e.preventDefault(); // CRITICAL: prevent form submit
        const val = tagInput.value.trim().toLowerCase();
        if (val && pendingTags.indexOf(val) === -1) {
          pendingTags.push(val);
          renderTagChips();
          tagInput.value = '';
          updateSubmitState();
        }
      }
    });

    nameInput.addEventListener('input', function () {
      updateSubmitState();
    });

    // Form submit
    form.addEventListener('submit', async function (e) {
      e.preventDefault();
      errorEl.textContent = '';
      nameInput.removeAttribute('aria-invalid');

      const name = nameInput.value.trim();
      if (!name) {
        nameInput.setAttribute('aria-invalid', 'true');
        errorEl.textContent = 'Name is required';
        return;
      }

      if (pendingTags.length === 0) {
        tagHint.hidden = false;
        return;
      }

      const desc = descInput.value.trim() || null;
      const body = {
        name: name,
        description: desc,
        container_id: containerId,
        tags: pendingTags
      };

      submitBtn.setAttribute('aria-busy', 'true');
      const result = await apiCall('/api/items', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
      submitBtn.removeAttribute('aria-busy');

      if (result.ok) {
        collapseCell();
        updateCellCount(containerId, +1);
        pulseCell(findCell(containerId));
      } else {
        errorEl.textContent = (result.data && result.data.error) || 'Failed to create item';
      }
    });

    container.appendChild(form);

    // Focus name input for convenience
    nameInput.focus();
  }

  // --- Section 6: Inline edit ---

  function renderInlineEdit(li, item, cell, container, containerId) {
    li._originalHTML = li.innerHTML;
    li.innerHTML = '';

    // Use a div, not a form, to avoid nested form issues
    const editDiv = document.createElement('div');
    editDiv.className = 'expanded-form';

    // Name input
    const nameLabel = document.createElement('label');
    nameLabel.textContent = 'Name';
    const nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.value = item.name;
    nameLabel.appendChild(nameInput);
    editDiv.appendChild(nameLabel);

    // Description textarea
    const descLabel = document.createElement('label');
    descLabel.textContent = 'Description';
    const descInput = document.createElement('textarea');
    descInput.value = item.description || '';
    descLabel.appendChild(descInput);
    editDiv.appendChild(descLabel);

    // Tags section
    const tagsLabel = document.createElement('label');
    tagsLabel.textContent = 'Tags';
    editDiv.appendChild(tagsLabel);

    const chipsContainer = document.createElement('div');
    chipsContainer.className = 'tag-chips-container';
    editDiv.appendChild(chipsContainer);

    const tagInput = document.createElement('input');
    tagInput.type = 'text';
    tagInput.placeholder = 'Type tag, press Enter';
    editDiv.appendChild(tagInput);

    // Error area
    const errorEl = document.createElement('small');
    errorEl.className = 'form-error';
    errorEl.setAttribute('aria-live', 'polite');
    editDiv.appendChild(errorEl);

    // Local copy of tags for live management
    let liveTags = (item.tags || []).slice();

    function renderEditTagChips() {
      chipsContainer.innerHTML = '';
      liveTags.forEach(function (tag) {
        const chip = document.createElement('kbd');
        chip.className = 'tag-chip';
        chip.textContent = tag;

        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.textContent = '\u00D7';
        removeBtn.addEventListener('click', async function (e) {
          e.stopPropagation();
          // Live remove tag via API
          const result = await apiCall('/api/items/' + item.id + '/tags/' + encodeURIComponent(tag), {
            method: 'DELETE'
          });
          if (result.ok) {
            liveTags = liveTags.filter(function (t) { return t !== tag; });
            renderEditTagChips();
          } else {
            errorEl.textContent = (result.data && result.data.error) || 'Failed to remove tag';
          }
        });

        chip.appendChild(removeBtn);
        chipsContainer.appendChild(chip);
      });
    }

    renderEditTagChips();

    // Add tag on Enter (live via API)
    tagInput.addEventListener('keydown', async function (e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        const val = tagInput.value.trim().toLowerCase();
        if (!val || liveTags.indexOf(val) !== -1) return;

        const result = await apiCall('/api/items/' + item.id + '/tags', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: val })
        });

        if (result.ok) {
          liveTags.push(val);
          renderEditTagChips();
          tagInput.value = '';
        } else {
          errorEl.textContent = (result.data && result.data.error) || 'Failed to add tag';
        }
      }
    });

    // Button row
    const formActions = document.createElement('div');
    formActions.className = 'form-actions';

    const saveBtn = document.createElement('button');
    saveBtn.type = 'button';
    saveBtn.textContent = 'Save';
    saveBtn.addEventListener('click', async function (e) {
      e.stopPropagation();
      errorEl.textContent = '';
      nameInput.removeAttribute('aria-invalid');

      const name = nameInput.value.trim();
      if (!name) {
        nameInput.setAttribute('aria-invalid', 'true');
        errorEl.textContent = 'Name is required';
        return;
      }

      const desc = descInput.value.trim() || null;

      saveBtn.setAttribute('aria-busy', 'true');
      const result = await apiCall('/api/items/' + item.id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: name,
          description: desc,
          container_id: containerId
        })
      });
      saveBtn.removeAttribute('aria-busy');

      if (result.ok) {
        // Re-fetch and re-render item list
        const listResult = await apiCall('/api/containers/' + containerId + '/items');
        if (listResult.ok && listResult.data && listResult.data.items) {
          renderItemList(cell, container, listResult.data.items, containerId);
        }
        pulseCell(cell);
      } else {
        errorEl.textContent = (result.data && result.data.error) || 'Failed to update item';
      }
    });
    formActions.appendChild(saveBtn);

    const discardBtn = document.createElement('button');
    discardBtn.type = 'button';
    discardBtn.className = 'secondary';
    discardBtn.textContent = 'Discard changes';
    discardBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      // Restore original li HTML (tags added/removed are already committed via API)
      li.innerHTML = li._originalHTML;
    });
    formActions.appendChild(discardBtn);

    editDiv.appendChild(formActions);
    li.appendChild(editDiv);
  }

  // --- Section 7: Delete with inline confirmation ---

  function handleDelete(btn, item, cell, container, containerId) {
    if (btn.dataset.confirm === 'true') {
      // Second click: execute delete
      if (btn._resetTimer) {
        clearTimeout(btn._resetTimer);
      }
      performDelete(item, containerId);
    } else {
      // First click: show confirmation
      btn.dataset.confirm = 'true';
      btn.textContent = 'Confirm?';
      btn.classList.add('btn-confirm');

      // Reset after 3 seconds
      btn._resetTimer = setTimeout(function () {
        btn.dataset.confirm = '';
        btn.textContent = 'Delete';
        btn.classList.remove('btn-confirm');
      }, 3000);
    }
  }

  async function performDelete(item, containerId) {
    const result = await apiCall('/api/items/' + item.id, {
      method: 'DELETE'
    });

    if (result.ok) {
      collapseCell();
      updateCellCount(containerId, -1);
      pulseCell(findCell(containerId));
    } else {
      const msg = (result.data && result.data.error) || 'Failed to delete item';
      alert(msg);
    }
  }

  // --- Section 8: Helper -- find cell by container ID ---

  function findCell(containerId) {
    return document.querySelector('.grid-cell[data-container-id="' + containerId + '"]');
  }

})();
