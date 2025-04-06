from flask import Flask, request, render_template, send_from_directory
from PIL import Image, ImageSequence
import os
from werkzeug.utils import secure_filename
import time
import glob
import logging

# Set up logging
logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger(__name__)

app = Flask(__name__)

UPLOAD_FOLDER = 'static/uploads'
PROCESSED_FOLDER = 'static/processed'
ALLOWED_EXTENSIONS = {'png', 'jpg', 'jpeg', 'gif', 'bmp', 'tiff', 'webp', 'ico'}
RESIZE_METHODS = {
    'nearest': Image.Resampling.NEAREST,
    'bilinear': Image.Resampling.BILINEAR,
    'bicubic': Image.Resampling.BICUBIC,
    'lanczos': Image.Resampling.LANCZOS
}
OUTPUT_FORMATS = ['PNG', 'JPEG', 'GIF', 'BMP', 'TIFF', 'WEBP', 'ICO']
ICO_SIZES = [(16, 16), (32, 32), (64, 64), (128, 128), (256, 256)]  # Standard ICO sizes

app.config['UPLOAD_FOLDER'] = UPLOAD_FOLDER
app.config['PROCESSED_FOLDER'] = PROCESSED_FOLDER
app.config['MAX_CONTENT_LENGTH'] = 16 * 1024 * 1024  # 16MB max upload size

def allowed_file(filename):
    return '.' in filename and filename.rsplit('.', 1)[1].lower() in ALLOWED_EXTENSIONS

def cleanup_old_files():
    now = time.time()
    # Clean uploaded files older than 1 hour
    upload_cutoff = now - (1 * 3600)  # 1 hour
    for filepath in glob.glob(f"{UPLOAD_FOLDER}/*"):
        if os.path.exists(filepath) and os.path.getmtime(filepath) < upload_cutoff:
            try:
                os.remove(filepath)
                logger.debug(f"Cleaned up old uploaded file: {filepath}")
            except Exception as e:
                logger.error(f"Error cleaning up uploaded file {filepath}: {e}")

    # Clean processed files older than 12 hours
    processed_cutoff = now - (12 * 3600)  # 12 hours
    for filepath in glob.glob(f"{PROCESSED_FOLDER}/*"):
        if os.path.exists(filepath) and os.path.getmtime(filepath) < processed_cutoff:
            try:
                os.remove(filepath)
                logger.debug(f"Cleaned up old processed file: {filepath}")
            except Exception as e:
                logger.error(f"Error cleaning up processed file {filepath}: {e}")

def get_file_size(filepath):
    try:
        size_bytes = os.path.getsize(filepath)
        if size_bytes < 1024 * 1024:  # Less than 1MB
            return f"{size_bytes / 1024:.2f} KB"
        return f"{size_bytes / (1024 * 1024):.2f} MB"
    except Exception as e:
        logger.error(f"Error getting file size for {filepath}: {e}")
        return "Unknown"

def process_image(img, operation, percentage, width, height, quality, resize_method, is_gif=False, is_ico=False, output_format='JPEG'):
    original_width, original_height = img.size
    
    if operation == 'percentage' and percentage:
        new_width = int(original_width * (percentage / 100))
        new_height = int(original_height * (percentage / 100))
    elif operation == 'dimensions' and width and height:
        new_width = width
        new_height = height
    else:
        raise ValueError('Invalid parameters')

    if is_gif and output_format == 'GIF':
        # Handle animated GIF
        frames = []
        durations = []
        for frame in ImageSequence.Iterator(img):
            resized_frame = frame.copy().resize((new_width, new_height), RESIZE_METHODS[resize_method])
            frames.append(resized_frame)
            durations.append(frame.info.get('duration', 100))  # Default to 100ms if not specified
        return frames, durations, (new_width, new_height), None
    elif output_format == 'ICO':
        # Handle ICO output with multiple sizes
        ico_images = []
        ico_sizes = []
        for size in ICO_SIZES:
            # Ensure the size doesn't exceed the requested dimensions
            ico_width, ico_height = size
            if ico_width > new_width or ico_height > new_height:
                continue
            resized_img = img.resize(size, RESIZE_METHODS[resize_method])
            ico_images.append(resized_img)
            ico_sizes.append(size)
        return ico_images, None, (new_width, new_height), ico_sizes
    else:
        # Handle other formats
        resized_img = img.resize((new_width, new_height), RESIZE_METHODS[resize_method])
        return resized_img, None, (new_width, new_height), None

