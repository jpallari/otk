#!/usr/bin/bash
set -euo pipefail

GIT_REPO_DIR=${GIT_REPO_DIR:-/srv/git}
SSH_DIR=/home/git/.ssh

# Anonymise git password so it doesn't show when
# settings are printed below.
if [ -n "${GIT_PASSWORD:-}" ]; then
    GIT_PASSWORD_PLACEHOLDER="<specified>"
elif [ -n "${GIT_PASSWORD_FILE:-}" ]; then
    GIT_PASSWORD_PLACEHOLDER="<file>"
else
    GIT_PASSWORD_PLACEHOLDER="<default>"
fi

# Print out settings
cat <<EOF >&2
Settings:
  GIT_PASSWORD=${GIT_PASSWORD_PLACEHOLDER}
  GIT_PASSWORD_FILE=${GIT_PASSWORD_FILE:-}
  GIT_REPO_DIR=${GIT_REPO_DIR}
  GIT_REPOS=${GIT_REPOS:-}
  SSH_AUTHORIZED_KEY=${SSH_AUTHORIZED_KEY:-}
  SSH_AUTHORIZED_KEYS_FILE=${SSH_AUTHORIZED_KEYS_FILE:-}
  SSH_AUTHORIZED_KEYS_URL=${SSH_AUTHORIZED_KEYS_URL:-}
EOF

log() {
    echo "$@" >&2
}

init_git_user() {
    if ! getent group git 2>/dev/null; then
        log "Creating group 'git'"
        addgroup git
    fi

    if ! id git 2>/dev/null; then
        log "Creating user 'git'"
        adduser \
            --ingroup git --disabled-password \
            --shell /usr/bin/git-shell \
            --gecos Git git
    fi

    if [ -n "${GIT_PASSWORD_FILE:-}" ]; then
        if [ -s "${GIT_PASSWORD_FILE}" ]; then
            log "Setting git password from file ${GIT_PASSWORD_FILE}"
            echo "git:$(cat "${GIT_PASSWORD_FILE}")" | chpasswd
        else
            log "File '${GIT_PASSWORD_FILE}' not found or empty"
            return 1
        fi
    else
        log "Using default password for git"
        echo "git:${GIT_PASSWORD:-abcd}" | chpasswd
    fi
}

init_ssh_credentials() {
    mkdir -p "${SSH_DIR}"
    touch "${SSH_DIR}/authorized_keys"

    if [ -n "${SSH_AUTHORIZED_KEY:-}" ]; then
        log "Copying authorized key from env"
        echo "${SSH_AUTHORIZED_KEY}" >> "${SSH_DIR}/authorized_keys"
    fi

    if [ -n "${SSH_AUTHORIZED_KEYS_FILE:-}" ]; then
        if [ -s "${SSH_AUTHORIZED_KEYS_FILE}" ]; then
            log "Copying authorized keys from ${SSH_AUTHORIZED_KEYS_FILE}"
            cat "${SSH_AUTHORIZED_KEYS_FILE}" >> "${SSH_DIR}/authorized_keys"
        else
            log "File '${SSH_AUTHORIZED_KEYS_FILE}' not found or empty"
            return 1
        fi
    fi

    if [ -n "${SSH_AUTHORIZED_KEYS_URL:-}" ]; then
        log "Copying authorized keys from ${SSH_AUTHORIZED_KEYS_URL}"
        curl -sSfL "${SSH_AUTHORIZED_KEYS_URL}" >> "${SSH_DIR}/authorized_keys"
    fi

    chown -R git:git "${SSH_DIR}"
    chmod 700 "${SSH_DIR}"
}

init_git_repos() {
    local git_repo git_repo_path
    if [ -z "${GIT_REPOS:-}" ]; then
        return
    fi

    mkdir -p "${GIT_REPO_DIR}"

    for git_repo in $GIT_REPOS; do
        git_repo_path="${GIT_REPO_DIR}/${git_repo}.git"
        mkdir -p "${git_repo_path}"
        git init --bare "${git_repo_path}" -b main
    done

    chown -R git:git "${GIT_REPO_DIR}"
}

init_sshd() {
    mkdir -p /run/sshd
    chmod 0755 /run/sshd
}

init_git_user
init_ssh_credentials
init_git_repos
init_sshd
exec "$@"
