#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$repo_root"

WAS_RUNNING=0
PARTIAL_ARCHIVE=
RESTORE_COPY=
TEMP_LIST=
TEMP_VERBOSE=
SELF_TEST_DIRECTORY=
SELF_TEST_ACTIVE=0

usage() {
    cat >&2 <<'EOF'
Usage:
  scripts/ferry-data.sh backup ARCHIVE.tar.gz
  scripts/ferry-data.sh restore ARCHIVE.tar.gz SAFETY-BACKUP.tar.gz
  scripts/ferry-data.sh self-test

Restore accepts only Ferry's database files and server-generated blob names.
It writes the safety backup before replacing the Compose volume.
EOF
    exit 2
}

fail() {
    echo "ferry-data: $*" >&2
    exit 1
}

compose() {
    docker compose --project-directory "$repo_root" "$@"
}

absolute_file_parts() {
    requested=$1
    directory=$(dirname "$requested")
    filename=$(basename "$requested")
    [ -d "$directory" ] || fail "directory does not exist: $directory"
    case "$filename" in
        ''|.|..) fail "invalid archive name: $filename" ;;
    esac
    ARCHIVE_DIRECTORY=$(CDPATH= cd -- "$directory" && pwd)
    ARCHIVE_FILENAME=$filename
}

service_is_running() {
    compose ps --status running --services | awk '$0 == "ferry" { found = 1 } END { exit !found }'
}

stop_for_snapshot() {
    WAS_RUNNING=0
    if service_is_running; then
        WAS_RUNNING=1
        compose stop ferry
    fi
}

resume_service() {
    if [ "${WAS_RUNNING:-0}" -eq 1 ]; then
        compose start ferry
        WAS_RUNNING=0
    fi
}

cleanup() {
    status=$?
    trap - EXIT HUP INT TERM
    [ -z "$PARTIAL_ARCHIVE" ] || rm -f "$PARTIAL_ARCHIVE"
    [ -z "$RESTORE_COPY" ] || rm -f "$RESTORE_COPY"
    [ -z "$TEMP_LIST" ] || rm -f "$TEMP_LIST"
    [ -z "$TEMP_VERBOSE" ] || rm -f "$TEMP_VERBOSE"
    resume_service || true
    if [ "$SELF_TEST_ACTIVE" -eq 1 ]; then
        compose down --volumes >/dev/null 2>&1 || true
    fi
    [ -z "$SELF_TEST_DIRECTORY" ] || rm -rf "$SELF_TEST_DIRECTORY"
    exit "$status"
}

archive_volume() {
    destination=$1
    absolute_file_parts "$destination"
    [ ! -e "$ARCHIVE_DIRECTORY/$ARCHIVE_FILENAME" ] || fail "archive already exists: $destination"
    destination_directory=$ARCHIVE_DIRECTORY
    destination_filename=$ARCHIVE_FILENAME
    temporary_name=".$ARCHIVE_FILENAME.tmp.$$"
    PARTIAL_ARCHIVE="$ARCHIVE_DIRECTORY/$temporary_name"
    stop_for_snapshot
    compose run --rm --no-deps -T --user 0:0 \
        -v "$ARCHIVE_DIRECTORY:/backup" --entrypoint sh ferry \
        -c 'umask 077; tar -C /data -czf "/backup/$1" .' sh "$temporary_name"
    validate_archive "$PARTIAL_ARCHIVE" || fail "created backup contains unsupported Ferry data"
    mv "$PARTIAL_ARCHIVE" "$destination_directory/$destination_filename"
    PARTIAL_ARCHIVE=
    resume_service
    echo "backup written: $destination_directory/$destination_filename"
}

