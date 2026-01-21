#!/usr/bin/env python3
"""
Глубокий анализ MOBI файлов для сравнения fb2c и Calibre output.
"""

import struct
import sys

def parse_palmdb_header(data):
    """Парсит PalmDB header"""
    header = {}

    # Database name (offset 0-31)
    header['database_name'] = data[0:32].rstrip(b'\x00').decode('utf-8', errors='ignore')

    # Attributes (offset 32-33)
    header['attributes'] = struct.unpack('>H', data[32:34])[0]

    # Version (offset 34-35)
    header['version'] = struct.unpack('>H', data[34:36])[0]

    # Creation date (offset 36-39)
    header['creation_date'] = struct.unpack('>I', data[36:40])[0]

    # Modification date (offset 40-43)
    header['modification_date'] = struct.unpack('>I', data[40:44])[0]

    # Last backup date (offset 44-47)
    header['last_backup_date'] = struct.unpack('>I', data[44:48])[0]

    # Modification number (offset 48-51)
    header['modification_number'] = struct.unpack('>I', data[48:52])[0]

    # App info offset (offset 52-55)
    header['app_info_offset'] = struct.unpack('>I', data[52:56])[0]

    # Sort info offset (offset 56-59)
    header['sort_info_offset'] = struct.unpack('>I', data[56:60])[0]

    # Type (offset 60-63)
    header['type'] = data[60:64].decode('ascii', errors='ignore')

    # Creator (offset 64-67)
    header['creator'] = data[64:68].decode('ascii', errors='ignore')

    # Unique ID seed (offset 68-71)
    header['unique_id_seed'] = struct.unpack('>I', data[68:72])[0]

    # Next record list ID (offset 72-75)
    header['next_record_list_id'] = struct.unpack('>I', data[72:76])[0]

    # Number of records (offset 76-77)
    header['num_records'] = struct.unpack('>H', data[76:78])[0]

    return header

def parse_record_info_list(data, num_records):
    """Парсит Record Info List (начинается с offset 78)"""
    records = []
    offset = 78

    for i in range(num_records):
        rec_data = data[offset:offset+8]
        rec_offset = struct.unpack('>I', rec_data[0:4])[0]
        attrs = rec_data[4]
        rec_id = struct.unpack('>I', b'\x00' + rec_data[5:8])[0]

        records.append({
            'index': i,
            'offset': rec_offset,
            'attributes': attrs,
            'unique_id': rec_id
        })
        offset += 8

    return records

def parse_palmdoc_header(data):
    """Парсит PalmDOC header (первые 16 bytes Record 0)"""
    header = {}

    # Compression (offset 0-1)
    header['compression'] = struct.unpack('>H', data[0:2])[0]

    # Unused (offset 2-3)
    header['unused'] = struct.unpack('>H', data[2:4])[0]

    # Uncompressed text size (offset 4-7)
    header['uncompressed_text_size'] = struct.unpack('>I', data[4:8])[0]

    # Record count (offset 8-9)
    header['record_count'] = struct.unpack('>H', data[8:10])[0]

    # Record size (offset 10-13)
    header['record_size'] = struct.unpack('>I', data[10:14])[0]

    # Encryption type (offset 14-15)
    header['encryption_type'] = struct.unpack('>H', data[14:16])[0]

    return header

def parse_mobi_header(data):
    """Парсит MOBI header (начинается после PalmDOC header)"""
    header = {}

    # MOBI marker (offset 16-19)
    header['mobi_marker'] = data[16:20].decode('ascii', errors='ignore')

    if header['mobi_marker'] != 'MOBI':
        return None

    # Header length (offset 20-23)
    header['header_length'] = struct.unpack('>I', data[20:24])[0]

    # MOBI type (offset 24-27)
    header['mobi_type'] = struct.unpack('>I', data[24:28])[0]

    # Text encoding (offset 28-31)
    header['text_encoding'] = struct.unpack('>I', data[28:32])[0]

    # ID (offset 32-35)
    header['id'] = struct.unpack('>I', data[32:36])[0]

    # Format version (offset 36-39)
    header['format_version'] = struct.unpack('>I', data[36:40])[0]

    # First non-book index (offset 52-55)
    header['first_non_book_index'] = struct.unpack('>I', data[52:56])[0]

    # Full name offset (offset 56-59)
    header['full_name_offset'] = struct.unpack('>I', data[56:60])[0]

    # Full name length (offset 60-63)
    header['full_name_length'] = struct.unpack('>I', data[60:64])[0]

    # Locale (offset 64-67)
    header['locale'] = struct.unpack('>I', data[64:68])[0]

    # EXTH flags (offset 128-131)
    header['exth_flags'] = struct.unpack('>I', data[128:132])[0]

    # DRM offset (offset 152-155)
    header['drm_offset'] = struct.unpack('>I', data[152:156])[0]

    # DRM count (offset 156-159)
    header['drm_count'] = struct.unpack('>I', data[156:160])[0]

    # DRM flags (offset 164-167)
    header['drm_flags'] = struct.unpack('>I', data[164:168])[0]

    # First content record (offset 192-193)
    header['first_content_rec'] = struct.unpack('>H', data[192:194])[0]

    # Last content record (offset 194-195)
    header['last_content_rec'] = struct.unpack('>H', data[194:196])[0]

    return header