@app.route('/', methods=['GET', 'POST'])
def upload_file():
    cleanup_old_files()
    
    if request.method == 'POST':
        files = request.files.getlist('files[]')
        if not files:
            return 'No files uploaded'
        
        processed_files = []
        operation = request.form.get('operation')
        percentage = request.form.get('percentage', type=int)
        width = request.form.get('width', type=int)
        height = request.form.get('height', type=int)
        quality = request.form.get('quality', type=int, default=85)
        output_format = request.form.get('format', 'JPEG')
        resize_method = request.form.get('resize_method', 'lanczos')

        for file in files:
            if file and allowed_file(file.filename):
                try:
                    filename = secure_filename(file.filename)
                    filepath = os.path.join(app.config['UPLOAD_FOLDER'], filename)
                    file.save(filepath)

                    img = Image.open(filepath)
                    is_gif = filename.lower().endswith('.gif') and img.format == 'GIF' and getattr(img, 'is_animated', False)
                    is_ico = filename.lower().endswith('.ico') and img.format == 'ICO'

                    # For ICO files, select the largest image if multiple sizes exist
                    if is_ico and hasattr(img, 'size') and isinstance(img.info.get('sizes'), set):
                        largest_size = max(img.info['sizes'], key=lambda s: s[0] * s[1])
                        img = img.resize(largest_size, RESIZE_METHODS[resize_method])

                    if img.mode in ('RGBA', 'P') and output_format not in ['GIF', 'PNG', 'ICO']:
                        img = img.convert('RGB')
                    
                    result = process_image(img, operation, percentage, width, height, 
                                        quality, resize_method, is_gif, is_ico, output_format)
                    
                    processed_filename = f"processed_{int(time.time())}_{filename.rsplit('.', 1)[0]}.{output_format.lower()}"
                    processed_path = os.path.join(app.config['PROCESSED_FOLDER'], processed_filename)

                    if is_gif and output_format == 'GIF':
                        # Save animated GIF
                        frames, durations, new_size, _ = result
                        frames[0].save(
                            processed_path,
                            save_all=True,
                            append_images=frames[1:],
                            duration=durations,
                            loop=0
                        )
                    elif output_format == 'ICO':
                        # Save ICO with multiple sizes
                        ico_images, _, new_size, ico_sizes = result
                        if not ico_images:
                            logger.error("No ICO sizes were generated.")
                            continue
                        ico_images[0].save(
                            processed_path,
                            format='ICO',
                            sizes=[(img.width, img.height) for img in ico_images]
                        )
                    else:
                        # Save single frame image
                        resized_img, _, new_size, _ = result
                        save_params = {'quality': quality} if output_format == 'JPEG' else {}
                        resized_img.save(processed_path, format=output_format, **save_params)

                    # Delete the uploaded file immediately after processing
                    try:
                        os.remove(filepath)
                        logger.debug(f"Deleted uploaded file after processing: {filepath}")
                    except Exception as e:
                        logger.error(f"Error deleting uploaded file {filepath}: {e}")

                    processed_files.append({
                        'original': filename,
                        'processed': processed_filename,
                        'original_size': f"{img.size[0]}x{img.size[1]}",
                        'original_file_size': get_file_size(filepath) if os.path.exists(filepath) else "Deleted",
                        'new_size': f"{new_size[0]}x{new_size[1]}",
                        'new_file_size': get_file_size(processed_path),
                        'ico_sizes': ico_sizes if output_format == 'ICO' else None  # Add ICO sizes if applicable
                    })
                except Exception as e:
                    logger.error(f"Error processing file {file.filename}: {e}")
                    continue

        if not processed_files:
            return "No files were processed successfully. Check logs for details."

        logger.debug(f"Processed files: {processed_files}")
        return render_template('index.html', 
                            processed_files=processed_files,
                            resize_methods=RESIZE_METHODS.keys(),
                            output_formats=OUTPUT_FORMATS)

    return render_template('index.html', 
                         resize_methods=RESIZE_METHODS.keys(),
                         output_formats=OUTPUT_FORMATS)

@app.route('/uploads/<filename>')
def uploaded_file(filename):
    return send_from_directory(app.config['UPLOAD_FOLDER'], filename)

@app.route('/processed/<filename>')
def processed_file(filename):
    return send_from_directory(app.config['PROCESSED_FOLDER'], filename)

if __name__ == '__main__':
    os.makedirs(UPLOAD_FOLDER, exist_ok=True)
    os.makedirs(PROCESSED_FOLDER, exist_ok=True)
    app.run(host='0.0.0.0', port=5000, debug=True)