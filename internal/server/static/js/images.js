(function() {
    'use strict';

    var uploadArea = document.getElementById('images-upload-area');
    var uploadBtn = document.getElementById('images-upload-btn');
    var fileInput = document.getElementById('images-file-input');
    var uploadList = document.getElementById('images-upload-list');
    var grid = document.getElementById('images-grid');

    if (!uploadArea || !uploadBtn || !fileInput || !grid) return;

    function getCSRFToken() {
        var match = document.cookie.match(/(?:^|; )screwsbox_csrf=([^;]*)/);
        return match ? match[1] : '';
    }

    // Click-to-upload
    uploadBtn.addEventListener('click', function() { fileInput.click(); });
    fileInput.addEventListener('change', function() {
        for (var i = 0; i < fileInput.files.length; i++) {
            uploadFile(fileInput.files[i]);
        }
        fileInput.value = '';
    });

    // Drag-and-drop on upload area
    var dragCounter = 0;
    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(function(evt) {
        uploadArea.addEventListener(evt, function(e) { e.preventDefault(); e.stopPropagation(); });
    });
    uploadArea.addEventListener('dragenter', function() {
        dragCounter++;
        uploadArea.classList.add('drag-active');
    });
    uploadArea.addEventListener('dragleave', function() {
        dragCounter--;
        if (dragCounter === 0) uploadArea.classList.remove('drag-active');
    });
    uploadArea.addEventListener('drop', function(e) {
        dragCounter = 0;
        uploadArea.classList.remove('drag-active');
        var files = e.dataTransfer.files;
        for (var i = 0; i < files.length; i++) {
            uploadFile(files[i]);
        }
    });

    // Prevent default drag on document
    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(function(evt) {
        document.addEventListener(evt, function(e) { e.preventDefault(); });
    });

    function uploadFile(file) {
        if (file.size > 10 * 1024 * 1024) {
            addUploadStatus(file.name, 'error', 'File exceeds the 10 MB limit.');
            return;
        }

        var statusEl = addUploadStatus(file.name, 'uploading', '');
        var progressEl = statusEl.querySelector('progress');

        var xhr = new XMLHttpRequest();
        var formData = new FormData();
        formData.append('photo', file);

        xhr.upload.addEventListener('progress', function(e) {
            if (e.lengthComputable) {
                progressEl.value = Math.round((e.loaded / e.total) * 100);
            }
        });
        xhr.addEventListener('load', function() {
            if (xhr.status === 201) {
                var photo = JSON.parse(xhr.responseText);
                statusEl.remove();
                addPhotoCard(photo);
                removeEmptyState();
            } else {
                var err = 'Upload failed. Please try again.';
                try { err = JSON.parse(xhr.responseText).error || err; } catch(ex) {}
                statusEl.className = 'upload-progress error';
                statusEl.querySelector('.upload-progress-filename').textContent = err;
            }
        });
        xhr.addEventListener('error', function() {
            statusEl.className = 'upload-progress error';
            statusEl.querySelector('.upload-progress-filename').textContent = 'Upload failed. Please try again.';
        });

        xhr.open('POST', '/api/photos/upload');
        xhr.setRequestHeader('X-CSRF-Token', getCSRFToken());
        xhr.send(formData);
    }

    function addUploadStatus(filename, state, message) {
        var div = document.createElement('div');
        div.className = 'upload-progress' + (state === 'error' ? ' error' : '');
        if (state === 'uploading') {
            var prog = document.createElement('progress');
            prog.max = 100;
            prog.value = 0;
            prog.setAttribute('aria-label', 'Uploading ' + filename);
            div.appendChild(prog);
            var fnDiv = document.createElement('div');
            fnDiv.className = 'upload-progress-filename';
            fnDiv.textContent = filename;
            div.appendChild(fnDiv);
        } else {
            var msgDiv = document.createElement('div');
            msgDiv.className = 'upload-progress-filename';
            msgDiv.textContent = message || filename;
            div.appendChild(msgDiv);
        }
        uploadList.appendChild(div);
        return div;
    }

    function addPhotoCard(photo) {
        var card = document.createElement('div');
        card.className = 'images-card';
        card.dataset.uuid = photo.uuid;
        card.dataset.filename = photo.original_filename;
        card.dataset.uploaded = photo.uploaded_at;

        var img = document.createElement('img');
        img.src = photo.thumb_url;
        img.alt = photo.original_filename;
        img.loading = 'lazy';
        card.appendChild(img);

        var infoDiv = document.createElement('div');
        infoDiv.className = 'images-card-info';
        var fnDiv = document.createElement('div');
        fnDiv.className = 'images-card-filename';
        fnDiv.textContent = photo.original_filename;
        infoDiv.appendChild(fnDiv);
        var dateDiv = document.createElement('div');
        dateDiv.className = 'images-card-date';
        dateDiv.textContent = photo.uploaded_at;
        infoDiv.appendChild(dateDiv);
        card.appendChild(infoDiv);

        card.addEventListener('click', function() {
            showLightbox(photo.uuid, photo.original_filename, photo.uploaded_at);
        });
        grid.appendChild(card);
    }

    function removeEmptyState() {
        var empty = grid.querySelector('.images-empty');
        if (empty) empty.remove();
    }

    // Lightbox
    function showLightbox(photoUUID, filename, uploadedAt) {
        var overlay = document.createElement('div');
        overlay.className = 'lightbox-overlay';
        overlay.setAttribute('role', 'dialog');
        overlay.setAttribute('aria-modal', 'true');
        overlay.setAttribute('aria-label', 'Photo viewer');

        var img = document.createElement('img');
        img.src = '/api/photos/' + photoUUID + '/full';
        img.alt = filename;

        var info = document.createElement('div');
        info.className = 'lightbox-info';
        info.textContent = filename + ' \u2014 Uploaded ' + formatDate(uploadedAt);

        var closeBtn = document.createElement('button');
        closeBtn.className = 'lightbox-close';
        closeBtn.setAttribute('aria-label', 'Close');
        closeBtn.textContent = '\u00D7';

        overlay.appendChild(img);
        overlay.appendChild(info);
        overlay.appendChild(closeBtn);
        document.body.appendChild(overlay);

        function closeLB() { overlay.remove(); document.removeEventListener('keydown', kh); }
        function kh(e) { if (e.key === 'Escape') closeLB(); }
        closeBtn.addEventListener('click', closeLB);
        overlay.addEventListener('click', function(e) { if (e.target === overlay) closeLB(); });
        document.addEventListener('keydown', kh);
        closeBtn.focus();
    }

    function formatDate(iso) {
        var d = new Date(iso);
        return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
    }

    // Attach lightbox to server-rendered cards
    var cards = grid.querySelectorAll('.images-card');
    for (var i = 0; i < cards.length; i++) {
        (function(card) {
            card.addEventListener('click', function() {
                showLightbox(card.dataset.uuid, card.dataset.filename, card.dataset.uploaded);
            });
        })(cards[i]);
    }
})();
