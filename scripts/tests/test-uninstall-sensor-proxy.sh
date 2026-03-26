#!/usr/bin/env bash
#
# Smoke tests for scripts/uninstall-sensor-proxy.sh helper behavior.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
UNINSTALL_SCRIPT="${ROOT_DIR}/scripts/uninstall-sensor-proxy.sh"

if [[ ! -f "${UNINSTALL_SCRIPT}" ]]; then
  echo "uninstall-sensor-proxy.sh not found at ${UNINSTALL_SCRIPT}" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "${UNINSTALL_SCRIPT}"

failures=0

assert_success() {
  local desc="$1"
  shift
  if "$@"; then
    echo "[PASS] ${desc}"
    return 0
  else
    echo "[FAIL] ${desc}" >&2
    ((failures++))
    return 1
  fi
}

test_remove_managed_keys_from_authorized_keys_file_resolves_symlink_target() {
  local tmpdir
  tmpdir="$(mktemp -d)"
  local real_auth="${tmpdir}/real_authorized_keys"
  local linked_auth="${tmpdir}/authorized_keys"

  cat > "${real_auth}" <<'EOF'
ssh-ed25519 AAAAkeep keep
command="sensors -j" ssh-ed25519 AAAAmanaged # pulse-managed-key
command="sensors -j" ssh-ed25519 AAAAproxy # pulse-proxy-key
EOF
  ln -s "${real_auth}" "${linked_auth}"

  remove_managed_keys_from_authorized_keys_file "$(resolve_path "${linked_auth}")"

  if [[ "$(cat "${real_auth}")" != "ssh-ed25519 AAAAkeep keep" ]]; then
    rm -rf "${tmpdir}"
    return 1
  fi

  rm -rf "${tmpdir}"
}

test_cleanup_sensor_proxy_lines_in_conf_preserves_snapshot_sections() {
  local tmpdir
  tmpdir="$(mktemp -d)"
  local conf="${tmpdir}/101.conf"

  cat > "${conf}" <<'EOF'
arch: amd64
mp0: /run/pulse-sensor-proxy,mp=/mnt/pulse-proxy
lxc.mount.entry: /run/pulse-sensor-proxy mnt/pulse-proxy none bind,create=dir 0 0
rootfs: local-lvm:vm-101-disk-0,size=8G
[snapshot]
mp0: /run/pulse-sensor-proxy,mp=/mnt/pulse-proxy
lxc.mount.entry: /run/pulse-sensor-proxy mnt/pulse-proxy none bind,create=dir 0 0
EOF

  cleanup_sensor_proxy_lines_in_conf "${conf}" "$(main_section_snapshot_line "${conf}")"

  if awk 'BEGIN{snapshot=0} /^\[snapshot\]$/{snapshot=1} !snapshot {print}' "${conf}" | grep -q '^mp0: /run/pulse-sensor-proxy'; then
    rm -rf "${tmpdir}"
    return 1
  fi
  if awk 'BEGIN{snapshot=0} /^\[snapshot\]$/{snapshot=1} !snapshot {print}' "${conf}" | grep -q '^lxc\.mount\.entry: /run/pulse-sensor-proxy'; then
    rm -rf "${tmpdir}"
    return 1
  fi
  if ! awk 'f{print} /^\[snapshot\]$/{f=1}' "${conf}" | grep -q '^mp0: /run/pulse-sensor-proxy'; then
    rm -rf "${tmpdir}"
    return 1
  fi
  if ! awk 'f{print} /^\[snapshot\]$/{f=1}' "${conf}" | grep -q '^lxc\.mount\.entry: /run/pulse-sensor-proxy'; then
    rm -rf "${tmpdir}"
    return 1
  fi

  rm -rf "${tmpdir}"
}

main() {
  assert_success "managed SSH key cleanup follows symlink targets" test_remove_managed_keys_from_authorized_keys_file_resolves_symlink_target
  assert_success "config cleanup keeps snapshot-only sensor-proxy entries intact" test_cleanup_sensor_proxy_lines_in_conf_preserves_snapshot_sections

  if (( failures > 0 )); then
    echo "Total failures: ${failures}" >&2
    return 1
  fi

  echo "All uninstall-sensor-proxy smoke tests passed."
}

main "$@"
