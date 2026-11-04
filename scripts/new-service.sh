#!/usr/bin/env bash
set -euo pipefail

fail() { printf '%s\n' "$1" >&2; exit 1; }

[[ $# -eq 1 ]] || fail "Usage: $0 <service-name>"
service_name=$1
[[ $service_name =~ ^[a-z][a-z0-9]*(-[a-z0-9]+)*$ ]] || fail "Service name must use lowercase kebab-case."
[[ $service_name != service-template ]] || fail "The name service-template is reserved."

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(cd "$script_dir/.." && pwd -P)
template="$repo_root/services/service-template"
target="$repo_root/services/$service_name"
[[ -d $template && ! -L $template ]] || fail "Golden service template is missing or is a symlink."

check_target() {
    [[ ! -L $target ]] || fail "Refusing an existing symlink at the service destination."
    if [[ -e $target ]]; then
        [[ -d $target ]] || fail "Service destination already exists."
        # Only an empty directory or a single empty .gitkeep is a placeholder.
        local entry
        for entry in "$target"/* "$target"/.[!.]* "$target"/..?*; do
            [[ -e $entry || -L $entry ]] || continue
            [[ $entry == "$target/.gitkeep" && -f $entry && ! -L $entry && ! -s $entry ]] || fail "Service directory contains existing work; refusing to overwrite it."
        done
    fi
}
check_target

stage=$(mktemp -d "$repo_root/services/.new-service.XXXXXX")
cleanup() {
    local status=$?
    trap - EXIT
    if [[ -d $stage/placeholder ]]; then
        if [[ ! -e $target && ! -L $target ]]; then
            if ! mv "$stage/placeholder" "$target"; then
                printf 'Could not restore placeholder; preserved at %s\n' "$stage/placeholder" >&2
                exit 1
            fi
        fi
    fi
    rm -rf "$stage"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
mkdir "$stage/generated"
find "$template" -mindepth 1 -print0 > "$stage/files"
while IFS= read -r -d '' source; do
    relative=${source#"$template/"}
    # Copy source artifacts only; never copy environment files or secret stores.
    case "/$relative/" in
        */.env*/*|*/secret/*|*/secrets/*|*/credentials/*|*/.git/*) continue ;;
    esac
    [[ ! -L $source ]] || fail "Template symlinks are not supported."
    if [[ -d $source ]]; then
        mkdir -p "$stage/generated/$relative"
    elif [[ -f $source ]]; then
        case ${source##*/} in
            *.go|*.sql|Dockerfile|.gitkeep)
                mkdir -p "$(dirname "$stage/generated/$relative")"
                # The template name also occurs in tests, migration paths and binary names.
                sed "s/service-template/$service_name/g" "$source" > "$stage/generated/$relative"
                ;;
        esac
    else
        fail "Unsupported template file type."
    fi
done < "$stage/files"

# Finish all copying and rewriting before replacing an allowed placeholder.
check_target
if [[ -d $target ]]; then
    mv "$target" "$stage/placeholder"
fi
mv "$stage/generated" "$target"
printf 'Created services/%s\n' "$service_name"
