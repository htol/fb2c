# Architecture

`fb2c` is a pipeline-based converter that transforms FB2 (FictionBook 2.0) files into MOBI or EPUB formats. The core design philosophy is to use an intermediate representation (OPF/OEB) to decouple the input parsing from the output generation.

## High-Level Data Flow

1. **Input**: The process starts with an `.fb2` file (XML based).
2. **Parsing (`fb2`)**: The file is parsed into a Go struct representation (`fb2.FictionBook`). This step handles XML unmarshalling and encoding normalization.
3. **Transformation (`fb2`)**: The structural FB2 data is transformed into HTML. This is necessary because both MOBI and EPUB use HTML/XHTML for their content. This step also extracts the Table of Contents (TOC).
4. **Intermediate Representation (`opf`)**: The metadata, HTML content, images, and TOC are assembled into an `opf.OEBBook`. This structure mirrors the Open eBook (OEB) standard, which is the predecessor to both MOBI and EPUB 2.0.
5. **Output Generation**:
    * **EPUB (`epub`)**: The `OEBBook` is packaged directly into an EPUB container (ZIP structure with specific manifests).
    * **MOBI (`mobi`)**: The `OEBBook` is converted into MOBI records. This involves compressing the HTML (PalmDOC), building the database records (PDB header, PalmDOC header, MOBI header), and generating the file structure.

## Key Components

### `cmd/fb2c`

The command-line interface. It handles argument parsing, flag management, and orchestration of the conversion process using the `fb2c.Converter` facade.

### `fb2`

* **Parser**: Handles XML parsing.
* **Transformer**: Converts FB2 XML structures into HTML.
* **Metadata**: Extracts and normalizes book metadata.

### `opf`

Defines the `OEBBook` structure, which serves as the "universal" book format within the application. It contains:

* Standardized Metadata (Dublin Core)
* Manifest (list of all files)
* Spine (reading order)
* TOC structure

### `mobi`

Handles the complexity of the MOBI format.

* **Compression**: Implements PalmDOC compression.
* **Record Building**: Constructs the specific binary records required by the MOBI format.
* **KF8 Support**: Includes support for the newer KF8 (Kindle Format 8) which is essentially EPUB-in-a-wrapper.

### `epub`

A lightweight EPUB writer that packages the `OEBBook` content into a valid EPUB zip file.

## Directory Structure

* `cmd/`: Application entry points.
* `fb2/b64/`: Base64 decoding utilities (for embedded images in FB2).
* `epub/`: EPUB generation logic.
* `fb2/`: FB2 parsing and transformation.
* `fb2encoding/`: Character encoding handling (often needed for older Russian FB2 files).
* `mobi/`: MOBI format generation.
* `opf/`: Intermediate Open eBook representation.
* `mobi/varint/`: Variable-length integer encoding (used in MOBI).
