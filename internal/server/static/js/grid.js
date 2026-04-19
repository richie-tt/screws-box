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

  function getCSRFToken() {
    var match = document.cookie.match(/(?:^|; )screwsbox_csrf=([^;]*)/);
    return match ? match[1] : '';
  }

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
      if (options && options.method && options.method !== 'GET') {
        options.headers = Object.assign({ 'X-CSRF-Token': getCSRFToken() }, options.headers);
      }
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
    var cell = e.target.closest('.grid-cell');
    if (!cell) return;
    e.stopPropagation();

    // D-07: Toggle if same cell clicked
    if (expandedCell === cell) {
      collapseCell();
      return;
    }

    // D-06: If panel already open, swap content directly (no animation)
    if (expandedPanel && expandedCell) {
      swapPanelContent(cell);
      return;
    }

    // First open
    expandCell(cell);
  });

  async function expandCell(cell) {
    // Collapse any existing panel first (safety)
    if (expandedCell) {
      collapseCell();
    }

    cell.classList.add('cell-active');
    cell.setAttribute('aria-expanded', 'true');
    expandedCell = cell;

    var sidebar = document.querySelector('.search-sidebar');

    var panel = document.createElement('div');
    panel.className = 'expanded-panel';
    panel.setAttribute('role', 'region');
    panel.setAttribute('aria-label', 'Container details');
    sidebar.appendChild(panel);
    expandedPanel = panel;

    // Build header
    var headerBar = document.createElement('div');
    headerBar.className = 'panel-header';

    var coordWrap = document.createElement('div');
    var coordLabel = document.createElement('span');
    coordLabel.className = 'panel-coord';
    coordLabel.textContent = cell.dataset.coord;
    coordWrap.appendChild(coordLabel);

    var countLabel = document.createElement('span');
    countLabel.className = 'panel-count';
    var cnt = parseInt(cell.dataset.count, 10) || 0;
    countLabel.textContent = cnt === 0 ? 'Empty' : formatCount(cnt);
    coordWrap.appendChild(countLabel);
    headerBar.appendChild(coordWrap);

    var closeBtn = document.createElement('button');
    closeBtn.className = 'expanded-close';
    closeBtn.type = 'button';
    closeBtn.textContent = '\u00D7';
    closeBtn.setAttribute('aria-label', 'Close panel');
    closeBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      collapseCell();
    });
    headerBar.appendChild(closeBtn);
    panel.appendChild(headerBar);

    var content = document.createElement('div');
    content.className = 'panel-body';
    panel.appendChild(content);

    // D-08: Trigger slide-down animation via requestAnimationFrame
    requestAnimationFrame(function () {
      panel.classList.add('panel-open');
    });

    // Fetch items
    var containerId = parseInt(cell.dataset.containerId, 10);
    var result = await apiCall('/api/containers/' + containerId + '/items');

    if (result.ok && result.data && result.data.items && result.data.items.length > 0) {
      renderItemList(cell, content, result.data.items, containerId);
    } else {
      renderAddForm(cell, content, containerId);
    }
  }

  function collapseCell() {
    if (!expandedPanel) return;

    var panel = expandedPanel;
    var cell = expandedCell;

    panel.classList.remove('panel-open');

    function onTransitionEnd(e) {
      if (e.propertyName !== 'max-height') return;
      panel.removeEventListener('transitionend', onTransitionEnd);
      panel.remove();
    }
    panel.addEventListener('transitionend', onTransitionEnd);

    // Fallback: if reduced-motion or transition doesn't fire, remove after 250ms
    setTimeout(function () {
      if (panel.parentNode) {
        panel.removeEventListener('transitionend', onTransitionEnd);
        panel.remove();
      }
    }, 250);

    expandedPanel = null;
    if (cell) {
      cell.setAttribute('aria-expanded', 'false');
      cell.classList.remove('cell-active');
      expandedCell = null;
    }
  }

  async function swapPanelContent(cell) {
    // Update cell highlighting
    if (expandedCell) {
      expandedCell.setAttribute('aria-expanded', 'false');
      expandedCell.classList.remove('cell-active');
    }
    cell.classList.add('cell-active');
    cell.setAttribute('aria-expanded', 'true');
    expandedCell = cell;

    // Update header
    var coordLabel = expandedPanel.querySelector('.panel-coord');
    var countLabel = expandedPanel.querySelector('.panel-count');
    coordLabel.textContent = cell.dataset.coord;
    var cnt = parseInt(cell.dataset.count, 10) || 0;
    countLabel.textContent = cnt === 0 ? 'Empty' : formatCount(cnt);

    // Clear body and refetch content
    var content = expandedPanel.querySelector('.panel-body');
    while (content.firstChild) { content.removeChild(content.firstChild); }

    var containerId = parseInt(cell.dataset.containerId, 10);
    var result = await apiCall('/api/containers/' + containerId + '/items');

    if (result.ok && result.data && result.data.items && result.data.items.length > 0) {
      renderItemList(cell, content, result.data.items, containerId);
    } else {
      renderAddForm(cell, content, containerId);
    }
  }

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

  // --- Section 9: Unified Search ---

  var searchInput = document.getElementById('search-input');
  var searchDropdown = document.getElementById('search-dropdown');
  var searchListbox = document.getElementById('search-unified-listbox');

  if (!searchInput || !searchDropdown || !searchListbox) return;

  var dropdownTagsSection = document.getElementById('search-dropdown-tags');
  var tagSuggestList = document.getElementById('tag-suggest-list');
  var dropdownItemsSection = document.getElementById('search-dropdown-items');
  var dropdownDivider = document.getElementById('search-dropdown-divider');
  var searchItemsHeader = document.getElementById('search-items-header');
  var dropdownEmpty = document.getElementById('search-dropdown-empty');
  var dropdownError = document.getElementById('search-dropdown-error');
  var filterChipsContainer = document.getElementById('search-filter-chips');
  var badgeCount = document.getElementById('search-badge-count');
  var clearBtn = document.querySelector('.search-clear-btn');
  var spinner = document.querySelector('.search-spinner');

  // 9a. State variables
  var activeFilterTags = [];
  var searchAbort = null;
  var tagAbort = null;
  var debounceTimer = null;
  var pushStateTimer = null;
  var currentResults = [];
  var currentTagSuggestions = [];
  var focusedIndex = -1;
  var totalFocusableCount = 0;
  var lastQuery = '';

  // 9b. Helper: highlightMatch (D-19: use <mark> instead of <strong>)
  function highlightMatch(text, query) {
    if (!query) return document.createTextNode(text);
    var lower = text.toLowerCase();
    var idx = lower.indexOf(query.toLowerCase());
    if (idx === -1) return document.createTextNode(text);

    var frag = document.createDocumentFragment();
    if (idx > 0) frag.appendChild(document.createTextNode(text.slice(0, idx)));
    var mark = document.createElement('mark');
    mark.textContent = text.slice(idx, idx + query.length);
    frag.appendChild(mark);
    if (idx + query.length < text.length) {
      frag.appendChild(document.createTextNode(text.slice(idx + query.length)));
    }
    return frag;
  }

  // 9c. Helper: showDropdown / hideDropdown
  function showDropdown() {
    searchDropdown.removeAttribute('hidden');
    void searchDropdown.offsetHeight;
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
    if (searchInput.value.length > 0 || activeFilterTags.length > 0) {
      clearBtn.removeAttribute('hidden');
    } else {
      clearBtn.setAttribute('hidden', '');
    }
  }

  // 9f. Helper: updateBadgeCount (D-06)
  function updateBadgeCount() {
    if (!badgeCount) return;
    if (activeFilterTags.length > 0) {
      badgeCount.textContent = activeFilterTags.length === 1 ? '1 tag' : activeFilterTags.length + ' tags';
      badgeCount.removeAttribute('hidden');
    } else {
      badgeCount.setAttribute('hidden', '');
    }
  }

  // 9g. Filter chip management (D-03, D-04, D-07)
  function renderFilterChips() {
    if (!filterChipsContainer) return;
    while (filterChipsContainer.firstChild) {
      filterChipsContainer.removeChild(filterChipsContainer.firstChild);
    }

    if (activeFilterTags.length === 0) {
      filterChipsContainer.setAttribute('hidden', '');
      updateBadgeCount();
      return;
    }

    activeFilterTags.forEach(function (tagName) {
      var chip = document.createElement('span');
      chip.className = 'tag-filter-chip tag-chip';
      chip.appendChild(document.createTextNode(tagName));

      var removeBtn = document.createElement('button');
      removeBtn.type = 'button';
      removeBtn.setAttribute('aria-label', 'Remove tag ' + tagName);
      removeBtn.textContent = '\u00D7';
      removeBtn.addEventListener('click', function (e) {
        e.stopPropagation();
        removeFilterTag(tagName);
      });
      chip.appendChild(removeBtn);
      filterChipsContainer.appendChild(chip);
    });

    // Clear All button (D-07)
    var clearAllBtn = document.createElement('button');
    clearAllBtn.type = 'button';
    clearAllBtn.className = 'search-clear-all-btn';
    clearAllBtn.setAttribute('aria-label', 'Clear all tag filters');
    clearAllBtn.textContent = 'Clear All';
    clearAllBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      clearAllFilters();
    });
    filterChipsContainer.appendChild(clearAllBtn);

    filterChipsContainer.removeAttribute('hidden');
    updateBadgeCount();
  }

  function addFilterTag(tagName) {
    if (activeFilterTags.indexOf(tagName) !== -1) return;
    activeFilterTags.push(tagName);
    renderFilterChips();
    searchInput.value = '';
    updateClearButton();
    // Immediate search with new filter (D-12: 0ms for tag changes)
    performUnifiedSearch('');
    // pushState immediately (discrete action)
    updateSearchURL('', activeFilterTags, true);
  }

  function removeFilterTag(tagName) {
    activeFilterTags = activeFilterTags.filter(function (t) { return t !== tagName; });
    renderFilterChips();
    updateClearButton();
    var query = searchInput.value.trim();
    performUnifiedSearch(query);
    updateSearchURL(query, activeFilterTags, true);
  }

  function clearAllFilters() {
    activeFilterTags = [];
    renderFilterChips();
    updateClearButton();
    var query = searchInput.value.trim();
    if (query) {
      performUnifiedSearch(query);
    } else {
      clearSearch();
    }
    updateSearchURL(query, activeFilterTags, true);
  }

  // 9h. Tag suggestions rendering (D-02, D-05)
  function renderTagSuggestions(tags, query) {
    if (!tagSuggestList || !dropdownTagsSection) return;
    while (tagSuggestList.firstChild) {
      tagSuggestList.removeChild(tagSuggestList.firstChild);
    }

    // Filter out already-selected tags (D-05)
    var filtered = tags.filter(function (t) {
      var name = typeof t === 'string' ? t : t.name;
      return activeFilterTags.indexOf(name) === -1;
    });

    currentTagSuggestions = filtered.map(function (t) {
      return typeof t === 'string' ? t : t.name;
    });

    if (filtered.length === 0) {
      dropdownTagsSection.setAttribute('hidden', '');
      return;
    }

    filtered.forEach(function (tag, i) {
      var name = typeof tag === 'string' ? tag : tag.name;
      var li = document.createElement('li');
      li.setAttribute('role', 'option');
      li.setAttribute('id', 'tag-suggest-' + i);
      li.setAttribute('aria-selected', 'false');
      li.appendChild(highlightMatch(name, query));

      // mousedown + preventDefault to prevent input blur (Pitfall 6)
      li.addEventListener('mousedown', function (e) {
        e.preventDefault();
      });
      li.addEventListener('click', function (e) {
        e.stopPropagation();
        addFilterTag(name);
      });

      tagSuggestList.appendChild(li);
    });

    dropdownTagsSection.removeAttribute('hidden');
  }

  // 9i. Item results rendering (D-18, D-19, D-24, D-26)
  function renderItemResults(results, totalCount, query) {
    currentResults = results;
    while (searchListbox.firstChild) {
      searchListbox.removeChild(searchListbox.firstChild);
    }

    if (results.length === 0 && activeFilterTags.length === 0 && !query) {
      dropdownItemsSection.setAttribute('hidden', '');
      return;
    }

    if (results.length === 0) {
      dropdownItemsSection.setAttribute('hidden', '');
      // Show empty state (D-25)
      var emptyMsg;
      if (activeFilterTags.length > 0 && !query) {
        emptyMsg = 'No items match all selected tags';
      } else if (activeFilterTags.length > 0 && query) {
        emptyMsg = 'No items match your search with selected tags';
      } else {
        emptyMsg = 'No items found';
      }
      dropdownEmpty.textContent = emptyMsg;
      dropdownEmpty.removeAttribute('hidden');
      dropdownError.setAttribute('hidden', '');
      updateGridHighlights([], null);
      return;
    }

    dropdownEmpty.setAttribute('hidden', '');
    dropdownError.setAttribute('hidden', '');

    // Header: normal or truncated (D-24)
    if (totalCount > results.length) {
      searchItemsHeader.textContent = 'Showing ' + results.length + ' of ' + totalCount + ' results';
    } else {
      searchItemsHeader.textContent = 'ITEMS';
    }

    dropdownItemsSection.removeAttribute('hidden');

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
      // Only highlight name when matched_on includes "name"
      if (query && result.matched_on && result.matched_on.indexOf('name') !== -1) {
        nameSpan.appendChild(highlightMatch(result.name, query));
      } else {
        nameSpan.appendChild(document.createTextNode(result.name));
      }
      topDiv.appendChild(nameSpan);

      var badge = document.createElement('span');
      badge.className = 'position-badge';
      badge.textContent = result.container_label;
      topDiv.appendChild(badge);

      li.appendChild(topDiv);

      // Description line (D-18)
      if (result.description) {
        var descDiv = document.createElement('div');
        descDiv.className = 'search-result-desc';
        if (query && result.matched_on && result.matched_on.indexOf('description') !== -1) {
          descDiv.appendChild(highlightMatch(result.description, query));
        } else {
          descDiv.appendChild(document.createTextNode(result.description));
        }
        li.appendChild(descDiv);
      }

      // Tags row with clickable chips (D-26)
      if (result.tags && result.tags.length > 0) {
        var tagsDiv = document.createElement('div');
        tagsDiv.className = 'search-result-tags';
        result.tags.forEach(function (tag) {
          var chip = document.createElement('span');
          chip.className = 'tag-chip';
          // Highlight tag if matched_on includes "tag" and query matches
          if (query && result.matched_on && result.matched_on.indexOf('tag') !== -1 && tag.toLowerCase() === query.toLowerCase()) {
            chip.appendChild(highlightMatch(tag, query));
          } else {
            chip.appendChild(document.createTextNode(tag));
          }
          // Clicking tag chip adds it as filter (D-26)
          chip.addEventListener('mousedown', function (e) {
            e.preventDefault();
          });
          chip.addEventListener('click', function (e) {
            e.stopPropagation();
            addFilterTag(tag);
          });
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

    updateGridHighlights(results, null);

    // Auto-scroll grid to first highlighted cell
    if (results.length > 0) {
      var firstCell = findCell(results[0].container_id);
      if (firstCell) {
        firstCell.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    }
  }

  // 9j. Core: performUnifiedSearch (replaces old performSearch)
  function performUnifiedSearch(query) {
    // Abort stale requests (Pitfall 3)
    if (searchAbort) searchAbort.abort();
    if (tagAbort) tagAbort.abort();

    showSpinner();

    // Tag suggestions: only fetch when query has text
    var tagPromise;
    if (query.length > 0) {
      tagAbort = new AbortController();
      tagPromise = fetch('/api/tags?q=' + encodeURIComponent(query), { signal: tagAbort.signal })
        .then(function (resp) { return resp.json(); })
        .catch(function (err) {
          if (err.name === 'AbortError') return null;
          return [];
        });
    } else {
      tagPromise = Promise.resolve([]);
    }

    // Item results: fetch when 2+ chars OR active filters
    var itemPromise;
    if (query.length >= 2 || activeFilterTags.length > 0) {
      searchAbort = new AbortController();
      var searchUrl = '/api/search?q=' + encodeURIComponent(query);
      if (activeFilterTags.length > 0) {
        searchUrl += '&tags=' + encodeURIComponent(activeFilterTags.join(','));
      }
      itemPromise = fetch(searchUrl, { signal: searchAbort.signal })
        .then(function (resp) { return resp.json(); })
        .catch(function (err) {
          if (err.name === 'AbortError') return null;
          return { results: [], total_count: 0 };
        });
    } else {
      itemPromise = Promise.resolve({ results: [], total_count: 0 });
    }

    Promise.all([tagPromise, itemPromise]).then(function (responses) {
      var tagData = responses[0];
      var itemData = responses[1];

      hideSpinner();
      searchAbort = null;
      tagAbort = null;

      // Both aborted -- skip rendering
      if (tagData === null && itemData === null) return;

      // Race condition guard: check input still matches
      if (searchInput.value.trim() !== query && !(query === '' && activeFilterTags.length > 0)) return;

      // Render tag suggestions
      renderTagSuggestions(tagData || [], query);

      // Render item results
      var results = (itemData && itemData.results) || [];
      var totalCount = (itemData && itemData.total_count) || results.length;
      renderItemResults(results, totalCount, query);

      // Show/hide divider (only when both sections have content)
      var hasTags = currentTagSuggestions.length > 0;
      var hasItems = results.length > 0;
      if (dropdownDivider) {
        if (hasTags && hasItems) {
          dropdownDivider.removeAttribute('hidden');
        } else {
          dropdownDivider.setAttribute('hidden', '');
        }
      }

      // Update focusable count for keyboard navigation
      totalFocusableCount = currentTagSuggestions.length + results.length;
      focusedIndex = -1;
      lastQuery = query;

      // Show dropdown if any content
      if (hasTags || hasItems || (activeFilterTags.length > 0) || query.length >= 2) {
        showDropdown();
      } else {
        hideDropdown();
      }
    });
  }

  // 9k. Core: updateGridHighlights
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

  // 9l. Core: clearSearch
  function clearSearch() {
    searchInput.value = '';
    activeFilterTags = [];
    renderFilterChips();
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
    currentTagSuggestions = [];
    focusedIndex = -1;
    lastQuery = '';

    updateClearButton();
    updateBadgeCount();

    if (searchAbort) { searchAbort.abort(); searchAbort = null; }
    if (tagAbort) { tagAbort.abort(); tagAbort = null; }
    if (debounceTimer) { clearTimeout(debounceTimer); debounceTimer = null; }
    if (pushStateTimer) { clearTimeout(pushStateTimer); pushStateTimer = null; }

    // Clear URL params (D-16)
    updateSearchURL('', [], true);
  }

  // 9m. Core: selectResult
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

  // 9n. URL state management (D-13 through D-17)
  function updateSearchURL(query, tags, usePush) {
    var params = new URLSearchParams();
    if (query) params.set('q', query);
    if (tags.length > 0) params.set('tags', tags.join(','));
    var url = params.toString() ? '?' + params.toString() : window.location.pathname;
    if (usePush) {
      history.pushState({ q: query, tags: tags.slice() }, '', url);
    } else {
      history.replaceState({ q: query, tags: tags.slice() }, '', url);
    }
  }

  function restoreSearchFromURL() {
    var params = new URLSearchParams(window.location.search);
    var q = params.get('q') || '';
    var tagStr = params.get('tags') || '';
    var tags = tagStr ? tagStr.split(',').filter(Boolean) : [];

    // Set input value
    searchInput.value = q;

    // Set active filter tags and render chips
    activeFilterTags = tags;
    renderFilterChips();
    updateClearButton();

    // Execute search if any state present
    if (q || tags.length > 0) {
      performUnifiedSearch(q);
    }
  }

  // popstate handler (D-17)
  window.addEventListener('popstate', function () {
    restoreSearchFromURL();
  });

  // 9o. Helper: updateFocusedResult (unified across tags + items)
  function updateFocusedResult() {
    var tagItems = tagSuggestList ? tagSuggestList.querySelectorAll('li[role="option"]') : [];
    var resultItems = searchListbox.querySelectorAll('li[role="option"]');
    var tagCount = tagItems.length;

    // Clear all aria-selected
    var i;
    for (i = 0; i < tagItems.length; i++) { tagItems[i].setAttribute('aria-selected', 'false'); }
    for (i = 0; i < resultItems.length; i++) { resultItems[i].setAttribute('aria-selected', 'false'); }

    if (focusedIndex >= 0 && focusedIndex < tagCount) {
      // Focus is on a tag suggestion
      tagItems[focusedIndex].setAttribute('aria-selected', 'true');
      tagItems[focusedIndex].scrollIntoView({ block: 'nearest' });
      searchInput.setAttribute('aria-activedescendant', 'tag-suggest-' + focusedIndex);
    } else if (focusedIndex >= tagCount && focusedIndex < tagCount + resultItems.length) {
      // Focus is on an item result
      var itemIdx = focusedIndex - tagCount;
      resultItems[itemIdx].setAttribute('aria-selected', 'true');
      resultItems[itemIdx].scrollIntoView({ block: 'nearest' });
      searchInput.setAttribute('aria-activedescendant', 'search-result-' + itemIdx);
      // Update grid highlight-focus
      updateGridHighlights(currentResults, currentResults[itemIdx] ? currentResults[itemIdx].container_id : null);
    } else {
      searchInput.setAttribute('aria-activedescendant', '');
      updateGridHighlights(currentResults, null);
    }
  }

  // 9p. Event: searchInput 'input' handler
  searchInput.addEventListener('input', function () {
    updateClearButton();

    if (debounceTimer) clearTimeout(debounceTimer);
    if (pushStateTimer) clearTimeout(pushStateTimer);

    var query = searchInput.value.trim();

    // If no active filters and query too short, clear
    if (query.length < 1 && activeFilterTags.length === 0) {
      hideDropdown();
      hideSpinner();
      gridContainer.classList.remove('search-active');
      var cells = document.querySelectorAll('.grid-cell');
      cells.forEach(function (cell) {
        cell.classList.remove('highlight');
        cell.classList.remove('highlight-focus');
        var badge = cell.querySelector('.match-count');
        if (badge) badge.remove();
      });
      if (searchAbort) { searchAbort.abort(); searchAbort = null; }
      if (tagAbort) { tagAbort.abort(); tagAbort = null; }
      // Clear URL (D-16)
      updateSearchURL('', [], false);
      return;
    }

    // Close any open panel before entering search mode
    if (expandedPanel) collapseCell();

    showSpinner();
    debounceTimer = setTimeout(function () {
      performUnifiedSearch(query);
      // replaceState during typing (Pitfall 4)
      updateSearchURL(query, activeFilterTags, false);
    }, 250);

    // pushState after 1s of typing inactivity (settled state)
    pushStateTimer = setTimeout(function () {
      updateSearchURL(searchInput.value.trim(), activeFilterTags, true);
    }, 1000);
  });

  // 9q. Event: searchInput 'keydown' handler
  searchInput.addEventListener('keydown', function (e) {
    var dropdownVisible = searchDropdown.classList.contains('visible');
    var tagCount = currentTagSuggestions.length;

    if (e.key === 'ArrowDown') {
      if (dropdownVisible && totalFocusableCount > 0) {
        e.preventDefault();
        if (focusedIndex < totalFocusableCount - 1) {
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
        if (focusedIndex < tagCount) {
          // Tag suggestion selected
          addFilterTag(currentTagSuggestions[focusedIndex]);
        } else {
          // Item result selected
          selectResult(focusedIndex - tagCount);
        }
      }
    } else if (e.key === 'Escape') {
      if (focusedIndex >= 0) {
        focusedIndex = -1;
        updateFocusedResult();
        searchInput.focus();
        e.preventDefault();
        e.stopPropagation();
      } else if (dropdownVisible) {
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
      } else if (searchInput.value.length > 0 || activeFilterTags.length > 0) {
        clearSearch();
        e.preventDefault();
        e.stopPropagation();
      }
    } else if (e.key === 'Backspace') {
      // Remove last filter chip when input is empty (keyboard shortcut)
      if (searchInput.value === '' && activeFilterTags.length > 0) {
        removeFilterTag(activeFilterTags[activeFilterTags.length - 1]);
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
        focusedIndex = totalFocusableCount - 1;
        updateFocusedResult();
      }
    }
  });

  // 9r. Event: clearBtn 'click' handler
  if (clearBtn) {
    clearBtn.addEventListener('click', function (e) {
      e.preventDefault();
      e.stopPropagation();
      clearSearch();
      searchInput.focus();
    });
  }

  // 9s. Event: document 'keydown' for / shortcut
  document.addEventListener('keydown', function (e) {
    if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey) {
      var tag = document.activeElement.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if (document.activeElement.isContentEditable) return;
      e.preventDefault();
      searchInput.focus();
    }
  });

  // 9t. Event: click outside search area to close dropdown
  document.addEventListener('click', function (e) {
    if (searchDropdown.classList.contains('visible') && !e.target.closest('.search-sidebar')) {
      hideDropdown();
    }
  });

  // 9u. Event: input blur -- close dropdown after delay (allow click on suggestions)
  searchInput.addEventListener('blur', function () {
    setTimeout(function () {
      if (!searchDropdown.contains(document.activeElement) && document.activeElement !== searchInput) {
        hideDropdown();
      }
    }, 150);
  });

  // 9w. Cell deep-linking (D-12): auto-expand cell from ?cell= parameter
  (function handleCellDeepLink() {
    var params = new URLSearchParams(window.location.search);
    var cellParam = params.get('cell');
    if (!cellParam) return;

    var targetCell = document.querySelector('.grid-cell[data-coord="' + CSS.escape(cellParam) + '"]');
    if (targetCell) {
      targetCell.scrollIntoView({ behavior: 'smooth', block: 'center' });
      expandCell(targetCell);

      // Apply highlight pulse after a short delay to let the panel render
      setTimeout(function() {
        targetCell.classList.add('cell-highlight-pulse');
        targetCell.addEventListener('animationend', function handler() {
          targetCell.classList.remove('cell-highlight-pulse');
          targetCell.removeEventListener('animationend', handler);
        });
      }, 100);
    }

    // Clean up URL: remove cell param, preserve other params (q, tags)
    params.delete('cell');
    var cleanURL = params.toString() ? '?' + params.toString() : window.location.pathname;
    history.replaceState(null, '', cleanURL);
  })();

  // 9v. On page load: restore search state from URL (D-14)
  restoreSearchFromURL();

  // --- Avatar Pill: Initials + Dropdown ---

  (function initAvatarDropdown() {
    var pill = document.getElementById('avatar-pill');
    if (!pill) return;

    // Derive initials from display name
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

    // Shorten name to first name only
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