def parse_exth_header(data, exth_offset):
    """Парсит EXTH header"""
    header = {}

    # EXTH marker (offset 0-3)
    marker = data[exth_offset:exth_offset+4].decode('ascii', errors='ignore')

    if marker != 'EXTH':
        return None

    header['marker'] = marker

    # Header length (offset 4-7)
    header['header_length'] = struct.unpack('>I', data[exth_offset+4:exth_offset+8])[0]

    # Record count (offset 8-11)
    header['record_count'] = struct.unpack('>I', data[exth_offset+8:exth_offset+12])[0]

    return header

def parse_indx_header(data, offset):
    """Парсит INDX header (начинается с offset)"""
    header = {}
    
    # ID (offset 0-3)
    header['id'] = data[offset:offset+4].decode('ascii', errors='ignore')
    if header['id'] != 'INDX':
        return None

    # Header length (offset 4-7)
    header['header_length'] = struct.unpack('>I', data[offset+4:offset+8])[0]

    # Index Type (offset 12-15) - Corrected offset (0x0C)
    header['index_type'] = struct.unpack('>I', data[offset+12:offset+16])[0]

    # Unknown Offset / Gap (offset 16-19)
    header['unknown_gap'] = struct.unpack('>I', data[offset+16:offset+20])[0]

    # IDXT Offset (offset 20-23) - Corrected offset (0x14)
    header['idxt_offset'] = struct.unpack('>I', data[offset+20:offset+24])[0]

    # Count (offset 24-27)
    header['count'] = struct.unpack('>I', data[offset+24:offset+28])[0]
    
    # Encoding (offset 28-31)
    header['encoding'] = struct.unpack('>I', data[offset+28:offset+32])[0]
    
    # CNCX Count (offset 48-51)
    # The struct has been variable, but let's check standard offset 0x30
    header['cncx_count'] = struct.unpack('>I', data[offset+48:offset+52])[0]

    return header

def find_indx_record_index(records, data, indx_offset_in_header):
    """Находит Record ID для INDX"""
    # indx_offset_in_header - это field INDXRecordOffset в MOBI хедере (offset record index? or byte offset?)
    # В MOBI хедере INDXRecordOffset - это обычно Record Index (ID).
    # Но иногда это byte offset. Проверим.
    
    # Если это индекс записи (малое число)
    if indx_offset_in_header < len(records):
        return indx_offset_in_header
    
    # Иначе ищем по byte offset (не очень надежно)
    return -1

