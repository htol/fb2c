#!/bin/bash
# Автоматическая валидация FB2-to-MOBI конвертера
# Использует Calibre как эталон и mobitool для анализа

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
echo_ok() { echo -e "${GREEN}[OK]${NC} $1"; }
echo_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
echo_err() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check prerequisites
echo_info "Проверка зависимостей..."

MISSING_DEPS=0

if ! command -v ./fb2c &> /dev/null; then
    echo_err "fb2c не собран. Запустите: go build -o fb2c ./cmd/fb2c"
    MISSING_DEPS=1
fi

if ! command -v ebook-convert &> /dev/null; then
    echo_warn "Calibre (ebook-convert) не найден. Установите: sudo pacman -S calibre"
    MISSING_DEPS=1
fi

if ! command -v mobitool &> /dev/null; then
    echo_warn "mobitool (libmobi) не найден. Установите: yay -S libmobi"
    echo_warn "Продолжаем без mobitool - ограниченная валидация"
    HAVE_MOBITOOL=false
else
    HAVE_MOBITOOL=true
    echo_ok "mobitool найден"
fi

if [ $MISSING_DEPS -eq 1 ]; then
    echo_err "Установите все зависимости и перезапустите скрипт"
    exit 1
fi

# Create output directories
OUTPUT_DIR="$PROJECT_ROOT/validation_output"
REFERENCE_DIR="$OUTPUT_DIR/reference"
FB2C_DIR="$OUTPUT_DIR/fb2c"
EXTRACT_REF_DIR="$OUTPUT_DIR/extracted_reference"
EXTRACT_FB2C_DIR="$OUTPUT_DIR/extracted_fb2c"

mkdir -p "$OUTPUT_DIR" "$REFERENCE_DIR" "$FB2C_DIR" "$EXTRACT_REF_DIR" "$EXTRACT_FB2C_DIR"

echo_ok "Директории для валидации созданы"

# Find test files (from both testdata and testdata2)
TEST_FILES=()
for dir in testdata testdata2; do
    if [ -d "$dir" ]; then
        while IFS= read -r -d '' file; do
            TEST_FILES+=("$file")
        done < <(find "$dir" -maxdepth 1 -name "*.fb2" -type f -print0 2>/dev/null | sort -z)
    fi
done