validate_archive() {
    source_archive=$1
    absolute_file_parts "$source_archive"
    [ -f "$ARCHIVE_DIRECTORY/$ARCHIVE_FILENAME" ] || fail "archive does not exist: $source_archive"
    list_file=$(mktemp "${TMPDIR:-/tmp}/ferry-archive-list.XXXXXX")
    verbose_file=$(mktemp "${TMPDIR:-/tmp}/ferry-archive-verbose.XXXXXX")
    TEMP_LIST=$list_file
    TEMP_VERBOSE=$verbose_file
    if ! compose run --rm --no-deps -T --user 0:0 \
        -v "$ARCHIVE_DIRECTORY:/backup:ro" --entrypoint sh ferry \
        -c 'tar -tzf "/backup/$1"' sh "$ARCHIVE_FILENAME" >"$list_file"; then
        echo "ferry-data: archive cannot be listed" >&2
        return 1
    fi
    if ! compose run --rm --no-deps -T --user 0:0 \
        -v "$ARCHIVE_DIRECTORY:/backup:ro" --entrypoint sh ferry \
        -c 'tar -tvzf "/backup/$1"' sh "$ARCHIVE_FILENAME" >"$verbose_file"; then
        echo "ferry-data: archive entry types cannot be inspected" >&2
        return 1
    fi
    awk '
        {
            seen = 1
            path = $0
            sub(/^\.\//, "", path)
            if (path == "" || path == "ferry.db" || path == "ferry.db-wal" ||
                path == "ferry.db-shm" || path == "blobs" || path == "blobs/") next
            if (index(path, "blobs/") == 1) {
                name = substr(path, 7)
                hex = substr(name, 1, 32)
                if (length(name) == 37 && substr(name, 33) == ".blob" && hex !~ /[^0-9a-f]/) next
            }
            print "unexpected archive entry: " $0 > "/dev/stderr"
            bad = 1
        }
        END { exit bad || !seen }
    ' "$list_file" || {
        echo "ferry-data: archive is not a Ferry data backup" >&2
        return 1
    }
    awk 'substr($1, 1, 1) != "-" && substr($1, 1, 1) != "d" { exit 1 }' "$verbose_file" ||
        {
            echo "ferry-data: archive contains a link or unsupported entry type" >&2
            return 1
        }
    rm -f "$list_file" "$verbose_file"
    TEMP_LIST=
    TEMP_VERBOSE=
}

restore_volume() {
    source_archive=$1
    safety_archive=$2
    absolute_file_parts "$source_archive"
    original_source="$ARCHIVE_DIRECTORY/$ARCHIVE_FILENAME"
    [ -f "$original_source" ] || fail "archive does not exist: $source_archive"
    absolute_file_parts "$safety_archive"
    safety_directory=$ARCHIVE_DIRECTORY
    safety_filename=$ARCHIVE_FILENAME
    [ ! -e "$safety_directory/$safety_filename" ] || fail "safety backup already exists: $safety_archive"

    restore_copy_name=".ferry-restore-source.$$.tar.gz"
    RESTORE_COPY="$safety_directory/$restore_copy_name"
    (umask 077; cp "$original_source" "$RESTORE_COPY")
    validate_archive "$RESTORE_COPY" || fail "restore archive validation failed"

    temporary_safety=".$safety_filename.tmp.$$"
    PARTIAL_ARCHIVE="$safety_directory/$temporary_safety"
    stop_for_snapshot
    compose run --rm --no-deps -T --user 0:0 \
        -v "$safety_directory:/safety" --entrypoint sh ferry \
        -c 'umask 077; tar -C /data -czf "/safety/$1" .' sh "$temporary_safety"
    validate_archive "$PARTIAL_ARCHIVE" || fail "safety backup contains unsupported Ferry data"
    mv "$PARTIAL_ARCHIVE" "$safety_directory/$safety_filename"
    PARTIAL_ARCHIVE=
    restart_after_restore=$WAS_RUNNING
    WAS_RUNNING=0
    compose run --rm --no-deps -T --user 0:0 \
        -v "$safety_directory:/archives:ro" --entrypoint sh ferry \
        -c 'set -eu; find /data -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +; tar -xzf "$1/$2" -C /data; chown -R 10001:10001 /data' \
        sh /archives "$restore_copy_name"
    rm -f "$RESTORE_COPY"
    RESTORE_COPY=
    if [ "$restart_after_restore" -eq 1 ]; then
        compose start ferry
    fi
    echo "restore complete; previous data: $safety_directory/$safety_filename"
}

