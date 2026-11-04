#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(cd "$script_dir/.." && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/jobtrack-new-service-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
trap 'exit 1' HUP INT TERM
fixture="$scratch/repository with spaces"
mkdir -p "$fixture/scripts" "$fixture/services" "$fixture/gen" "$fixture/pkg"
cp "$script_dir/new-service.sh" "$fixture/scripts/"
cp "$repo_root/go.mod" "$repo_root/go.sum" "$fixture/"
cp -R "$repo_root/gen/go" "$fixture/gen/"
cp -R "$repo_root/pkg/." "$fixture/pkg/"
cp -R "$repo_root/services/service-template" "$fixture/services/"
generator="$fixture/scripts/new-service.sh"
template="$fixture/services/service-template"

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
expect_failure() {
    if "$@" > "$scratch/expected-failure.log" 2>&1; then
        fail "Command unexpectedly succeeded."
    fi
}

# Synthetic fixtures ensure sensitive artifacts are excluded without changing the real template.
mkdir -p "$template/secrets" "$template/credentials"
printf 'synthetic-only\n' > "$template/.env"
printf 'synthetic-only\n' > "$template/internal/.env.local"
printf 'synthetic-only\n' > "$template/secrets/private.go"
printf 'synthetic-only\n' > "$template/credentials/private.sql"
printf 'synthetic-only\n' > "$template/private.key"
cp -R "$template" "$scratch/template-before"

expect_failure "$generator"
expect_failure "$generator" one two
for name in '' Service_Name Uppercase 'two words' '-leading' 'trailing-' 'double--dash' '../escape' 'name/child' '0name' 'service-template'; do
    expect_failure "$generator" "$name"
done
printf 'PASS: missing, invalid and reserved names rejected\n'

mkdir "$fixture/services/implemented"
printf 'existing source\n' > "$fixture/services/implemented/main.go"
cp -R "$fixture/services/implemented" "$scratch/implemented-before"
expect_failure "$generator" implemented
diff -r "$scratch/implemented-before" "$fixture/services/implemented"
printf 'PASS: implemented service preserved\n'

# Invoke from outside the repository, including a repository path containing spaces.
(cd "$scratch" && "$generator" sample-service)
generated="$fixture/services/sample-service"
[[ -f $generated/cmd/server/main.go && -f $generated/migrations/00001_initial.sql ]] || fail "Code or migrations missing."
for dir in domain ports app adapters/grpc adapters/postgres adapters/rabbitmq adapters/clients; do
    [[ -d $generated/internal/$dir ]] || fail "Hexagonal directory missing."
done
if grep -R -n 'service-template' "$generated"; then fail "Template references remain."; fi
grep -q 'services/sample-service/internal/app' "$generated/cmd/server/main.go"
grep -q 'services/sample-service/migrations' "$generated/cmd/server/main.go"
grep -q 'COPY services/sample-service/' "$generated/Dockerfile"
grep -q './services/sample-service/cmd/server' "$generated/Dockerfile"
grep -q 'ENV SERVICE_NAME=sample-service ' "$generated/Dockerfile"
[[ ! -e $generated/.env && ! -e $generated/internal/.env.local && ! -e $generated/secrets && ! -e $generated/credentials && ! -e $generated/private.key ]] || fail "Sensitive artifact copied."
printf 'PASS: generation, imports, Dockerfile, migration path and secret exclusions\n'

mkdir "$fixture/services/placeholder"
touch "$fixture/services/placeholder/.gitkeep"
"$generator" placeholder
[[ -f $fixture/services/placeholder/cmd/server/main.go ]] || fail "Placeholder not populated."
printf 'PASS: .gitkeep-only placeholder accepted\n'

# Fail rewriting before publication, then fail the final rename after moving a placeholder.
mkdir "$scratch/failing-tools"
printf '#!/usr/bin/env bash\nexit 1\n' > "$scratch/failing-tools/sed"
chmod +x "$scratch/failing-tools/sed"
expect_failure env PATH="$scratch/failing-tools:$PATH" "$generator" failed-copy
[[ ! -e $fixture/services/failed-copy ]] || fail "Failed rewriting left partial output."
rm "$scratch/failing-tools/sed"
mkdir "$fixture/services/restore-placeholder"
touch "$fixture/services/restore-placeholder/.gitkeep"
cat > "$scratch/failing-tools/mv" <<'SH'
#!/usr/bin/env bash
case $1 in */generated) exit 1 ;; esac
exec /bin/mv "$@"
SH
chmod +x "$scratch/failing-tools/mv"
expect_failure env PATH="$scratch/failing-tools:$PATH" "$generator" restore-placeholder
[[ -f $fixture/services/restore-placeholder/.gitkeep && ! -e $fixture/services/restore-placeholder/cmd ]] || fail "Placeholder was not restored."
if find "$fixture/services" -name '.new-service.*' | grep -q .; then fail "Temporary staging output remains."; fi
diff -r "$scratch/template-before" "$template"
printf 'PASS: failures leave no partial output; golden template unchanged\n'

(cd "$fixture" && go build -mod=readonly -o "$scratch/sample-server" ./services/sample-service/cmd/server)
(cd "$fixture" && go test -count=1 ./services/sample-service/...)
printf 'PASS: generated service builds and its unit tests pass in the existing module\n'