if [ ${#TEST_FILES[@]} -eq 0 ]; then
    echo_err "Не найдено тестовых FB2 файлов в testdata/ или testdata2/"
    exit 1
fi

echo_info "Найдено ${#TEST_FILES[@]} тестовых файлов"

# Statistics
TOTAL_TESTS=0
PASSED_TESTS=0
WARNINGS=0

# Process each test file
for fb2_file in "${TEST_FILES[@]}"; do
    basename=$(basename "$fb2_file" .fb2)
    echo ""
    echo_info "=========================================="
    echo_info "Тест: $basename"
    echo_info "=========================================="

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    # Step 1: Convert with fb2c
    echo_info "[1/6] Конвертация через fb2c..."
    if ./fb2c convert "$fb2_file" "$FB2C_DIR/${basename}.mobi" 2>&1; then
        echo_ok "fb2c конвертация успешна"
    else
        echo_err "fb2c конвертация завершилась с ошибкой"
        continue
    fi

    # Step 2: Convert with Calibre (reference)
    echo_info "[2/6] Конвертация через Calibre (эталон)..."
    if ebook-convert "$fb2_file" "$REFERENCE_DIR/${basename}.mobi" \
        --mobi-file-type both \
        --pretty-print \
        > /dev/null 2>&1; then
        echo_ok "Calibre конвертация успешна"
    else
        echo_err "Calibre конвертация завершилась с ошибкой"
        continue
    fi

    # Step 3: Compare file sizes
    echo_info "[3/6] Сравнение размеров файлов..."

    fb2c_size=$(stat -c%s "$FB2C_DIR/${basename}.mobi" 2>/dev/null || stat -f%z "$FB2C_DIR/${basename}.mobi")
    ref_size=$(stat -c%s "$REFERENCE_DIR/${basename}.mobi" 2>/dev/null || stat -f%z "$REFERENCE_DIR/${basename}.mobi")
    ratio=$(awk "BEGIN {printf \"%.2f\", $fb2c_size / $ref_size}")

    echo "  fb2c:      $fb2c_size bytes"
    echo "  calibre:   $ref_size bytes"
    echo "  ratio:     $ratio"

    # Check if size is reasonable (within 10% to 500% of reference)
    if awk "BEGIN {exit !($ratio >= 0.1 && $ratio <= 5.0)}"; then
        echo_ok "Размер в разумных пределах"
    else
        echo_warn "Размер значительно отличается от эталона"
        WARNINGS=$((WARNINGS + 1))
    fi

    # Step 4: Extract and analyze structure (if mobitool available)
    if [ "$HAVE_MOBITOOL" = true ]; then
        echo_info "[4/6] Извлечение структуры MOBI..."

        # Extract fb2c output
        rm -rf "$EXTRACT_FB2C_DIR/${basename}"
        if mobitool -x "$FB2C_DIR/${basename}.mobi" -o "$EXTRACT_FB2C_DIR/${basename}" > /dev/null 2>&1; then
            echo_ok "fb2c MOBI структура извлечена"

            # Check for essential components
            if [ -f "$EXTRACT_FB2C_DIR/${basename}/mobi.html" ]; then
                html_lines=$(wc -l < "$EXTRACT_FB2C_DIR/${basename}/mobi.html")
                echo "  HTML lines: $html_lines"
            fi

            if [ -f "$EXTRACT_FB2C_DIR/${basename}/mobi.opf" ]; then
                echo "  ✓ OPF файл присутствует"
            else
                echo_warn "OPF файл отсутствует"
            fi

            # Count images
            img_count=$(find "$EXTRACT_FB2C_DIR/${basename}" -type f \( -name "*.jpg" -o -name "*.png" -o -name "*.gif" \) 2>/dev/null | wc -l)
            echo "  Images: $img_count"

        else
            echo_err "Не удалось извлечь fb2c MOBI"
        fi

        # Extract reference output
        rm -rf "$EXTRACT_REF_DIR/${basename}"
        if mobitool -x "$REFERENCE_DIR/${basename}.mobi" -o "$EXTRACT_REF_DIR/${basename}" > /dev/null 2>&1; then
            echo_ok "Calibre MOBI структура извлечена"

            # Compare HTML content
            if [ -f "$EXTRACT_FB2C_DIR/${basename}/mobi.html" ] && [ -f "$EXTRACT_REF_DIR/${basename}/mobi.html" ]; then
                fb2c_html_lines=$(wc -l < "$EXTRACT_FB2C_DIR/${basename}/mobi.html")
                ref_html_lines=$(wc -l < "$EXTRACT_REF_DIR/${basename}/mobi.html")
                html_ratio=$(awk "BEGIN {printf \"%.2f\", $fb2c_html_lines / $ref_html_lines}")
                echo "  HTML lines ratio: $html_ratio (fb2c=$fb2c_html_lines, ref=$ref_html_lines)"
            fi
        else
            echo_warn "Не удалось извлечь Calibre MOBI"
        fi
    else
        echo_info "[4/6] Пропуск (mobitool не установлен)"
    fi

    # Step 5: Validate PalmDB structure
    echo_info "[5/6] Проверка PalmDB структуры..."

    check_magic_string() {
        local file=$1
        local offset=$2
        local expected=$3
        local name=$4

        if [ -f "$file" ]; then
            actual=$(dd if="$file" bs=1 skip="$offset" count=4 2>/dev/null | tr -d '\0')
            if [ "$actual" = "$expected" ]; then
                echo "  ✓ $name: $actual"
                return 0
            else
                echo_err "  $name: got '$actual', expected '$expected'"
                return 1
            fi
        fi
        return 1
    }

    errors=0

    if ! check_magic_string "$FB2C_DIR/${basename}.mobi" 60 "BOOK" "PalmDB Type"; then
        errors=$((errors + 1))
    fi

    if ! check_magic_string "$FB2C_DIR/${basename}.mobi" 64 "MOBI" "PalmDB Creator"; then
        errors=$((errors + 1))
    fi

    # Check for MOBI header
    mobi_magic=$(dd if="$FB2C_DIR/${basename}.mobi" bs=1 skip=78 count=4 2>/dev/null | tr -d '\0')
    if [ "$mobi_magic" = "MOBI" ]; then
        echo "  ✓ MOBI header found"
    else
        echo_err "  MOBI header not found at offset 78"
        errors=$((errors + 1))
    fi

    if [ $errors -eq 0 ]; then
        echo_ok "PalmDB структура корректна"
    else
        echo_err "Найдено $errors ошибок в структуре"
    fi

    # Step 6: Metadata comparison
    echo_info "[6/6] Проверка метаданных..."

    if [ "$HAVE_MOBITOOL" = true ]; then
        echo "  fb2c metadata:"
        fb2c_meta=$(mobitool -m "$FB2C_DIR/${basename}.mobi" 2>/dev/null || echo "")

        if echo "$fb2c_meta" | grep -q "Title:"; then
            echo "$fb2c_meta" | grep "Title:" | sed 's/^/    /'
        else
            echo_warn "    Title not found"
        fi

        if echo "$fb2c_meta" | grep -q "Author:"; then
            echo "$fb2c_meta" | grep "Author:" | sed 's/^/    /'
        fi
    fi

    # Extract metadata using fb2c metadata command
    if ./fb2c metadata "$fb2_file" > "$OUTPUT_DIR/${basename}_metadata.txt" 2>&1; then
        echo "  ✓ fb2c может извлекать метаданные"
    fi

    # Test result
    if [ $errors -eq 0 ]; then
        echo_ok "Тест '$basename' PASSED"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo_err "Тест '$basename' FAILED"
    fi
done

# Final summary
echo ""
echo_info "=========================================="
echo_info "ИТОГОВАЯ СТАТИСТИКА"
echo_info "=========================================="
echo "Всего тестов:    $TOTAL_TESTS"
echo "Прошло:          $PASSED_TESTS"
echo "Предупреждений:  $WARNINGS"
echo ""

if [ $PASSED_TESTS -eq $TOTAL_TESTS ]; then
    echo_ok "Все тесты прошли успешно!"
    echo_info "Результаты сохранены в: $OUTPUT_DIR"
    exit 0
else
    echo_err "Некоторые тесты не прошли"
    echo_info "Проверьте вывод выше для деталей"
    exit 1
fi
