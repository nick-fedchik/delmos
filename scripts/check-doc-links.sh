#!/usr/bin/env bash
# Перевірка відносних посилань у Markdown-документації DELMOS.
# Зовнішні посилання (http, https, mailto) не перевіряються.
set -euo pipefail

cd "$(dirname "$0")/.."

broken=0
checked=0

while IFS= read -r file; do
    dir="$(dirname "$file")"

    while IFS= read -r target; do
        case "$target" in
            http://*|https://*|mailto:*|\#*|"") continue ;;
        esac

        path="${target%%#*}"
        [[ -z "$path" ]] && continue

        # Декодування пробілів у шляхах виду My%20File.md
        path="${path//%20/ }"

        if [[ ! -e "$dir/$path" ]]; then
            printf '%s: непрацююче посилання -> %s\n' "$file" "$target"
            broken=$((broken + 1))
        fi
        checked=$((checked + 1))
    done < <(grep -oE '\]\([^)]+\)' "$file" | sed -E 's/^\]\(//; s/\)$//')
done < <(find . -name '*.md' -not -path '*/node_modules/*' -not -path '*/.git/*' -not -path '*/dist/*')

printf 'Перевірено відносних посилань: %d\n' "$checked"

if [[ "$broken" -gt 0 ]]; then
    printf 'Непрацюючих посилань: %d\n' "$broken" >&2
    exit 1
fi
