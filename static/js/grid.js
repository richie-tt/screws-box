// grid.js -- Inline cell expansion CRUD logic
// Phase 06 Plan 02: Complete rewrite -- inline expansion, no dialog
(function () {
  'use strict';

  // --- Section 1: State and DOM references ---

  const gridContainer = document.querySelector('.grid-container');
  if (!gridContainer) return;

  let expandedCell = null;   // the grid cell that was clicked
  let expandedPanel = null;  // the floating overlay panel element

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
    // Do not re-expand an already active cell
    if (cell.classList.contains('cell-active')) return;
    // Stop propagation so the click-outside handler doesn't immediately close the panel
    e.stopPropagation();
    expandCell(cell);
  });

  async function expandCell(cell) {
    // Collapse any previously expanded panel first
    if (expandedCell) {
      collapseCell();
    }

    // Highlight the source cell
    cell.classList.add('cell-active');
    expandedCell = cell;

    // Create a floating panel outside the grid
    const panel = document.createElement('div');
    panel.className = 'expanded-panel';
    document.body.appendChild(panel);
    expandedPanel = panel;

    // Position panel near the clicked cell
    const rect = cell.getBoundingClientRect();
    const panelWidth = 300;
    let left = rect.right + 12;
    // If it overflows the right edge, place it to the left of the cell
    if (left + panelWidth > window.innerWidth) {
      left = rect.left - panelWidth - 12;
    }
    // If still off-screen, center horizontally
    if (left < 8) {
      left = Math.max(8, (window.innerWidth - panelWidth) / 2);
    }
    // Clamp right edge
    if (left + panelWidth > window.innerWidth - 8) {
      left = window.innerWidth - panelWidth - 8;
    }
    let top = rect.top;
    // Keep within viewport vertically
    if (top + 300 > window.innerHeight) {
      top = Math.max(8, window.innerHeight - 400);
    }
    panel.style.top = top + 'px';
    panel.style.left = left + 'px';

    // Panel header bar
    const headerBar = document.createElement('div');
    headerBar.className = 'panel-header';

    const coordWrap = document.createElement('div');
    const coordLabel = document.createElement('span');
    coordLabel.className = 'panel-coord';
    coordLabel.textContent = cell.dataset.coord;
    coordWrap.appendChild(coordLabel);

    const countLabel = document.createElement('span');
    countLabel.className = 'panel-count';
    const cnt = parseInt(cell.dataset.count, 10) || 0;
    countLabel.textContent = cnt === 0 ? 'Empty' : formatCount(cnt);
    coordWrap.appendChild(countLabel);
    headerBar.appendChild(coordWrap);

    const closeBtn = document.createElement('button');
    closeBtn.className = 'expanded-close';
    closeBtn.type = 'button';
    closeBtn.textContent = '\u00D7';
    closeBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      collapseCell();
    });
    headerBar.appendChild(closeBtn);
    panel.appendChild(headerBar);

    // Content container
    const content = document.createElement('div');
    content.className = 'panel-body';
    panel.appendChild(content);

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
    if (expandedPanel) {
      expandedPanel.remove();
      expandedPanel = null;
    }
    if (expandedCell) {
      expandedCell.classList.remove('cell-active');
      expandedCell = null;
    }
  }

  // Click-outside handler
  document.addEventListener('click', function (e) {
    if (expandedPanel && !expandedPanel.contains(e.target)) {
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
      li.className = 'item-card';

      // Top row: name + action icons
      const topRow = document.createElement('div');
      topRow.className = 'item-card-top';

      const nameSpan = document.createElement('span');
      nameSpan.className = 'item-name';
      nameSpan.textContent = item.name;
      topRow.appendChild(nameSpan);

      // Compact action buttons (reveal on hover)
      const actions = document.createElement('div');
      actions.className = 'item-actions';

      const editBtn = document.createElement('button');
      editBtn.type = 'button';
      editBtn.textContent = '\u270E'; // pencil
      editBtn.title = 'Edit';
      editBtn.addEventListener('click', function (e) {
        e.stopPropagation();
        renderInlineEdit(li, item, cell, container, containerId);
      });
      actions.appendChild(editBtn);

      const deleteBtn = document.createElement('button');
      deleteBtn.type = 'button';
      deleteBtn.textContent = '\u2715'; // small x
      deleteBtn.title = 'Delete';
      deleteBtn.addEventListener('click', function (e) {
        e.stopPropagation();
        handleDelete(deleteBtn, item, cell, container, containerId);
      });
      actions.appendChild(deleteBtn);

      topRow.appendChild(actions);
      li.appendChild(topRow);

      // Tags row below name
      if (item.tags && item.tags.length > 0) {
        const tagsRow = document.createElement('div');
        tagsRow.className = 'item-tags-row';
        item.tags.forEach(function (tag) {
          const chip = document.createElement('span');
          chip.className = 'tag-chip';
          chip.textContent = tag;
          tagsRow.appendChild(chip);
        });
        li.appendChild(tagsRow);
      }

      ul.appendChild(li);
    });

    container.appendChild(ul);

    // Dashed "+ Add item" button
    const addBtn = document.createElement('button');
    addBtn.type = 'button';
    addBtn.className = 'add-item-btn';
    addBtn.innerHTML = '+ Add item';
    addBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      renderAddForm(cell, container, containerId);
    });
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
    submitBtn.classList.add('btn-disabled');
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
      if (enabled) {
        submitBtn.classList.remove('btn-disabled');
      } else {
        submitBtn.classList.add('btn-disabled');
      }
      // Show tag hint when name is filled but no tags added yet
      if (hasName && !hasTags) {
        tagHint.hidden = false;
      } else {
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

    nameInput.addEventListener('input', updateSubmitState);

    // Form submit
    form.addEventListener('submit', async function (e) {
      e.preventDefault();
      if (submitBtn.classList.contains('btn-disabled')) return;

      errorEl.textContent = '';
      nameInput.removeAttribute('aria-invalid');

      const name = nameInput.value.trim();
      if (!name) {
        nameInput.setAttribute('aria-invalid', 'true');
        errorEl.textContent = 'Name is required';
        nameInput.focus();
        return;
      }

      if (pendingTags.length === 0) {
        tagHint.hidden = false;
        tagInput.focus();
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