def analyze_mobi_file(filepath, name):
    """Полный анализ MOBI файла"""
    print(f"\n{'='*80}")
    print(f"Анализ: {name}")
    print(f"Файл: {filepath}")
    print(f"{'='*80}\n")

    with open(filepath, 'rb') as f:
        data = f.read()

    print(f"Размер файла: {len(data)} bytes\n")

    # PalmDB Header
    print("=== PALMDB HEADER ===")
    palmdb = parse_palmdb_header(data)
    for key, value in palmdb.items():
        if isinstance(value, bytes) and len(value) > 20:
            print(f"  {key}: {value[:20]}...")
        else:
            print(f"  {key}: {value}")

    # Record Info List
    print(f"\n=== RECORD INFO LIST (offset 78, {palmdb['num_records']} records) ===")
    records = parse_record_info_list(data, palmdb['num_records'])

    for rec in records[:10]:  # Показываем первые 10
        print(f"  Record {rec['index']}: offset={rec['offset']} (0x{rec['offset']:x}), "
              f"attrs=0x{rec['attributes']:02x}, id={rec['unique_id']}")

    if len(records) > 10:
        print(f"  ... и еще {len(records)-10} записей")

    # Record 0 - PalmDOC + MOBI
    if records:
        rec0_offset = records[0]['offset']
        print(f"\n=== RECORD 0 (offset {rec0_offset}/0x{rec0_offset:x}) ===")

        # PalmDOC header (first 16 bytes)
        palmdoc = parse_palmdoc_header(data[rec0_offset:rec0_offset+16])
        print("  PalmDOC Header (16 bytes):")
        for key, value in palmdoc.items():
            print(f"    {key}: {value}")

        # MOBI header (after PalmDOC)
        mobi = parse_mobi_header(data[rec0_offset:])
        if mobi:
            print(f"\n  MOBI Header (offset +16):")
            for key, value in mobi.items():
                if key == 'mobi_marker':
                    print(f"    {key}: {value}")
                elif isinstance(value, int) and value > 1000000:
                    print(f"    {key}: {value} (0x{value:x})")
                else:
                    print(f"    {key}: {value}")

            # EXTH header
            exth_offset = rec0_offset + 16 + mobi['header_length']
            exth = parse_exth_header(data, exth_offset)
            if exth:
                print(f"\n  EXTH Header (offset {exth_offset}/0x{exth_offset:x}):")
                for key, value in exth.items():
                    print(f"    {key}: {value}")

    # Text records
    if len(records) > 1:
        print(f"\n=== TEXT RECORDS ===")
        for i in range(1, min(len(records), 5)):
            rec = records[i]
            start = rec['offset']
            # Следующая запись или конец файла
            end = records[i+1]['offset'] if i+1 < len(records) else len(data)
            size = end - start
            print(f"  Record {i}: offset={start}, size={size} bytes, "
                  f"preview={data[start:start+20].hex()}")

    # Analyze INDX if present (from MOBI header 0xF4 = 244)
    if records and len(data) > records[0]['offset'] + 248:
        # INDX Record Offset is at 244 in MOBI header (0xF4)
        rec0_start = records[0]['offset']
        # Check header length first (offset 20)
        hdr_len = struct.unpack('>I', data[rec0_start+20:rec0_start+24])[0]
        # INDX offset is at rec0_start + 16 (marker) + 228 (F4 relative to marker 10?)
        # Start of MOBI header is rec0_start + 16
        # Field 0xF4 is at offset 0xE4 from MOBI header start
        # So absolute: rec0_start + 16 + 228 = rec0_start + 244
        
        indx_rec_idx = struct.unpack('>I', data[rec0_start+244:rec0_start+248])[0]
        print(f"\n=== INDX RECORD POINTER (0xF4): {indx_rec_idx} ===")
        
        if indx_rec_idx != 0xFFFFFFFF and indx_rec_idx < len(records):
            indx_offset = records[indx_rec_idx]['offset']
            print(f"  Parsing INDX Record {indx_rec_idx} at offset {indx_offset}:")
            indx_header = parse_indx_header(data, indx_offset)
            if indx_header:
                for k,v in indx_header.items():
                    print(f"    {k}: {v}")
                
            # Try to find Secondary INDX (often next record)
            sec_idx = indx_rec_idx + 1
            if sec_idx < len(records):
                sec_offset = records[sec_idx]['offset']
                print(f"  Checking Candidate Secondary INDX Record {sec_idx} at offset {sec_offset}:")
                sec_header = parse_indx_header(data, sec_offset)
                if sec_header:
                    for k,v in sec_header.items():
                        print(f"    {k}: {v}")
                    
                    # Dump TAGX/IDXT/Content details if possible
                    # We need to know where TAGX/IDXT start.
                    # Primary INDX (Rec 340) usually has TAGX.
                    # Secondary INDX (Rec 341) usually has Entries.
            
            # Dump Primary Content (TAGX)
            # TAGX follows header (192 bytes)
            if indx_header['header_length'] == 192:
                print(f"  Dumping Primary Record Content (offset {indx_offset+192}):")
                tagx_start = indx_offset + 192
                if data[tagx_start:tagx_start+4] == b'TAGX':
                    print("    Found TAGX marker!")
                    tagx_len = struct.unpack('>I', data[tagx_start+4:tagx_start+8])[0]
                    print(f"    TAGX Length: {tagx_len}")
                    # Dump TAGX data (simple hex dump)
                    print(f"    TAGX Data: {data[tagx_start:tagx_start+tagx_len+12].hex()}")
                else:
                    print(f"    No TAGX marker found at {tagx_start}. First 4 bytes: {data[tagx_start:tagx_start+4].hex()}")

            # Dump Secondary Content (Entries)
            # Secondary INDX (index_type 1) usually has entries
            if sec_header and sec_header['index_type'] == 1:
                print(f"  Dumping Secondary Entries (Rec {sec_idx}):")
                # Parse IDXT to get entry offsets
                # idxt_offset is relative to start of record?
                # The field 'idxt_offset' in header is relative to record start.
                
                idxt_absolute_start = sec_offset + sec_header['idxt_offset']
                count = sec_header['count']
                
                print(f"    Reading IDXT at +{sec_header['idxt_offset']} (abs {idxt_absolute_start}), Count {count}")
                
                # Check bounds
                if idxt_absolute_start + (count*2) <= len(data):
                    entry_offsets = []
                    for i in range(count):
                        e_off = struct.unpack('>H', data[idxt_absolute_start + (i*2) : idxt_absolute_start + (i*2) + 2])[0]
                        entry_offsets.append(e_off)
                    
                    # Dump first 3 entries
                    for i in range(min(3, count)):
                        start_rel = entry_offsets[i]
                        start_abs = sec_offset + start_rel
                        # Length is diff between this and next offset (or start of IDXT)
                        end_pos = 0
                        if i < count - 1:
                            end_pos = sec_offset + entry_offsets[i+1]
                        else:
                            end_pos = idxt_absolute_start
                        
                        entry_len = end_pos - start_abs
                        entry_data = data[start_abs:end_pos]
                        print(f"    Entry {i} (Offset {start_rel}): {entry_data.hex()}")
                else:
                    print("    IDXT start out of bounds")

