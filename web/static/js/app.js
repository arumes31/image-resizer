document.addEventListener('DOMContentLoaded', () => {
    const dropZone = document.getElementById('drop-zone');
    const fileInput = document.getElementById('file-input');
    const processBtn = document.getElementById('process-btn');
    const modePercentage = document.getElementById('mode-percentage');
    const modeDimensions = document.getElementById('mode-dimensions');
    const percentageInput = document.getElementById('percentage-input');
    const dimensionsInput = document.getElementById('dimensions-input');
    const resultsContainer = document.getElementById('results-container');
    const resultsList = document.getElementById('results-list');
    const downloadAllBtn = document.getElementById('download-all-btn');

    let selectedFiles = [];
    let processedFileNames = [];

    // Toggle inputs
    modePercentage.addEventListener('change', () => {
        percentageInput.classList.remove('hidden');
        dimensionsInput.classList.add('hidden');
    });

    modeDimensions.addEventListener('change', () => {
        percentageInput.classList.add('hidden');
        dimensionsInput.classList.remove('hidden');
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
        
        const operation = document.querySelector('input[name="operation"]:checked').value;
        formData.append('operation', operation);
        formData.append('percentage', document.getElementById('percentage').value);
        formData.append('width', document.getElementById('width').value);
        formData.append('height', document.getElementById('height').value);
        formData.append('format', document.getElementById('format').value);
        formData.append('resize_method', document.getElementById('resize-method').value);
        formData.append('rotation', document.getElementById('rotation').value);
        formData.append('flip', document.getElementById('flip').value);
        formData.append('text_overlay', document.getElementById('text-overlay').value);
        
        const watermarkFile = document.getElementById('watermark-file').files[0];
        if (watermarkFile) {
            formData.append('watermark', watermarkFile);
        }

        const filters = Array.from(document.querySelectorAll('input[name="filters"]:checked')).map(cb => cb.value);
        filters.forEach(f => formData.append('filters[]', f));

        processBtn.querySelector('span').textContent = 'Processing...';
        processBtn.disabled = true;

        try {
            const response = await fetch('/', {
                method: 'POST',
                body: formData
            });

            const results = await response.json();
            processedFileNames = results.map(r => r.ProcessedName);
            displayResults(results);
        } catch (error) {
            console.error('Error:', error);
            alert('Processing failed');
        } finally {
            processBtn.querySelector('span').textContent = 'Process Images';
            processBtn.disabled = false;
        }
    });

    downloadAllBtn.addEventListener('click', () => {
        if (processedFileNames.length === 0) return;
        const url = `/download-all?files=${processedFileNames.join(',')}`;
        window.location.href = url;
    });

    function displayResults(results) {
        resultsContainer.classList.remove('hidden');
        resultsList.innerHTML = '';

        results.forEach(res => {
            const card = document.createElement('div');
            card.className = 'result-card';
            card.innerHTML = `
                <img src="/processed/${res.ProcessedName}" alt="${res.ProcessedName}">
                <p style="font-size: 0.75rem; margin-bottom: 8px;">${res.NewSize}</p>
                <a href="/processed/${res.ProcessedName}" download="${res.ProcessedName}" class="download-link">Download</a>
            `;
            resultsList.appendChild(card);
        });

        resultsContainer.scrollIntoView({ behavior: 'smooth' });
    }
});
