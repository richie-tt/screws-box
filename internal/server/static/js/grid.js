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
    // Close settings panel if open (mutual exclusion)
    if (settingsPanel) {
      closeSettingsPanel();
    }

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
    if (expandedPanel && !expandedPanel.contains(e.target) && !e.target.closest('.search-bar')) {
      collapseCell();
    }
  });

  // Escape key handler (search-aware: skips when search is active)
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
      var si = document.getElementById('search-input');
      var sd = document.getElementById('search-dropdown');
      if (si && (si === document.activeElement || (sd && sd.classList.contains('visible')))) return;
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

  // --- Section 4b: Tag Autocomplete Component ---

  function createAutocomplete(tagInput, onSelect, getExistingTags) {
    // Wrap tagInput in a relative container
    var wrapper = document.createElement('div');
    wrapper.className = 'autocomplete-wrapper';
    tagInput.parentNode.insertBefore(wrapper, tagInput);
    wrapper.appendChild(tagInput);

    // Create dropdown list
    var dropdown = document.createElement('ul');
    dropdown.className = 'autocomplete-dropdown';
    wrapper.appendChild(dropdown);

    var activeIndex = -1;
    var suggestions = [];
    var debounceTimer = null;

    function clearDropdown() {
      dropdown.classList.remove('visible');
      dropdown.innerHTML = '';
      suggestions = [];
      activeIndex = -1;
    }

    function renderSuggestions(tags) {
      dropdown.innerHTML = '';
      suggestions = tags;
      activeIndex = -1;
      tags.forEach(function (tag, i) {
        var li = document.createElement('li');
        li.textContent = tag;
        li.addEventListener('mousedown', function (e) {
          e.preventDefault(); // prevent blur before click registers
          onSelect(tag);
          clearDropdown();
          tagInput.value = '';
        });
        dropdown.appendChild(li);
      });
      // Trigger animation: force reflow then add visible class
      void dropdown.offsetHeight;
      dropdown.classList.add('visible');
    }

    function updateActive() {
      var items = dropdown.querySelectorAll('li');
      items.forEach(function (li, i) {
        if (i === activeIndex) {
          li.classList.add('active');
        } else {
          li.classList.remove('active');
        }
      });
    }

    function fetchSuggestions() {
      var val = tagInput.value.trim().toLowerCase();
      if (!val) {
        clearDropdown();
        return;
      }
      apiCall('/api/tags?q=' + encodeURIComponent(val)).then(function (result) {
        if (result.ok && Array.isArray(result.data)) {
          var existing = getExistingTags ? getExistingTags() : [];
          var names = result.data.map(function (t) {
            return typeof t === 'string' ? t : t.name;
          });
          var filtered = names.filter(function (t) {
            return existing.indexOf(t) === -1;
          }).slice(0, 5);
          if (filtered.length > 0) {
            renderSuggestions(filtered);
          } else {
            clearDropdown();
          }
        } else {
          clearDropdown();
        }
      });
    }

    // Input event: debounced fetch
    function onInput() {
      if (debounceTimer) clearTimeout(debounceTimer);
      debounceTimer = setTimeout(fetchSuggestions, 200);
    }
    tagInput.addEventListener('input', onInput);

    // Keydown: navigation and selection (added before existing handlers)
    function onKeydown(e) {
      if (e.key === 'ArrowDown') {
        if (suggestions.length > 0) {
          e.preventDefault();
          activeIndex = (activeIndex + 1) % suggestions.length;
          updateActive();
        }
      } else if (e.key === 'ArrowUp') {
        if (suggestions.length > 0) {
          e.preventDefault();
          activeIndex = activeIndex <= 0 ? suggestions.length - 1 : activeIndex - 1;
          updateActive();
        }
      } else if (e.key === 'Enter') {
        if (activeIndex >= 0 && suggestions.length > 0) {
          e.preventDefault();
          e.stopImmediatePropagation();
          onSelect(suggestions[activeIndex]);
          clearDropdown();
          tagInput.value = '';
        }
        // If activeIndex === -1, do nothing — let existing handler proceed
      } else if (e.key === 'Escape') {
        clearDropdown();
      }
    }
    tagInput.addEventListener('keydown', onKeydown, true);

    // Blur: delayed clear (allow click on suggestion)
    function onBlur() {
      setTimeout(function () {
        clearDropdown();
      }, 150);
    }
    tagInput.addEventListener('blur', onBlur);

    // Focus: re-fetch if input has value
    function onFocus() {
      if (tagInput.value.trim()) {
        fetchSuggestions();
      }
    }
    tagInput.addEventListener('focus', onFocus);

    return {
      destroy: function () {
        tagInput.removeEventListener('input', onInput);
        tagInput.removeEventListener('keydown', onKeydown, true);
        tagInput.removeEventListener('blur', onBlur);
        tagInput.removeEventListener('focus', onFocus);
        if (debounceTimer) clearTimeout(debounceTimer);
        clearDropdown();
      }
    };
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

    // Wire autocomplete to tag input (must be after tagInput is appended to form)
    createAutocomplete(tagInput, function (selectedTag) {
      if (pendingTags.indexOf(selectedTag) === -1) {
        pendingTags.push(selectedTag);
        renderTagChips();
        tagInput.value = '';
        updateSubmitState();
      } else {
        tagInput.value = '';
      }
    }, function () { return pendingTags; });

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

    // Wire autocomplete to edit tag input
    createAutocomplete(tagInput, async function (selectedTag) {
      if (liveTags.indexOf(selectedTag) !== -1) {
        tagInput.value = '';
        return;
      }
      var result = await apiCall('/api/items/' + item.id + '/tags', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: selectedTag })
      });
      if (result.ok) {
        liveTags.push(selectedTag);
        renderEditTagChips();
        tagInput.value = '';
      } else {
        errorEl.textContent = (result.data && result.data.error) || 'Failed to add tag';
      }
    }, function () { return liveTags; });

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

  // --- Section 8b: Settings Panel ---

  var settingsPanel = null;
  var settingsTrigger = document.querySelector('.settings-trigger');

  function closeSettingsPanel() {
    if (settingsPanel) {
      settingsPanel.remove();
      settingsPanel = null;
    }
    if (settingsTrigger) {
      settingsTrigger.classList.remove('active');
      settingsTrigger.focus();
    }
  }

  function openSettingsPanel() {
    // Close any open cell panel first (mutual exclusion)
    collapseCell();

    if (settingsPanel) {
      closeSettingsPanel();
      return;
    }

    if (!settingsTrigger) return;

    var shelfName = settingsTrigger.dataset.shelfName || '';
    var shelfRows = settingsTrigger.dataset.shelfRows || '5';
    var shelfCols = settingsTrigger.dataset.shelfCols || '10';

    settingsTrigger.classList.add('active');

    var panel = document.createElement('div');
    panel.className = 'expanded-panel settings-panel';
    panel.setAttribute('role', 'dialog');
    panel.setAttribute('aria-label', 'Grid settings');

    // Header — same pattern as cell panel
    var header = document.createElement('div');
    header.className = 'panel-header';
    var coordSpan = document.createElement('span');
    coordSpan.className = 'panel-coord';
    coordSpan.textContent = 'Settings';
    header.appendChild(coordSpan);
    var closeBtn = document.createElement('button');
    closeBtn.className = 'expanded-close';
    closeBtn.type = 'button';
    closeBtn.textContent = '\u00D7';
    closeBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      closeSettingsPanel();
    });
    header.appendChild(closeBtn);
    panel.appendChild(header);

    // Body — uses .expanded-form wrapper like cell panel forms
    var body = document.createElement('div');
    body.className = 'panel-body';

    var form = document.createElement('div');
    form.className = 'expanded-form';

    var nameLabel = document.createElement('label');
    nameLabel.setAttribute('for', 'settings-name');
    nameLabel.textContent = 'Shelf name';
    form.appendChild(nameLabel);
    var nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.id = 'settings-name';
    nameInput.value = shelfName;
    nameInput.maxLength = 100;
    form.appendChild(nameInput);

    var gridInputs = document.createElement('div');
    gridInputs.className = 'settings-grid-inputs';

    var colDiv = document.createElement('div');
    var colLabel = document.createElement('label');
    colLabel.setAttribute('for', 'settings-cols');
    colLabel.textContent = 'Columns';
    colDiv.appendChild(colLabel);
    var colInput = document.createElement('input');
    colInput.type = 'number';
    colInput.id = 'settings-cols';
    colInput.min = '1';
    colInput.max = '30';
    colInput.step = '1';
    colInput.value = shelfCols;
    colDiv.appendChild(colInput);
    gridInputs.appendChild(colDiv);

    var rowDiv = document.createElement('div');
    var rowLabel = document.createElement('label');
    rowLabel.setAttribute('for', 'settings-rows');
    rowLabel.textContent = 'Rows';
    rowDiv.appendChild(rowLabel);
    var rowInput = document.createElement('input');
    rowInput.type = 'number';
    rowInput.id = 'settings-rows';
    rowInput.min = '1';
    rowInput.max = '26';
    rowInput.step = '1';
    rowInput.value = shelfRows;
    rowDiv.appendChild(rowInput);
    gridInputs.appendChild(rowDiv);

    form.appendChild(gridInputs);

    var errorEl = document.createElement('div');
    errorEl.className = 'form-error';
    errorEl.id = 'settings-error';
    errorEl.hidden = true;
    form.appendChild(errorEl);

    var actions = document.createElement('div');
    actions.className = 'form-actions';

    var cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.className = 'secondary';
    cancelBtn.id = 'settings-cancel';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      closeSettingsPanel();
    });
    actions.appendChild(cancelBtn);

    var saveBtn = document.createElement('button');
    saveBtn.type = 'button';
    saveBtn.id = 'settings-save';
    saveBtn.textContent = 'Save';
    saveBtn.addEventListener('click', async function (e) {
      e.stopPropagation();

      // Clear previous errors
      errorEl.hidden = true;
      errorEl.textContent = '';
      rowInput.removeAttribute('aria-invalid');
      colInput.removeAttribute('aria-invalid');

      var rows = parseInt(rowInput.value, 10);
      var cols = parseInt(colInput.value, 10);
      var name = nameInput.value.trim();

      // Client-side validation
      if (isNaN(rows) || rows < 1 || rows > 26) {
        rowInput.setAttribute('aria-invalid', 'true');
        errorEl.textContent = '1-26 rows allowed (A-Z)';
        errorEl.hidden = false;
        rowInput.focus();
        return;
      }
      if (isNaN(cols) || cols < 1 || cols > 30) {
        colInput.setAttribute('aria-invalid', 'true');
        errorEl.textContent = '1-30 columns allowed';
        errorEl.hidden = false;
        colInput.focus();
        return;
      }

      // Set busy state
      saveBtn.setAttribute('aria-busy', 'true');
      saveBtn.style.opacity = '0.7';
      saveBtn.disabled = true;
      cancelBtn.disabled = true;

      var payload = { rows: rows, cols: cols, name: name };

      try {
        var resp = await fetch('/api/shelf/resize', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        var data = await resp.json().catch(function () { return null; });

        if (resp.status === 200) {
          window.location.reload();
          return;
        }

        if (resp.status === 409 && data && data.affected) {
          closeSettingsPanel();
          showResizeBlockedModal(data.affected);
          return;
        }

        // 400 or other error
        errorEl.textContent = (data && (data.error || data.message)) || 'An error occurred';
        errorEl.hidden = false;
      } catch (err) {
        errorEl.textContent = 'An error occurred';
        errorEl.hidden = false;
      }

      // Re-enable buttons
      saveBtn.removeAttribute('aria-busy');
      saveBtn.style.opacity = '';
      saveBtn.disabled = false;
      cancelBtn.disabled = false;
    });
    actions.appendChild(saveBtn);

    form.appendChild(actions);
    body.appendChild(form);
    panel.appendChild(body);

    document.body.appendChild(panel);
    settingsPanel = panel;

    // Position panel below the gear icon, right-aligned
    var rect = settingsTrigger.getBoundingClientRect();
    var panelWidth = 280;
    var topPos = rect.bottom + 8;
    var leftPos = rect.right - panelWidth;
    if (leftPos < 8) leftPos = 8;
    if (topPos + 300 > window.innerHeight) {
      topPos = Math.max(8, rect.top - 300 - 8);
    }
    panel.style.top = topPos + 'px';
    panel.style.left = leftPos + 'px';

    nameInput.focus();
  }

  function showResizeBlockedModal(affected) {
    var backdrop = document.createElement('div');
    backdrop.className = 'resize-modal-backdrop';

    var modal = document.createElement('div');
    modal.className = 'resize-modal';
    modal.setAttribute('role', 'alertdialog');
    modal.setAttribute('aria-labelledby', 'resize-modal-title');

    // Header
    var mHeader = document.createElement('div');
    mHeader.className = 'resize-modal-header';
    var h3 = document.createElement('h3');
    h3.id = 'resize-modal-title';
    h3.textContent = 'Cannot Resize';
    mHeader.appendChild(h3);
    modal.appendChild(mHeader);

    // Body
    var mBody = document.createElement('div');
    mBody.className = 'resize-modal-body';
    var desc = document.createElement('p');
    desc.textContent = 'The following containers have items. Move or delete these items before resizing.';
    mBody.appendChild(desc);

    var ul = document.createElement('ul');
    ul.className = 'resize-blocked-list';
    affected.forEach(function (container) {
      var li = document.createElement('li');

      var badge = document.createElement('span');
      badge.className = 'position-badge';
      badge.textContent = container.label;
      li.appendChild(badge);

      var countSpan = document.createElement('span');
      countSpan.className = 'blocked-item-count';
      countSpan.textContent = container.item_count + (container.item_count === 1 ? ' item' : ' items');
      li.appendChild(countSpan);

      if (container.items && container.items.length > 0) {
        var itemUl = document.createElement('ul');
        itemUl.className = 'blocked-item-names';
        container.items.forEach(function (itemName) {
          var itemLi = document.createElement('li');
          itemLi.textContent = itemName;
          itemUl.appendChild(itemLi);
        });
        li.appendChild(itemUl);
      }

      ul.appendChild(li);
    });
    mBody.appendChild(ul);
    modal.appendChild(mBody);

    // Footer
    var mFooter = document.createElement('div');
    mFooter.className = 'resize-modal-footer';
    var okBtn = document.createElement('button');
    okBtn.id = 'resize-modal-ok';
    okBtn.className = 'secondary';
    okBtn.textContent = 'Back to Grid';
    okBtn.addEventListener('click', function () {
      closeResizeModal();
    });
    mFooter.appendChild(okBtn);
    modal.appendChild(mFooter);

    backdrop.appendChild(modal);
    document.body.appendChild(backdrop);

    // Focus the OK button
    okBtn.focus();

    // Close on backdrop click
    backdrop.addEventListener('click', function (e) {
      if (e.target === backdrop) {
        closeResizeModal();
      }
    });

    // Close on Escape
    function onEsc(e) {
      if (e.key === 'Escape') {
        closeResizeModal();
        document.removeEventListener('keydown', onEsc);
      }
    }
    document.addEventListener('keydown', onEsc);

    function closeResizeModal() {
      backdrop.remove();
      if (settingsTrigger) settingsTrigger.focus();
    }
  }

  // Wire gear button click
  if (settingsTrigger) {
    settingsTrigger.addEventListener('click', function (e) {
      e.stopPropagation();
      openSettingsPanel();
    });
  }

  // Close settings panel on click outside
  document.addEventListener('click', function (e) {
    if (settingsPanel && !settingsPanel.contains(e.target) && e.target !== settingsTrigger && !settingsTrigger.contains(e.target)) {
      closeSettingsPanel();
    }
  });

  // Close settings panel on Escape (integrated with existing handler)
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && settingsPanel) {
      closeSettingsPanel();
      e.stopPropagation();
    }
  });

  // --- Section 9: Search ---

  var searchInput = document.getElementById('search-input');
  var searchDropdown = document.getElementById('search-dropdown');
  var searchListbox = document.getElementById('search-results-listbox');

  if (!searchInput || !searchDropdown || !searchListbox) return;

  var dropdownHeader = searchDropdown.querySelector('.search-dropdown-header');
  var dropdownEmpty = searchDropdown.querySelector('.search-dropdown-empty');
  var dropdownError = searchDropdown.querySelector('.search-dropdown-error');
  var clearBtn = document.querySelector('.search-clear-btn');
  var spinner = document.querySelector('.search-spinner');

  var searchController = null;
  var debounceTimer = null;
  var currentResults = [];
  var focusedIndex = -1;
  var lastQuery = '';

  // 9b. Helper: highlightMatch
  function highlightMatch(text, query) {
    var lower = text.toLowerCase();
    var idx = lower.indexOf(query.toLowerCase());
    if (idx === -1) return document.createTextNode(text);

    var frag = document.createDocumentFragment();
    if (idx > 0) frag.appendChild(document.createTextNode(text.slice(0, idx)));
    var strong = document.createElement('strong');
    strong.textContent = text.slice(idx, idx + query.length);
    frag.appendChild(strong);
    if (idx + query.length < text.length) {
      frag.appendChild(document.createTextNode(text.slice(idx + query.length)));
    }
    return frag;
  }

  // 9c. Helper: showDropdown / hideDropdown
  function showDropdown() {
    searchDropdown.removeAttribute('hidden');
    searchDropdown.classList.add('visible');
    searchInput.setAttribute('aria-expanded', 'true');
  }

  function hideDropdown() {
    searchDropdown.setAttribute('hidden', '');
    searchDropdown.classList.remove('visible');
    searchInput.setAttribute('aria-expanded', 'false');
    focusedIndex = -1;
    searchInput.setAttribute('aria-activedescendant', '');
  }

  // 9d. Helper: showSpinner / hideSpinner
  function showSpinner() {
    if (spinner) spinner.removeAttribute('hidden');
  }

  function hideSpinner() {
    if (spinner) spinner.setAttribute('hidden', '');
  }

  // 9e. Helper: updateClearButton
  function updateClearButton() {
    if (!clearBtn) return;
    if (searchInput.value.length > 0) {
      clearBtn.removeAttribute('hidden');
    } else {
      clearBtn.setAttribute('hidden', '');
    }
  }

  // 9f. Core: renderResults
  function renderResults(results, query) {
    currentResults = results;
    searchListbox.innerHTML = '';

    if (results.length === 0) {
      dropdownEmpty.textContent = "No results for '" + query + "' -- try fewer characters or a different tag";
      dropdownEmpty.removeAttribute('hidden');
      dropdownHeader.setAttribute('hidden', '');
      searchListbox.setAttribute('hidden', '');
      dropdownError.setAttribute('hidden', '');
      updateGridHighlights([], null);
      showDropdown();
      return;
    }

    dropdownEmpty.setAttribute('hidden', '');
    dropdownError.setAttribute('hidden', '');
    dropdownHeader.textContent = results.length + (results.length === 1 ? ' result' : ' results');
    dropdownHeader.removeAttribute('hidden');
    searchListbox.removeAttribute('hidden');

    results.forEach(function (result, index) {
      var li = document.createElement('li');
      li.setAttribute('role', 'option');
      li.setAttribute('id', 'search-result-' + index);
      li.setAttribute('aria-selected', 'false');
      li.className = 'search-result';

      // Top row: name + position badge
      var topDiv = document.createElement('div');
      topDiv.className = 'search-result-top';

      var nameSpan = document.createElement('span');
      nameSpan.className = 'search-result-name';
      nameSpan.appendChild(highlightMatch(result.name, query));
      topDiv.appendChild(nameSpan);

      var badge = document.createElement('span');
      badge.className = 'position-badge';
      badge.textContent = result.container_label;
      topDiv.appendChild(badge);

      li.appendChild(topDiv);

      // Tags row
      if (result.tags && result.tags.length > 0) {
        var tagsDiv = document.createElement('div');
        tagsDiv.className = 'search-result-tags';
        result.tags.forEach(function (tag) {
          var chip = document.createElement('span');
          chip.className = 'tag-chip';
          if (tag.toLowerCase() === query.toLowerCase()) {
            var s = document.createElement('strong');
            s.textContent = tag;
            chip.appendChild(s);
          } else {
            chip.appendChild(highlightMatch(tag, query));
          }
          tagsDiv.appendChild(chip);
        });
        li.appendChild(tagsDiv);
      }

      li.addEventListener('click', function (e) {
        e.stopPropagation();
        selectResult(index);
      });

      searchListbox.appendChild(li);
    });

    showDropdown();
    focusedIndex = -1;
    updateGridHighlights(results, null);

    // Auto-scroll grid to first highlighted cell (D-26)
    if (results.length > 0) {
      var firstCell = findCell(results[0].container_id);
      if (firstCell) {
        firstCell.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    }
  }

  // 9g. Core: updateGridHighlights
  function updateGridHighlights(results, focusedContainerId) {
    var matchCounts = {};
    results.forEach(function (item) {
      matchCounts[item.container_id] = (matchCounts[item.container_id] || 0) + 1;
    });

    var hasResults = Object.keys(matchCounts).length > 0;
    gridContainer.classList.toggle('search-active', hasResults);

    var cells = document.querySelectorAll('.grid-cell');
    cells.forEach(function (cell) {
      var cid = parseInt(cell.dataset.containerId, 10);
      var count = matchCounts[cid] || 0;

      cell.classList.toggle('highlight', count > 0);
      cell.classList.toggle('highlight-focus', cid === focusedContainerId);

      // Match count badge (D-22)
      var badge = cell.querySelector('.match-count');
      if (count > 1) {
        if (!badge) {
          badge = document.createElement('span');
          badge.className = 'match-count';
          cell.appendChild(badge);
        }
        badge.textContent = count;
      } else if (badge) {
        badge.remove();
      }
    });
  }

  // 9h. Core: clearSearch
  function clearSearch() {
    searchInput.value = '';
    hideDropdown();

    // Clear all grid highlights
    gridContainer.classList.remove('search-active');
    var cells = document.querySelectorAll('.grid-cell');
    cells.forEach(function (cell) {
      cell.classList.remove('highlight');
      cell.classList.remove('highlight-focus');
      var badge = cell.querySelector('.match-count');
      if (badge) badge.remove();
    });

    currentResults = [];
    focusedIndex = -1;
    lastQuery = '';

    updateClearButton();

    if (searchController) {
      searchController.abort();
      searchController = null;
    }
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  }

  // 9i. Core: performSearch
  function performSearch(query) {
    if (searchController) searchController.abort();
    searchController = new AbortController();

    showSpinner();

    fetch('/api/search?q=' + encodeURIComponent(query), { signal: searchController.signal })
      .then(function (resp) { return resp.json(); })
      .then(function (data) {
        hideSpinner();
        searchController = null;

        // Race condition guard (Pitfall 1): check input still matches
        if (searchInput.value.trim() !== query) return;

        renderResults(data.results || [], query);
        lastQuery = query;
      })
      .catch(function (err) {
        if (err.name === 'AbortError') return;
        hideSpinner();
        searchController = null;

        // Show error state (D-13)
        dropdownHeader.setAttribute('hidden', '');
        searchListbox.setAttribute('hidden', '');
        dropdownEmpty.setAttribute('hidden', '');
        dropdownError.textContent = 'Search failed -- check your connection and try again';
        dropdownError.removeAttribute('hidden');
        showDropdown();

        // Clear grid highlights on error
        updateGridHighlights([], null);
      });
  }

  // 9j. Core: selectResult
  function selectResult(index) {
    var result = currentResults[index];
    if (!result) return;

    hideDropdown();

    var cell = findCell(result.container_id);
    if (cell) {
      cell.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      expandCell(cell);
      pulseCell(cell);
    }
  }

  // 9k. Event: searchInput 'input' handler
  searchInput.addEventListener('input', function () {
    updateClearButton();

    if (debounceTimer) clearTimeout(debounceTimer);

    var query = searchInput.value.trim();

    if (query.length < 2) {
      hideDropdown();
      hideSpinner();
      // Clear grid highlights
      gridContainer.classList.remove('search-active');
      var cells = document.querySelectorAll('.grid-cell');
      cells.forEach(function (cell) {
        cell.classList.remove('highlight');
        cell.classList.remove('highlight-focus');
        var badge = cell.querySelector('.match-count');
        if (badge) badge.remove();
      });
      // Abort any in-flight request
      if (searchController) {
        searchController.abort();
        searchController = null;
      }
      return;
    }

    // Close any open panel before entering search mode (D-24)
    if (expandedPanel) collapseCell();

    showSpinner();
    debounceTimer = setTimeout(function () {
      performSearch(query);
    }, 200);
  });

  // 9l. Event: searchInput 'keydown' handler
  searchInput.addEventListener('keydown', function (e) {
    var dropdownVisible = searchDropdown.classList.contains('visible');

    if (e.key === 'ArrowDown') {
      if (dropdownVisible && currentResults.length > 0) {
        e.preventDefault();
        if (focusedIndex < currentResults.length - 1) {
          focusedIndex++;
        } else {
          focusedIndex = 0;
        }
        updateFocusedResult();
      }
    } else if (e.key === 'ArrowUp') {
      if (dropdownVisible && focusedIndex >= 0) {
        e.preventDefault();
        if (focusedIndex > 0) {
          focusedIndex--;
        } else {
          focusedIndex = -1;
        }
        updateFocusedResult();
      }
    } else if (e.key === 'Enter') {
      if (focusedIndex >= 0) {
        e.preventDefault();
        selectResult(focusedIndex);
      }
    } else if (e.key === 'Escape') {
      if (focusedIndex >= 0) {
        // Layer 1: return focus from results to input
        focusedIndex = -1;
        updateFocusedResult();
        searchInput.focus();
        e.preventDefault();
        e.stopPropagation();
      } else if (dropdownVisible) {
        // Layer 2: close dropdown and clear highlights
        hideDropdown();
        gridContainer.classList.remove('search-active');
        var cells = document.querySelectorAll('.grid-cell');
        cells.forEach(function (cell) {
          cell.classList.remove('highlight');
          cell.classList.remove('highlight-focus');
          var badge = cell.querySelector('.match-count');
          if (badge) badge.remove();
        });
        e.preventDefault();
        e.stopPropagation();
      } else if (searchInput.value.length > 0) {
        // Layer 3: clear search text entirely
        clearSearch();
        e.preventDefault();
        e.stopPropagation();
      }
    } else if (e.key === 'Home') {
      if (focusedIndex >= 0) {
        e.preventDefault();
        focusedIndex = 0;
        updateFocusedResult();
      }
    } else if (e.key === 'End') {
      if (focusedIndex >= 0) {
        e.preventDefault();
        focusedIndex = currentResults.length - 1;
        updateFocusedResult();
      }
    }
  });

  // 9m. Helper: updateFocusedResult
  function updateFocusedResult() {
    var items = searchListbox.querySelectorAll('li[role="option"]');
    items.forEach(function (li, i) {
      li.setAttribute('aria-selected', i === focusedIndex ? 'true' : 'false');
    });

    searchInput.setAttribute('aria-activedescendant', focusedIndex >= 0 ? 'search-result-' + focusedIndex : '');

    if (focusedIndex >= 0 && items[focusedIndex]) {
      items[focusedIndex].scrollIntoView({ block: 'nearest' });
    }

    // Update grid highlight-focus (D-25)
    updateGridHighlights(currentResults, focusedIndex >= 0 ? currentResults[focusedIndex].container_id : null);
  }

  // 9n. Event: clearBtn 'click' handler
  if (clearBtn) {
    clearBtn.addEventListener('click', function (e) {
      e.preventDefault();
      e.stopPropagation();
      clearSearch();
      searchInput.focus();
    });
  }

  // 9o. Event: document 'keydown' for / shortcut (D-18)
  document.addEventListener('keydown', function (e) {
    if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey) {
      var tag = document.activeElement.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if (document.activeElement.isContentEditable) return;
      e.preventDefault();
      searchInput.focus();
    }
  });

  // 9p. Event: click outside search area to close dropdown (keep highlights per D-27)
  document.addEventListener('click', function (e) {
    if (searchDropdown.classList.contains('visible') && !e.target.closest('.search-sidebar')) {
      hideDropdown();
    }
  });

})();
