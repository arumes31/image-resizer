document.addEventListener('DOMContentLoaded', () => {
    const dropZone = document.getElementById('drop-zone');
    const fileInput = document.getElementById('file-input');
    const processBtn = document.getElementById('process-btn');
    const modePercentage = document.getElementById('mode-percentage');
    const modeDimensions = document.getElementById('mode-dimensions');
    const modeSocial = document.getElementById('mode-social');
    const percentageInput = document.getElementById('percentage-input');
    const dimensionsInput = document.getElementById('dimensions-input');
    const socialInput = document.getElementById('social-input');
    const socialPreset = document.getElementById('social-preset');
    const resultsContainer = document.getElementById('results-container');
    const resultsList = document.getElementById('results-list');
    const downloadAllBtn = document.getElementById('download-all-btn');
    const pixelateToggle = document.querySelector('input[name="filters"][value="pixelate"]');
    const pixelateInputRow = document.getElementById('pixelate-input');
    const loader = document.getElementById('loader');

    let selectedFiles = [];
    let processedFileNames = [];

    // Slider value updates
    document.querySelectorAll('input[type="range"]').forEach(slider => {
        slider.addEventListener('input', (e) => {
            e.target.nextElementSibling.textContent = e.target.value;
        });
    });

    // Toggle pixelate input
    pixelateToggle.addEventListener('change', () => {
        pixelateInputRow.style.display = pixelateToggle.checked ? 'flex' : 'none';
    });

    // Toggle inputs
    modePercentage.addEventListener('change', () => {
        percentageInput.classList.remove('hidden');
        dimensionsInput.classList.add('hidden');
        socialInput.classList.add('hidden');
    });

    modeDimensions.addEventListener('change', () => {
        percentageInput.classList.add('hidden');
        dimensionsInput.classList.remove('hidden');
        socialInput.classList.add('hidden');
    });

    modeSocial.addEventListener('change', () => {
        percentageInput.classList.add('hidden');
        dimensionsInput.classList.add('hidden');
        socialInput.classList.remove('hidden');
    });

    // File selection
    dropZone.addEventListener('click', () => fileInput.click());

    fileInput.addEventListener('change', (e) => {
        handleFiles(e.target.files);
    });

    dropZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropZone.classList.add('dragover');
    });

    ['dragleave', 'drop'].forEach(evt => {
        dropZone.addEventListener(evt, () => dropZone.classList.remove('dragover'));
    });

    dropZone.addEventListener('drop', (e) => {
        e.preventDefault();
        handleFiles(e.dataTransfer.files);
    });

    function handleFiles(files) {
        selectedFiles = Array.from(files);
        if (selectedFiles.length > 0) {
            const p = dropZone.querySelector('p');
            p.textContent = `${selectedFiles.length} file(s) selected`;
        }
    }

    // Processing
    processBtn.addEventListener('click', async () => {
        if (selectedFiles.length === 0) {
            alert('Please select files first');
            return;
        }

        const formData = new FormData();
        selectedFiles.forEach(file => formData.append('files[]', file));

        let operation = document.querySelector('input[name="operation"]:checked').value;

        // BUG-13 FIX: Social presets now send operation="fill" instead of
        // operation="dimensions". The Go processor handles "fill" mode by
        // using imaging.Fill (crop-to-fit) instead of imaging.Resize
        // (stretch-to-fit), producing correct social media thumbnails.
        if (operation === 'social') {
            const [w, h] = socialPreset.value.split('x');
            formData.append('width', w);
            formData.append('height', h);
            operation = 'fill';
        } else {
            formData.append('percentage', document.getElementById('percentage').value);
            formData.append('width', document.getElementById('width').value);
            formData.append('height', document.getElementById('height').value);
        }

        formData.append('operation', operation);
        formData.append('format', document.getElementById('format').value);
        formData.append('resize_method', document.getElementById('resize-method').value);
        formData.append('rotation', document.getElementById('rotation').value);
        formData.append('flip', document.getElementById('flip').value);
        formData.append('text_overlay', document.getElementById('text-overlay').value);
        // BUG-15 FIX: Send text_color parameter to server.
        // Previously, the text color was never sent, so text overlay
        // always defaulted to white regardless of user selection.
        formData.append('text_color', document.getElementById('text-color').value);
        formData.append('strip_exif', document.getElementById('strip-exif').checked ? 'on' : 'off');
        formData.append('copyright', document.getElementById('copyright').value);
        formData.append('rename_template', document.getElementById('rename-template').value);
        formData.append('crop', document.getElementById('crop').value);
        formData.append('brightness', document.getElementById('brightness').value);
        formData.append('contrast', document.getElementById('contrast').value);
        formData.append('saturation', document.getElementById('saturation').value);
        formData.append('pixelate', document.getElementById('pixelate').value);
        // BUG-14 FIX: Send quality value to server.
        // Previously, quality was never appended to the form data,
        // so the server always defaulted to 100, producing very large files.
        formData.append('quality', document.getElementById('quality').value);
        formData.append('vignette', document.getElementById('vignette').checked ? 'on' : 'off');

        const watermarkFile = document.getElementById('watermark-file').files[0];
        if (watermarkFile) {
            formData.append('watermark', watermarkFile);
        }

        const filters = Array.from(document.querySelectorAll('input[name="filters"]:checked')).map(cb => cb.value);
        filters.forEach(f => formData.append('filters[]', f));

        // IMP-06 FIX: Toggle loading spinner visibility during processing.
        // The loader element existed in HTML but was never shown/hidden.
        processBtn.querySelector('span').textContent = 'Processing...';
        processBtn.disabled = true;
        if (loader) loader.classList.remove('hidden');

        try {
            const response = await fetch('/', {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                const errData = await response.json().catch(() => null);
                const errMsg = errData?.error || `HTTP ${response.status}`;
                throw new Error(errMsg);
            }

            const data = await response.json();
            // Server now returns { results: [...], errors?: [...] }
            const results = data.results || data;
            processedFileNames = results.map(r => r.processedName || r.ProcessedName);
            displayResults(results);

            // Show partial errors if any
            if (data.errors && data.errors.length > 0) {
                console.warn('Some files had errors:', data.errors);
            }
        } catch (error) {
            console.error('Error:', error);
            alert(`Processing failed: ${error.message}`);
        } finally {
            processBtn.querySelector('span').textContent = 'Process Images';
            processBtn.disabled = false;
            if (loader) loader.classList.add('hidden');
        }
    });

    downloadAllBtn.addEventListener('click', () => {
        if (processedFileNames.length === 0) return;
        const url = `/download-all?files=${processedFileNames.map(encodeURIComponent).join(',')}`;
        window.location.href = url;
    });

    function displayResults(results) {
        resultsContainer.classList.remove('hidden');
        resultsList.innerHTML = '';

        results.forEach(res => {
            const card = document.createElement('div');
            card.className = 'result-card';

            const pName = res.processedName || res.ProcessedName;
            const pSize = res.newSize || res.NewSize;

            const img = document.createElement('img');
            img.src = `/processed/${encodeURI(pName)}`;
            img.alt = pName; // Safe as property assignment

            const p = document.createElement('p');
            p.style.fontSize = '0.75rem';
            p.style.marginBottom = '8px';
            p.textContent = pSize;

            const a = document.createElement('a');
            a.href = `/download/${encodeURI(pName)}`;
            a.download = pName;
            a.className = 'download-link';
            a.textContent = 'Download';

            card.appendChild(img);
            card.appendChild(p);
            card.appendChild(a);

            resultsList.appendChild(card);
        });

        resultsContainer.scrollIntoView({ behavior: 'smooth' });
    }
});
