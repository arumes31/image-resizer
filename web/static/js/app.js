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
    const removeBackgroundCheckbox = document.getElementById('removeBackground');
    const bgRemovalOptions = document.querySelectorAll('.bg-removal-options');
    const bgRemovalMethod = document.getElementById('bgRemovalMethod');
    const bgRemovalColorGroup = document.getElementById('bgRemovalColorGroup');
    const bgRemovalTolerance = document.getElementById('bgRemovalTolerance');
    const bgRemovalEdgeSmooth = document.getElementById('bgRemovalEdgeSmooth');
    const bgToleranceValue = document.getElementById('bgToleranceValue');
    const bgSmoothValue = document.getElementById('bgSmoothValue');

    // Social Media Tools
    const carouselSliceCheckbox = document.getElementById('carousel_slice');
    const carouselOptions = document.querySelectorAll('.carousel-options');
    const safeZoneOverlayCheckbox = document.getElementById('safe_zone_overlay');
    const safeZoneOptions = document.querySelectorAll('.safe-zone-options');
    const stitchImagesCheckbox = document.getElementById('stitch_images');
    const stitchOptions = document.querySelectorAll('.stitch-options');
    const faviconGenerateCheckbox = document.getElementById('favicon_generate');
    const faviconOptions = document.querySelectorAll('.favicon-options');
    const deviceFrameFileInput = document.getElementById('device-frame-file');

    let selectedFiles = [];
    let processedFileNames = [];

    // ---------------------------------------------------------------------------
    // Collapsible section toggle
    // ---------------------------------------------------------------------------
    window.toggleSection = function (sectionId) {
        const section = document.getElementById(sectionId);
        const header = section.previousElementSibling;
        const icon = header.querySelector('.toggle-icon');
        section.classList.toggle('collapsed');
        if (icon) {
            icon.textContent = section.classList.contains('collapsed') ? '▶' : '▼';
        }
    };

    // ---------------------------------------------------------------------------
    // Curves presets
    // ---------------------------------------------------------------------------
    const curvesPresets = {
        'high-contrast': '{"r":[[0,0],[64,40],[192,215],[255,255]],"g":[[0,0],[64,40],[192,215],[255,255]],"b":[[0,0],[64,40],[192,215],[255,255]]}',
        'low-contrast': '{"r":[[0,30],[64,80],[192,175],[255,225]],"g":[[0,30],[64,80],[192,175],[255,225]],"b":[[0,30],[64,80],[192,175],[255,225]]}',
        'brighten': '{"r":[[0,0],[64,80],[192,230],[255,255]],"g":[[0,0],[64,80],[192,230],[255,255]],"b":[[0,0],[64,80],[192,230],[255,255]]}',
        'darken': '{"r":[[0,0],[64,25],[192,175],[255,255]],"g":[[0,0],[64,25],[192,175],[255,255]],"b":[[0,0],[64,25],[192,175],[255,255]]}',
        'vintage': '{"r":[[0,20],[64,60],[192,200],[255,230]],"g":[[0,0],[64,50],[192,190],[255,240]],"b":[[0,30],[64,70],[192,210],[255,255]]}'
    };

    // ---------------------------------------------------------------------------
    // Selective color presets
    // ---------------------------------------------------------------------------
    const selectiveColorPresets = {
        'warm-sky': '{"reds":{"cyan":-5,"magenta":5},"blues":{"yellow":10,"cyan":-5}}',
        'cool-water': '{"blues":{"cyan":10},"cyans":{"cyan":5}}',
        'vivid-greens': '{"greens":{"cyan":5,"yellow":-5}}',
        'pop-reds': '{"reds":{"magenta":10,"yellow":-5}}'
    };

    // ---------------------------------------------------------------------------
    // Slider value updates (existing pattern)
    // ---------------------------------------------------------------------------
    document.querySelectorAll('input[type="range"]').forEach(slider => {
        slider.addEventListener('input', (e) => {
            const next = e.target.nextElementSibling;
            if (next && next.classList && next.classList.contains('value')) {
                next.textContent = e.target.value;
            }
        });
    });

    // ---------------------------------------------------------------------------
    // Professional adjustment slider value displays
    // ---------------------------------------------------------------------------
    const proSliders = [
        { id: 'hue', valueId: 'hueValue', format: v => v },
        { id: 'lightness', valueId: 'lightnessValue', format: v => v },
        { id: 'temperature', valueId: 'temperatureValue', format: v => v },
        { id: 'tint', valueId: 'tintValue', format: v => v },
        { id: 'shadow_recovery', valueId: 'shadowRecoveryValue', format: v => v },
        { id: 'highlight_recovery', valueId: 'highlightRecoveryValue', format: v => v },
        { id: 'levels_black', valueId: 'levelsBlackValue', format: v => v },
        { id: 'levels_white', valueId: 'levelsWhiteValue', format: v => v },
        { id: 'levels_gamma', valueId: 'levelsGammaValue', format: v => (v / 10).toFixed(1) },
        { id: 'chromatic_aberration', valueId: 'chromaticAberrationValue', format: v => v },
        { id: 'unsharp_amount', valueId: 'unsharpAmountValue', format: v => v },
        { id: 'unsharp_radius', valueId: 'unsharpRadiusValue', format: v => (v / 10).toFixed(1) },
        { id: 'grain_amount', valueId: 'grainAmountValue', format: v => v },
        { id: 'vignette_amount', valueId: 'vignetteAmountValue', format: v => v },
        { id: 'vignette_feather', valueId: 'vignetteFeatherValue', format: v => v },
        { id: 'vignette_roundness', valueId: 'vignetteRoundnessValue', format: v => v },
        { id: 'vignette_midpoint', valueId: 'vignetteMidpointValue', format: v => v }
    ];

    // ---------------------------------------------------------------------------
    // Branding & Overlays slider value displays
    // ---------------------------------------------------------------------------
    const brandingSliders = [
        { id: 'watermark_opacity', valueId: 'watermarkOpacityValue', format: v => v },
        { id: 'watermark_tile_spacing', valueId: 'watermarkTileSpacingValue', format: v => v },
        { id: 'qr_code_size', valueId: 'qrCodeSizeValue', format: v => v },
        { id: 'rounded_corners', valueId: 'roundedCornersValue', format: v => v },
        { id: 'drop_shadow_offset', valueId: 'dropShadowOffsetValue', format: v => v },
        { id: 'drop_shadow_blur', valueId: 'dropShadowBlurValue', format: v => (v / 10).toFixed(1) },
        { id: 'border_width', valueId: 'borderWidthValue', format: v => v },
        { id: 'signature_opacity', valueId: 'signatureOpacityValue', format: v => v },
        { id: 'signature_scale', valueId: 'signatureScaleValue', format: v => (v / 10).toFixed(1) }
    ];

    brandingSliders.forEach(({ id, valueId, format }) => {
        const slider = document.getElementById(id);
        const valueSpan = document.getElementById(valueId);
        if (slider && valueSpan) {
            valueSpan.textContent = format(parseFloat(slider.value));
            slider.addEventListener('input', () => {
                valueSpan.textContent = format(parseFloat(slider.value));
            });
        }
    });

    // ---------------------------------------------------------------------------
    // Branding & Overlays show/hide toggles
    // ---------------------------------------------------------------------------

    // Tile watermark options
    const watermarkTileCheckbox = document.getElementById('watermark_tile');
    const tileOptions = document.querySelectorAll('.branding-tile-options');
    if (watermarkTileCheckbox) {
        watermarkTileCheckbox.addEventListener('change', () => {
            tileOptions.forEach(el => { el.style.display = watermarkTileCheckbox.checked ? '' : 'none'; });
        });
    }

    // QR code options — show when text is entered
    const qrCodeTextInput = document.getElementById('qr_code_text');
    const qrCodeOptions = document.querySelectorAll('.qr-code-options');
    if (qrCodeTextInput) {
        qrCodeTextInput.addEventListener('input', () => {
            const show = qrCodeTextInput.value.trim() !== '';
            qrCodeOptions.forEach(el => { el.style.display = show ? '' : 'none'; });
        });
    }

    // Barcode options — show when text is entered
    const barcodeTextInput = document.getElementById('barcode_text');
    const barcodeOptions = document.querySelectorAll('.barcode-options');
    if (barcodeTextInput) {
        barcodeTextInput.addEventListener('input', () => {
            const show = barcodeTextInput.value.trim() !== '';
            barcodeOptions.forEach(el => { el.style.display = show ? '' : 'none'; });
        });
    }

    // Drop shadow options — show when offset > 0
    const dropShadowOffsetSlider = document.getElementById('drop_shadow_offset');
    const shadowOptions = document.querySelectorAll('.shadow-options');
    function updateShadowVisibility() {
        const show = parseInt(dropShadowOffsetSlider.value) > 0;
        shadowOptions.forEach(el => { el.style.display = show ? '' : 'none'; });
    }
    if (dropShadowOffsetSlider) {
        dropShadowOffsetSlider.addEventListener('input', updateShadowVisibility);
        updateShadowVisibility();
    }

    // Border options — show when width > 0
    const borderWidthSlider = document.getElementById('border_width');
    const borderOptions = document.querySelectorAll('.border-options');
    function updateBorderVisibility() {
        const show = parseInt(borderWidthSlider.value) > 0;
        borderOptions.forEach(el => { el.style.display = show ? '' : 'none'; });
    }
    if (borderWidthSlider) {
        borderWidthSlider.addEventListener('input', updateBorderVisibility);
        updateBorderVisibility();
    }

    // Placeholder options — show when width or height is filled
    const placeholderWidthInput = document.getElementById('placeholder_width');
    const placeholderHeightInput = document.getElementById('placeholder_height');
    const placeholderOptions = document.querySelectorAll('.placeholder-options');
    const placeholderNote = document.getElementById('placeholder-note');
    function updatePlaceholderVisibility() {
        const w = parseInt(placeholderWidthInput.value) || 0;
        const h = parseInt(placeholderHeightInput.value) || 0;
        const show = w > 0 && h > 0;
        placeholderOptions.forEach(el => { el.style.display = show ? '' : 'none'; });
        if (placeholderNote) {
            placeholderNote.style.display = show ? '' : 'none';
        }
    }
    if (placeholderWidthInput) {
        placeholderWidthInput.addEventListener('input', updatePlaceholderVisibility);
    }
    if (placeholderHeightInput) {
        placeholderHeightInput.addEventListener('input', updatePlaceholderVisibility);
    }

    // Signature options — show when a file is selected
    const signatureFileInput = document.getElementById('signature-file');
    const signatureOptions = document.querySelectorAll('.signature-options');
    if (signatureFileInput) {
        signatureFileInput.addEventListener('change', () => {
            const show = signatureFileInput.files && signatureFileInput.files.length > 0;
            signatureOptions.forEach(el => { el.style.display = show ? '' : 'none'; });
        });
    }

    // ---------------------------------------------------------------------------
    // Social Media Tools show/hide toggles
    // ---------------------------------------------------------------------------

    // Carousel slicer options
    if (carouselSliceCheckbox) {
        carouselSliceCheckbox.addEventListener('change', () => {
            carouselOptions.forEach(el => { el.style.display = carouselSliceCheckbox.checked ? '' : 'none'; });
        });
    }

    // Safe zone overlay options
    if (safeZoneOverlayCheckbox) {
        safeZoneOverlayCheckbox.addEventListener('change', () => {
            safeZoneOptions.forEach(el => { el.style.display = safeZoneOverlayCheckbox.checked ? '' : 'none'; });
        });
    }

    // Stitch options
    if (stitchImagesCheckbox) {
        stitchImagesCheckbox.addEventListener('change', () => {
            stitchOptions.forEach(el => { el.style.display = stitchImagesCheckbox.checked ? '' : 'none'; });
        });
    }

    // Favicon options
    if (faviconGenerateCheckbox) {
        faviconGenerateCheckbox.addEventListener('change', () => {
            faviconOptions.forEach(el => { el.style.display = faviconGenerateCheckbox.checked ? '' : 'none'; });
        });
    }

    // ---------------------------------------------------------------------------
    // Privacy & Security toggles
    // ---------------------------------------------------------------------------

    // Encryption toggle — show/hide password field
    const encryptOutput = document.getElementById('encryptOutput');
    const encryptionOptions = document.getElementById('encryptionOptions');
    if (encryptOutput) {
        encryptOutput.addEventListener('change', function () {
            if (encryptionOptions) {
                encryptionOptions.style.display = this.checked ? '' : 'none';
            }
        });
    }

    // Copy private link button
    const copyPrivateLink = document.getElementById('copyPrivateLink');
    if (copyPrivateLink) {
        copyPrivateLink.addEventListener('click', function () {
            const input = document.getElementById('privateLinkUrl');
            if (input && input.value) {
                navigator.clipboard.writeText(window.location.origin + input.value).then(() => {
                    this.textContent = 'Copied!';
                    setTimeout(() => { this.textContent = 'Copy'; }, 2000);
                });
            }
        });
    }

    // ---------------------------------------------------------------------------
    // Social preset auto-configuration
    // ---------------------------------------------------------------------------
    if (socialPreset) {
        socialPreset.addEventListener('change', () => {
            const selected = socialPreset.options[socialPreset.selectedIndex];
            const presetName = selected ? selected.getAttribute('data-name') : '';

            // Reset all social media options first
            if (carouselSliceCheckbox) carouselSliceCheckbox.checked = false;
            carouselOptions.forEach(el => { el.style.display = 'none'; });
            if (safeZoneOverlayCheckbox) safeZoneOverlayCheckbox.checked = false;
            safeZoneOptions.forEach(el => { el.style.display = 'none'; });
            if (faviconGenerateCheckbox) faviconGenerateCheckbox.checked = false;
            faviconOptions.forEach(el => { el.style.display = 'none'; });
            if (stitchImagesCheckbox) stitchImagesCheckbox.checked = false;
            stitchOptions.forEach(el => { el.style.display = 'none'; });

            const maxFileSizeInput = document.getElementById('max_file_size_kb');

            switch (presetName) {
                case 'Instagram Carousel':
                    if (carouselSliceCheckbox) {
                        carouselSliceCheckbox.checked = true;
                        carouselOptions.forEach(el => { el.style.display = ''; });
                        document.getElementById('carousel_slice_width').value = 1080;
                        document.getElementById('carousel_slice_height').value = 1350;
                    }
                    break;
                case 'Slack Emoji':
                case 'Discord Emoji':
                    if (maxFileSizeInput) maxFileSizeInput.value = 256;
                    break;
                case 'Favicon Pack':
                    if (faviconGenerateCheckbox) {
                        faviconGenerateCheckbox.checked = true;
                        faviconOptions.forEach(el => { el.style.display = ''; });
                    }
                    break;
                case 'LinkedIn Banner':
                    // Dimensions are already set by the preset value
                    break;
                case 'Twitter/X Header':
                    if (safeZoneOverlayCheckbox) {
                        safeZoneOverlayCheckbox.checked = true;
                        safeZoneOptions.forEach(el => { el.style.display = ''; });
                        document.getElementById('safe_zone_platform').value = 'twitter';
                    }
                    break;
                case 'YouTube Thumbnail':
                    if (safeZoneOverlayCheckbox) {
                        safeZoneOverlayCheckbox.checked = true;
                        safeZoneOptions.forEach(el => { el.style.display = ''; });
                        document.getElementById('safe_zone_platform').value = 'youtube';
                    }
                    break;
                case 'Pinterest Long Pin':
                    if (stitchImagesCheckbox) {
                        stitchImagesCheckbox.checked = true;
                        stitchOptions.forEach(el => { el.style.display = ''; });
                        document.getElementById('stitch_direction').value = 'vertical';
                    }
                    break;
                default:
                    if (maxFileSizeInput) maxFileSizeInput.value = 0;
                    break;
            }
        });
    }

    proSliders.forEach(({ id, valueId, format }) => {
        const slider = document.getElementById(id);
        const valueSpan = document.getElementById(valueId);
        if (slider && valueSpan) {
            // Set initial display
            valueSpan.textContent = format(parseFloat(slider.value));
            slider.addEventListener('input', () => {
                valueSpan.textContent = format(parseFloat(slider.value));
            });
        }
    });

    // ---------------------------------------------------------------------------
    // Curves preset handler
    // ---------------------------------------------------------------------------
    const curvesPresetSelect = document.getElementById('curvesPreset');
    const curvesPointsInput = document.getElementById('curvesPoints');
    if (curvesPresetSelect) {
        curvesPresetSelect.addEventListener('change', () => {
            const preset = curvesPresetSelect.value;
            if (preset && curvesPresets[preset]) {
                curvesPointsInput.value = curvesPresets[preset];
            } else {
                curvesPointsInput.value = '';
            }
        });
    }

    // ---------------------------------------------------------------------------
    // Selective color preset handler
    // ---------------------------------------------------------------------------
    const selectiveColorPresetSelect = document.getElementById('selectiveColorPreset');
    const selectiveColorDataInput = document.getElementById('selectiveColorData');
    if (selectiveColorPresetSelect) {
        selectiveColorPresetSelect.addEventListener('change', () => {
            const preset = selectiveColorPresetSelect.value;
            if (preset && selectiveColorPresets[preset]) {
                selectiveColorDataInput.value = selectiveColorPresets[preset];
            } else {
                selectiveColorDataInput.value = '';
            }
        });
    }

    // ---------------------------------------------------------------------------
    // Vignette custom options toggle
    // ---------------------------------------------------------------------------
    const vignetteAmountSlider = document.getElementById('vignette_amount');
    const vignetteCustomOptions = document.querySelectorAll('.vignette-custom-options');

    function updateVignetteCustomVisibility() {
        const amount = parseFloat(vignetteAmountSlider.value);
        vignetteCustomOptions.forEach(el => {
            el.style.display = amount > 0 ? '' : 'none';
        });
    }

    if (vignetteAmountSlider) {
        vignetteAmountSlider.addEventListener('input', updateVignetteCustomVisibility);
        updateVignetteCustomVisibility();
    }

    // Toggle pixelate input
    pixelateToggle.addEventListener('change', () => {
        pixelateInputRow.style.display = pixelateToggle.checked ? 'flex' : 'none';
    });

    // Toggle background removal options
    function updateBgRemovalVisibility() {
        const show = removeBackgroundCheckbox.checked;
        bgRemovalOptions.forEach(el => { el.style.display = show ? '' : 'none'; });
        // Color picker only visible when "color-match" is selected
        updateBgColorPickerVisibility();
    }

    function updateBgColorPickerVisibility() {
        const isColorMatch = bgRemovalMethod.value === 'color-match';
        if (bgRemovalColorGroup) {
            bgRemovalColorGroup.style.display = (removeBackgroundCheckbox.checked && isColorMatch) ? '' : 'none';
        }
    }

    removeBackgroundCheckbox.addEventListener('change', updateBgRemovalVisibility);
    bgRemovalMethod.addEventListener('change', updateBgColorPickerVisibility);

    // Update tolerance and smoothing value display labels
    bgRemovalTolerance.addEventListener('input', () => {
        bgToleranceValue.textContent = bgRemovalTolerance.value;
    });
    bgRemovalEdgeSmooth.addEventListener('input', () => {
        bgSmoothValue.textContent = bgRemovalEdgeSmooth.value;
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

        // Background Removal form data
        formData.append('remove_background', removeBackgroundCheckbox.checked ? 'on' : 'off');
        if (removeBackgroundCheckbox.checked) {
            formData.append('bg_removal_method', bgRemovalMethod.value);
            formData.append('bg_removal_tolerance', bgRemovalTolerance.value);
            formData.append('bg_removal_color', document.getElementById('bgRemovalColor').value);
            formData.append('bg_removal_edge_smooth', bgRemovalEdgeSmooth.value);
        }

        // Professional Adjustments form data
        const hueSlider = document.getElementById('hue');
        const lightnessSlider = document.getElementById('lightness');
        const temperatureSlider = document.getElementById('temperature');
        const tintSlider = document.getElementById('tint');
        const shadowRecoverySlider = document.getElementById('shadow_recovery');
        const highlightRecoverySlider = document.getElementById('highlight_recovery');
        const levelsBlackSlider = document.getElementById('levels_black');
        const levelsWhiteSlider = document.getElementById('levels_white');
        const levelsGammaSlider = document.getElementById('levels_gamma');
        const chromaticAberrationSlider = document.getElementById('chromatic_aberration');
        const unsharpAmountSlider = document.getElementById('unsharp_amount');
        const unsharpRadiusSlider = document.getElementById('unsharp_radius');
        const grainAmountSlider = document.getElementById('grain_amount');
        const vignetteAmountSlider = document.getElementById('vignette_amount');
        const vignetteFeatherSlider = document.getElementById('vignette_feather');
        const vignetteRoundnessSlider = document.getElementById('vignette_roundness');
        const vignetteMidpointSlider = document.getElementById('vignette_midpoint');

        if (hueSlider) formData.append('hue', hueSlider.value);
        if (lightnessSlider) formData.append('lightness', lightnessSlider.value);
        if (temperatureSlider) formData.append('temperature', temperatureSlider.value);
        if (tintSlider) formData.append('tint', tintSlider.value);
        if (shadowRecoverySlider) formData.append('shadow_recovery', shadowRecoverySlider.value);
        if (highlightRecoverySlider) formData.append('highlight_recovery', highlightRecoverySlider.value);
        if (levelsBlackSlider) formData.append('levels_black', levelsBlackSlider.value);
        if (levelsWhiteSlider) formData.append('levels_white', levelsWhiteSlider.value);
        // Levels gamma: slider value 1-100 maps to 0.1-10.0 (value/10)
        if (levelsGammaSlider) formData.append('levels_gamma', (parseFloat(levelsGammaSlider.value) / 10).toString());
        // Curves points from hidden field
        if (curvesPointsInput) formData.append('curves_points', curvesPointsInput.value);
        // Selective color from hidden field
        if (selectiveColorDataInput) formData.append('selective_color', selectiveColorDataInput.value);
        if (chromaticAberrationSlider) formData.append('chromatic_aberration', chromaticAberrationSlider.value);
        if (unsharpAmountSlider) formData.append('unsharp_amount', unsharpAmountSlider.value);
        // Unsharp radius: slider value 1-50 maps to 0.1-5.0 (value/10)
        if (unsharpRadiusSlider) formData.append('unsharp_radius', (parseFloat(unsharpRadiusSlider.value) / 10).toString());
        if (grainAmountSlider) formData.append('grain_amount', grainAmountSlider.value);
        if (vignetteAmountSlider) formData.append('vignette_amount', vignetteAmountSlider.value);
        if (vignetteFeatherSlider) formData.append('vignette_feather', vignetteFeatherSlider.value);
        if (vignetteRoundnessSlider) formData.append('vignette_roundness', vignetteRoundnessSlider.value);
        if (vignetteMidpointSlider) formData.append('vignette_midpoint', vignetteMidpointSlider.value);

        const watermarkFile = document.getElementById('watermark-file').files[0];
        if (watermarkFile) {
            formData.append('watermark', watermarkFile);
        }

        // Branding & Overlays form data
        const watermarkOpacitySlider = document.getElementById('watermark_opacity');
        const watermarkTileSpacingSlider = document.getElementById('watermark_tile_spacing');
        const qrCodeSizeSlider = document.getElementById('qr_code_size');
        const dropShadowBlurSlider = document.getElementById('drop_shadow_blur');
        const signatureOpacitySlider = document.getElementById('signature_opacity');
        const signatureScaleSlider = document.getElementById('signature_scale');

        formData.append('watermark_template', document.getElementById('watermark_template').value);
        formData.append('watermark_tile', watermarkTileCheckbox && watermarkTileCheckbox.checked ? 'on' : 'off');
        if (watermarkTileSpacingSlider) formData.append('watermark_tile_spacing', watermarkTileSpacingSlider.value);
        // Watermark opacity: slider 0-100 maps to 0.0-1.0 (value/100)
        if (watermarkOpacitySlider) formData.append('watermark_opacity', (parseFloat(watermarkOpacitySlider.value) / 100).toString());
        formData.append('qr_code_text', document.getElementById('qr_code_text').value);
        if (qrCodeSizeSlider) formData.append('qr_code_size', qrCodeSizeSlider.value);
        formData.append('qr_code_position', document.getElementById('qr_code_position').value);
        formData.append('barcode_text', document.getElementById('barcode_text').value);
        formData.append('barcode_type', document.getElementById('barcode_type').value);
        formData.append('rounded_corners', document.getElementById('rounded_corners').value);
        formData.append('drop_shadow_offset', document.getElementById('drop_shadow_offset').value);
        // Shadow blur: slider 1-20 maps to 0.1-2.0 (value/10)
        if (dropShadowBlurSlider) formData.append('drop_shadow_blur', (parseFloat(dropShadowBlurSlider.value) / 10).toString());
        formData.append('drop_shadow_color', document.getElementById('drop_shadow_color').value);
        formData.append('border_width', document.getElementById('border_width').value);
        formData.append('border_color', document.getElementById('border_color').value);
        formData.append('border_style', document.getElementById('border_style').value);
        formData.append('placeholder_width', document.getElementById('placeholder_width').value);
        formData.append('placeholder_height', document.getElementById('placeholder_height').value);
        formData.append('placeholder_text', document.getElementById('placeholder_text').value);
        formData.append('placeholder_bg_color', document.getElementById('placeholder_bg_color').value);
        formData.append('placeholder_text_color', document.getElementById('placeholder_text_color').value);
        formData.append('steganography_text', document.getElementById('steganography_text').value);
        formData.append('signature_position', document.getElementById('signature_position').value);
        // Signature opacity: slider 0-100 maps to 0.0-1.0 (value/100)
        if (signatureOpacitySlider) formData.append('signature_opacity', (parseFloat(signatureOpacitySlider.value) / 100).toString());
        // Signature scale: slider 1-20 maps to 0.1-2.0 (value/10)
        if (signatureScaleSlider) formData.append('signature_scale', (parseFloat(signatureScaleSlider.value) / 10).toString());

        const signatureFile = document.getElementById('signature-file').files[0];
        if (signatureFile) {
            formData.append('signature', signatureFile);
        }

        // Social Media Optimization form data
        if (carouselSliceCheckbox) formData.append('carousel_slice', carouselSliceCheckbox.checked ? 'on' : 'off');
        formData.append('carousel_slice_width', document.getElementById('carousel_slice_width').value);
        formData.append('carousel_slice_height', document.getElementById('carousel_slice_height').value);
        if (safeZoneOverlayCheckbox) formData.append('safe_zone_overlay', safeZoneOverlayCheckbox.checked ? 'on' : 'off');
        formData.append('safe_zone_platform', document.getElementById('safe_zone_platform').value);
        formData.append('max_file_size_kb', document.getElementById('max_file_size_kb').value);
        if (stitchImagesCheckbox) formData.append('stitch_images', stitchImagesCheckbox.checked ? 'on' : 'off');
        formData.append('stitch_direction', document.getElementById('stitch_direction').value);
        if (faviconGenerateCheckbox) formData.append('favicon_generate', faviconGenerateCheckbox.checked ? 'on' : 'off');
        formData.append('favicon_sizes', document.getElementById('favicon_sizes').value);
        formData.append('twitch_panel_width', document.getElementById('twitch_panel_width').value);
        formData.append('twitch_panel_height', document.getElementById('twitch_panel_height').value);

        const deviceFrameFile = deviceFrameFileInput ? deviceFrameFileInput.files[0] : null;
        if (deviceFrameFile) {
            formData.append('device_frame', deviceFrameFile);
        }

        const filters = Array.from(document.querySelectorAll('input[name="filters"]:checked')).map(cb => cb.value);
        filters.forEach(f => formData.append('filters[]', f));

        // Format & Encoding Options
        formData.append('progressive_jpeg', document.getElementById('progressive_jpeg').checked ? 'on' : 'off');
        formData.append('base64_output', document.getElementById('base64_output').checked ? 'on' : 'off');
        formData.append('ico_sizes', document.getElementById('ico_sizes').value);
        formData.append('lossless_webp', document.getElementById('lossless_webp').checked ? 'on' : 'off');
        formData.append('tone_map_hdr', document.getElementById('tone_map_hdr').checked ? 'on' : 'off');
        formData.append('tiff_extract_pages', document.getElementById('tiff_extract_pages').checked ? 'on' : 'off');
        formData.append('pdf_page', document.getElementById('pdf_page').value);
        formData.append('svg_scale', document.getElementById('svg_scale').value);

        // Privacy & Security
        formData.append('zero_log_mode', document.getElementById('zeroLogMode')?.checked ? 'on' : '');
        formData.append('encrypt_output', document.getElementById('encryptOutput')?.checked ? 'on' : '');
        formData.append('encryption_password', document.getElementById('encryptionPassword')?.value || '');

        // IMP-06 FIX: Toggle loading spinner visibility during processing.
        // The loader element existed in HTML but was never shown/hidden.
        processBtn.querySelector('span').textContent = 'Processing...';
        processBtn.disabled = true;
        if (loader) loader.classList.remove('hidden');

        // Check if zero-log mode is enabled — expect binary blob response
        const zeroLogMode = document.getElementById('zeroLogMode');
        if (zeroLogMode && zeroLogMode.checked) {
            try {
                const response = await fetch('/', { method: 'POST', body: formData });

                const contentType = response.headers.get('Content-Type') || '';
                if (contentType.startsWith('image/') || contentType === 'application/octet-stream') {
                    const blob = await response.blob();
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    // Try to get filename from Content-Disposition
                    const disposition = response.headers.get('Content-Disposition') || '';
                    let filename = 'processed-image';
                    const match = disposition.match(/filename="?([^";\n]+)"?/);
                    if (match) filename = match[1];
                    a.download = filename;
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);
                    URL.revokeObjectURL(url);
                    // Show success message
                    resultsContainer.classList.remove('hidden');
                    resultsList.innerHTML = '<div class="result-card"><p>✅ Image processed and downloaded (zero-log mode — no files stored on server)</p></div>';
                } else {
                    // Not a binary response — handle as text/HTML
                    const text = await response.text();
                    document.open();
                    document.write(text);
                    document.close();
                }
            } catch (error) {
                console.error('Error:', error);
                alert('Processing failed: ' + error.message);
            } finally {
                processBtn.querySelector('span').textContent = 'Process Images';
                processBtn.disabled = false;
                if (loader) loader.classList.add('hidden');
            }
            return; // Don't continue with normal submission
        }

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

            // Handle base64 output response (Item 33)
            if (data.base64_results && data.base64_results.length > 0) {
                displayBase64Results(data.base64_results);
                if (data.errors && data.errors.length > 0) {
                    console.warn('Some files had errors:', data.errors);
                }
                return;
            }

            // Server now returns { results: [...], errors?: [...] }
            const results = data.results || data;
            processedFileNames = [];
            results.forEach(r => {
                const pName = r.processedName || r.ProcessedName;
                if (pName) processedFileNames.push(pName);
                // Include extra files (carousel slides, favicon sizes, manifest)
                const extras = r.extraFiles || r.ExtraFiles;
                if (extras && Array.isArray(extras)) {
                    extras.forEach(f => processedFileNames.push(f));
                }
            });
            displayResults(results);

            // Show private links if present (encrypted output)
            if (data.private_links && data.private_links.length > 0) {
                const privateLinkGroup = document.getElementById('privateLinkGroup');
                const privateLinkUrl = document.getElementById('privateLinkUrl');
                if (privateLinkGroup && privateLinkUrl) {
                    privateLinkGroup.style.display = '';
                    // Show the first private link
                    privateLinkUrl.value = data.private_links[0].private_url || '';
                }
            }

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
            const isPdf = pName.toLowerCase().endsWith('.pdf');

            let displayElem;
            if (isPdf) {
                displayElem = document.createElement('div');
                displayElem.style.height = '120px';
                displayElem.style.display = 'flex';
                displayElem.style.alignItems = 'center';
                displayElem.style.justifyContent = 'center';
                displayElem.style.background = 'rgba(255,255,255,0.1)';
                displayElem.style.borderRadius = '8px';
                displayElem.style.marginBottom = '12px';
                displayElem.innerHTML = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>';
            } else {
                displayElem = document.createElement('img');
                displayElem.src = `/processed/${encodeURIComponent(pName)}`;
                displayElem.alt = pName;
            }

            const p = document.createElement('p');
            p.style.fontSize = '0.75rem';
            p.style.marginBottom = '8px';
            p.textContent = pSize;

            const a = document.createElement('a');
            a.href = `/download/${encodeURIComponent(pName)}`;
            a.download = pName;
            a.className = 'download-link';
            a.textContent = 'Download';

            card.appendChild(displayElem);
            card.appendChild(p);
            card.appendChild(a);

            // Extra files (carousel slides, favicon sizes, manifest.json)
            const extras = res.extraFiles || res.ExtraFiles;
            if (extras && Array.isArray(extras) && extras.length > 0) {
                const extrasDiv = document.createElement('div');
                extrasDiv.className = 'extra-files';

                const extrasLabel = document.createElement('p');
                extrasLabel.className = 'extra-files-label';
                extrasLabel.textContent = `+ ${extras.length} additional file(s)`;
                extrasDiv.appendChild(extrasLabel);

                extras.forEach(extraName => {
                    const ext = extraName.toLowerCase();
                    const isImage = /\.(png|jpg|jpeg|gif|webp|ico|svg)$/i.test(ext);
                    const isManifest = ext.endsWith('.json') || ext.endsWith('.webmanifest');

                    const extraRow = document.createElement('div');
                    extraRow.className = 'extra-file-row';

                    if (isImage) {
                        const thumb = document.createElement('img');
                        thumb.src = `/processed/${encodeURIComponent(extraName)}`;
                        thumb.alt = extraName;
                        thumb.className = 'extra-file-thumb';
                        extraRow.appendChild(thumb);
                    } else if (isManifest) {
                        const icon = document.createElement('span');
                        icon.className = 'extra-file-icon';
                        icon.innerHTML = '📄';
                        extraRow.appendChild(icon);
                    }

                    const nameSpan = document.createElement('span');
                    nameSpan.className = 'extra-file-name';
                    nameSpan.textContent = extraName;
                    extraRow.appendChild(nameSpan);

                    const dlLink = document.createElement('a');
                    dlLink.href = `/download/${encodeURIComponent(extraName)}`;
                    dlLink.download = extraName;
                    dlLink.className = 'download-link extra-download-link';
                    dlLink.textContent = '⬇';
                    extraRow.appendChild(dlLink);

                    extrasDiv.appendChild(extraRow);
                });

                card.appendChild(extrasDiv);
            }

            resultsList.appendChild(card);
        });

        resultsContainer.scrollIntoView({ behavior: 'smooth' });
    }

    // ---------------------------------------------------------------------------
    // Base64 output display (Item 33)
    // ---------------------------------------------------------------------------
    function displayBase64Results(base64Results) {
        resultsContainer.classList.remove('hidden');
        resultsList.innerHTML = '';

        base64Results.forEach(res => {
            const card = document.createElement('div');
            card.className = 'result-card base64-result-card';

            // Preview image from base64 data URI
            const preview = document.createElement('img');
            preview.src = res.base64;
            preview.alt = res.filename;
            preview.className = 'result-preview';
            card.appendChild(preview);

            // Filename
            const nameEl = document.createElement('p');
            nameEl.className = 'result-name';
            nameEl.textContent = res.filename;
            card.appendChild(nameEl);

            // Format and size info
            const infoEl = document.createElement('p');
            infoEl.className = 'result-info';
            infoEl.textContent = `Format: ${res.format} | Size: ${res.width}`;
            card.appendChild(infoEl);

            // Base64 textarea
            const textarea = document.createElement('textarea');
            textarea.className = 'base64-textarea';
            textarea.value = res.base64;
            textarea.readOnly = true;
            textarea.rows = 3;
            card.appendChild(textarea);

            // Copy button
            const copyBtn = document.createElement('button');
            copyBtn.className = 'secondary-btn copy-base64-btn';
            copyBtn.textContent = '📋 Copy Base64';
            copyBtn.addEventListener('click', () => {
                navigator.clipboard.writeText(res.base64).then(() => {
                    copyBtn.textContent = '✅ Copied!';
                    setTimeout(() => { copyBtn.textContent = '📋 Copy Base64'; }, 2000);
                });
            });
            card.appendChild(copyBtn);

            resultsList.appendChild(card);
        });

        resultsContainer.scrollIntoView({ behavior: 'smooth' });
    }

    // ---------------------------------------------------------------------------
    // Format-specific option visibility (Item 34/39)
    // Show/hide format options based on selected output format
    // ---------------------------------------------------------------------------
    function updateFormatOptions() {
        const format = document.getElementById('format').value;
        document.querySelectorAll('.format-option').forEach(opt => {
            const optFormat = opt.getAttribute('data-format');
            if (optFormat === 'jpeg' && (format === 'jpeg' || format === 'jpg')) {
                opt.style.display = '';
            } else if (optFormat === 'webp' && format === 'webp') {
                opt.style.display = '';
            } else if (optFormat === 'ico' && format === 'ico') {
                opt.style.display = '';
            } else if (optFormat === 'svg-input') {
                // Always show SVG scale option (it's input-related)
                opt.style.display = '';
            } else if (optFormat === 'tiff-input') {
                // Always show TIFF option (it's input-related)
                opt.style.display = '';
            } else if (optFormat === 'pdf-input') {
                // Always show PDF option (it's input-related)
                opt.style.display = '';
            } else {
                opt.style.display = 'none';
            }
        });
    }

    // Initialize format options and listen for changes
    updateFormatOptions();
    document.getElementById('format').addEventListener('change', updateFormatOptions);
});
