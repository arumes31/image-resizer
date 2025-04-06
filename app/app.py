from flask import Flask, request, render_template, send_from_directory
from PIL import Image
import os
from werkzeug.utils import secure_filename
import time
import glob

app = Flask(__name__)

UPLOAD_FOLDER = 'static/uploads'
PROCESSED_FOLDER = 'static/processed'
ALLOWED_EXTENSIONS = {'png', 'jpg', 'jpeg', 'gif', 'bmp', 'tiff', 'webp'}
RESIZE_METHODS = {
    'nearest': Image.Resampling.NEAREST,
    'bilinear': Image.Resampling.BILINEAR,
    'bicubic': Image.Resampling.BICUBIC,
    'lanczos': Image.Resampling.LANCZOS
}
OUTPUT_FORMATS = ['PNG', 'JPEG', 'GIF', 'BMP', 'TIFF', 'WEBP']

app.config['UPLOAD_FOLDER'] = UPLOAD_FOLDER
app.config['PROCESSED_FOLDER'] = PROCESSED_FOLDER
app.config['MAX_CONTENT_LENGTH'] = 16 * 1024 * 1024  # 16MB max upload size

def allowed_file(filename):
    return '.' in filename and filename.rsplit('.', 1)[1].lower() in ALLOWED_EXTENSIONS

def cleanup_old_files():
    now = time.time()
    cutoff = now - (24 * 3600)  # 24 hours
    for folder in [UPLOAD_FOLDER, PROCESSED_FOLDER]:
        for filepath in glob.glob(f"{folder}/*"):
            if os.path.getmtime(filepath) < cutoff:
                os.remove(filepath)

def process_image(img, operation, percentage, width, height, quality, resize_method):
    original_width, original_height = img.size
    
    if operation == 'percentage' and percentage:
        new_width = int(original_width * (percentage / 100))
        new_height = int(original_height * (percentage / 100))
    elif operation == 'dimensions' and width and height:
        new_width = width
        new_height = height
    else:
        raise ValueError('Invalid parameters')
    
    return img.resize((new_width, new_height), RESIZE_METHODS[resize_method])

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
                filename = secure_filename(file.filename)
                filepath = os.path.join(app.config['UPLOAD_FOLDER'], filename)
                file.save(filepath)

                img = Image.open(filepath)
                if img.mode in ('RGBA', 'P'):  # Convert to RGB if needed
                    img = img.convert('RGB')
                
                resized_img = process_image(img, operation, percentage, width, height, 
                                        quality, resize_method)
                
                processed_filename = f"processed_{int(time.time())}_{filename.rsplit('.', 1)[0]}.{output_format.lower()}"
                processed_path = os.path.join(app.config['PROCESSED_FOLDER'], processed_filename)
                
                save_params = {'quality': quality} if output_format == 'JPEG' else {}
                resized_img.save(processed_path, format=output_format, **save_params)

                processed_files.append({
                    'original': filename,
                    'processed': processed_filename,
                    'original_size': f"{img.size[0]}x{img.size[1]}",
                    'new_size': f"{resized_img.size[0]}x{resized_img.size[1]}"
                })

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