if __name__ == '__main__':
    if len(sys.argv) < 3:
        print("Usage: python3 analyze_mobi.py <fb2c_file> <calibre_file>")
        sys.exit(1)

    fb2c_file = sys.argv[1]
    calibre_file = sys.argv[2]

    analyze_mobi_file(fb2c_file, "fb2c output")
    analyze_mobi_file(calibre_file, "Calibre output")

    print(f"\n{'='*80}")
    print("СРАВНЕНИЕ КЛЮЧЕВЫХ РАЗЛИЧИЙ")
    print(f"{'='*80}\n")

    # Сравнение
    with open(fb2c_file, 'rb') as f:
        fb2c_data = f.read()
    with open(calibre_file, 'rb') as f:
        calibre_data = f.read()

    fb2c_palmdb = parse_palmdb_header(fb2c_data)
    calibre_palmdb = parse_palmdb_header(calibre_data)

    fb2c_records = parse_record_info_list(fb2c_data, fb2c_palmdb['num_records'])
    calibre_records = parse_record_info_list(calibre_data, calibre_palmdb['num_records'])

    # PalmDOC comparison
    fb2c_palmdoc = parse_palmdoc_header(fb2c_data[fb2c_records[0]['offset']:])
    calibre_palmdoc = parse_palmdoc_header(calibre_data[calibre_records[0]['offset']:])

    print("PalmDOC Header differences:")
    for key in ['compression', 'record_size', 'encryption_type']:
        fb2c_val = fb2c_palmdoc[key]
        calibre_val = calibre_palmdoc[key]
        diff = " ✗ DIFFER" if fb2c_val != calibre_val else " ✓"
        print(f"  {key}: fb2c={fb2c_val}, calibre={calibre_val}{diff}")

    # MOBI comparison
    fb2c_mobi = parse_mobi_header(fb2c_data[fb2c_records[0]['offset']:])
    calibre_mobi = parse_mobi_header(calibre_data[calibre_records[0]['offset']:])

    if fb2c_mobi and calibre_mobi:
        print("\nMOBI Header differences:")
        for key in ['format_version', 'first_content_rec', 'exth_flags']:
            fb2c_val = fb2c_mobi.get(key, 'N/A')
            calibre_val = calibre_mobi.get(key, 'N/A')
            diff = " ✗ DIFFER" if fb2c_val != calibre_val else " ✓"
            print(f"  {key}: fb2c={fb2c_val}, calibre={calibre_val}{diff}")