self_test() {
    test_directory=$(mktemp -d "${TMPDIR:-/tmp}/ferry-data-test.XXXXXX")
    SELF_TEST_DIRECTORY=$test_directory
    COMPOSE_PROJECT_NAME="ferry-data-test-$$"
    export COMPOSE_PROJECT_NAME
    SELF_TEST_ACTIVE=1

    compose build ferry >/dev/null
    compose run --rm --no-deps -T --user 0:0 --entrypoint sh ferry -c '
        mkdir -p /data/blobs
        printf original-db > /data/ferry.db
        printf original-blob > /data/blobs/0123456789abcdef0123456789abcdef.blob
        printf reject-me > /data/unexpected
        chown -R 10001:10001 /data
    '
    if "$repo_root/scripts/ferry-data.sh" backup "$test_directory/rejected.tar.gz" >/dev/null 2>&1; then
        fail "backup with unsupported volume data was accepted"
    fi
    [ ! -e "$test_directory/rejected.tar.gz" ] || fail "rejected backup left a published archive"
    compose run --rm --no-deps -T --user 0:0 --entrypoint sh ferry -c 'rm -f /data/unexpected'
    archive_volume "$test_directory/backup.tar.gz"
    if "$repo_root/scripts/ferry-data.sh" backup "$test_directory/backup.tar.gz" >/dev/null 2>&1; then
        fail "existing backup was overwritten"
    fi
    compose run --rm --no-deps -T --user 0:0 --entrypoint sh ferry -c '
        printf changed > /data/ferry.db
        rm -f /data/blobs/0123456789abcdef0123456789abcdef.blob
    '
    restore_volume "$test_directory/backup.tar.gz" "$test_directory/safety.tar.gz"
    compose run --rm --no-deps -T --entrypoint sh ferry -c '
        [ "$(cat /data/ferry.db)" = original-db ]
        [ "$(cat /data/blobs/0123456789abcdef0123456789abcdef.blob)" = original-blob ]
    '
    mkdir "$test_directory/hostile"
    printf no >"$test_directory/hostile/unexpected"
    tar -C "$test_directory/hostile" -czf "$test_directory/hostile.tar.gz" unexpected
    if validate_archive "$test_directory/hostile.tar.gz" 2>/dev/null; then
        fail "hostile archive was accepted"
    fi
    mkdir "$test_directory/hostile-link"
    ln -s /tmp "$test_directory/hostile-link/ferry.db"
    tar -C "$test_directory/hostile-link" -czf "$test_directory/hostile-link.tar.gz" ferry.db
    if validate_archive "$test_directory/hostile-link.tar.gz" 2>/dev/null; then
        fail "archive symlink was accepted"
    fi
    printf not-a-tar >"$test_directory/corrupt.tar.gz"
    if validate_archive "$test_directory/corrupt.tar.gz" 2>/dev/null; then
        fail "corrupt archive was accepted"
    fi
    compose run --rm --no-deps -T --entrypoint sh ferry -c '[ "$(cat /data/ferry.db)" = original-db ]'
    echo "ferry data backup/restore self-test passed"
}

trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

case "${1:-}" in
    backup)
        [ "$#" -eq 2 ] || usage
        archive_volume "$2"
        ;;
    restore)
        [ "$#" -eq 3 ] || usage
        restore_volume "$2" "$3"
        ;;
    self-test)
        [ "$#" -eq 1 ] || usage
        self_test
        ;;
    *) usage ;;
esac
