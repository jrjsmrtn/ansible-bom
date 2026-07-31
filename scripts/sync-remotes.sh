#!/bin/sh
# Propagate the current branch to every remote that already tracks it.
#
# This repository has more than one remote. A pull request merged on one forge lands only there,
# so the other remote silently falls behind — and `git pull <the stale remote>` then reports
# "Already up to date", which is true and completely misleading.
#
# Run from a post-merge hook: once a merge arrives locally, push it onward.
#
# Deliberate properties:
#   - fast-forward only; never force, never rewrite
#   - only updates branches the remote ALREADY has, so it cannot create branches by surprise
#   - always exits 0. A remote being unreachable must not fail the merge that just succeeded.
#   - names no remote and no host: it works on whatever remotes are configured.
#
# Skip with: ANSIBLE_BOM_NO_SYNC=1

set -u

[ "${ANSIBLE_BOM_NO_SYNC:-}" = "1" ] && exit 0
[ "${CI:-}" = "true" ] && exit 0

branch=$(git symbolic-ref --quiet --short HEAD 2>/dev/null) || exit 0
[ -n "$branch" ] || exit 0

local_sha=$(git rev-parse "$branch" 2>/dev/null) || exit 0

for remote in $(git remote); do
    # Only consider remotes that already publish this branch.
    remote_sha=$(git ls-remote --heads "$remote" "$branch" 2>/dev/null | cut -f1)
    [ -n "$remote_sha" ] || continue
    [ "$remote_sha" = "$local_sha" ] && continue

    # Push only when the remote is strictly behind us. If it has commits we lack, that is a
    # divergence for a human to resolve, not something a hook should paper over.
    if git merge-base --is-ancestor "$remote_sha" "$local_sha" 2>/dev/null; then
        printf 'sync: %s is behind, pushing %s\n' "$remote" "$branch" >&2
        if git push --quiet "$remote" "$branch" 2>/dev/null; then
            printf 'sync: %s updated to %s\n' "$remote" "$(git rev-parse --short "$local_sha")" >&2
        else
            printf 'sync: could not reach %s — push %s there when it is available\n' \
                "$remote" "$branch" >&2
        fi
    else
        printf 'sync: %s has commits not present locally; resolve by hand\n' "$remote" >&2
    fi
done

exit 0
