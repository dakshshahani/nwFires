#!/bin/zsh
# nwFires setup — installs prerequisites (gcloud, firebase CLI), the launcher
# scripts, and the `nwfires` shell alias.
#
# Usage: ./setup.sh            # check/install everything + Firebase ADC login
#        ./setup.sh --build    # also force a binary rebuild
#        ./setup.sh --skip-requirements   # only install launchers + alias
set -euo pipefail

PROJ="$( CDPATH= cd -- "$( dirname -- "$0" )" && pwd )"
BIN="$HOME/bin"

# Find a working `go` (some PATHs contain unrelated executables named go).
find_go() {
  if command -v go >/dev/null 2>&1 && go version >/dev/null 2>&1; then
    command -v go
    return 0
  fi
  for cand in /opt/homebrew/bin/go /usr/local/bin/go; do
    if [[ -x "$cand" ]] && "$cand" version >/dev/null 2>&1; then
      echo "$cand"
      return 0
    fi
  done
  return 1
}
GO="$(find_go || true)"
FORCE_BUILD=0
SKIP_REQUIREMENTS=0
for arg in "$@"; do
  case "$arg" in
    --build) FORCE_BUILD=1 ;;
    --skip-requirements) SKIP_REQUIREMENTS=1 ;;
    *) echo "unknown option: $arg (use --build or --skip-requirements)" >&2; exit 1 ;;
  esac
done

[[ -d "$PROJ" ]] || { echo "error: project not found at $PROJ" >&2; exit 1; }

# Helper: ask yes/no. Non-interactive shells default to skipping interactive steps.
ask() {
  local prompt="$1"
  if [[ ! -t 0 ]]; then
    echo "  (non-interactive — skipping: $prompt)" >&2
    return 1
  fi
  local ans
  printf "%s [y/N] " "$prompt"
  read -r ans
  [[ "$ans" =~ ^[Yy]([Ee][Ss])?$ ]]
}

# Helper: install via brew if available, else tell the user.
brew_install() {
  local pkg_label="$1" formula="$2"
  if command -v brew >/dev/null 2>&1; then
    echo "Installing $pkg_label via Homebrew..."
    brew install "$formula"
  else
    echo "error: Homebrew not found. Install $pkg_label manually, then re-run setup.sh." >&2
    return 1
  fi
}

# --- 0. Requirements -------------------------------------------------------
if [[ $SKIP_REQUIREMENTS -eq 0 ]]; then
  echo "== Requirements =="

  # 0a. Go (needed to build the binary) — find_go falls back to the common
  # Homebrew locations if a broken or unrelated \`go\` shadows the real one.
  GO="$(find_go || true)"
  if [[ -z "$GO" ]]; then
    if ask "Go is not installed. Install it?"; then
      brew_install "Go" go
    else
      echo "  Skipping Go — the binary install below may fail." >&2
    fi
  else
    echo "✓ $("$GO" version)"
  fi

  # 0b. gcloud CLI (used for Firebase application-default credentials)
  if ! command -v gcloud >/dev/null 2>&1; then
    if ask "gcloud CLI is not installed. Install it?"; then
      brew_install "Google Cloud SDK (gcloud)" google-cloud-sdk
      # brew keeps the CLI outside PATH for the current session
      export PATH="$PATH:/opt/homebrew/Caskroom/google-cloud-sdk/latest/google-cloud-sdk/bin"
    else
      echo "  Skipping gcloud — Firebase auth will not work." >&2
    fi
  else
    echo "✓ gcloud $(gcloud --version | head -n1 | awk '{print $NF}')"
  fi

  # 0c. Firebase CLI
  if ! command -v firebase >/dev/null 2>&1; then
    if ask "Firebase CLI is not installed. Install it?"; then
      if command -v npm >/dev/null 2>&1; then
        echo "Installing Firebase CLI via npm..."
        npm install -g firebase-tools
      elif command -v brew >/dev/null 2>&1; then
        brew_install "Firebase CLI" firebase-cli
      else
        echo "error: Neither npm nor brew found. Install the Firebase CLI manually:" >&2
        echo "  curl -sL https://firebase.tools | bash" >&2
      fi
    else
      echo "  Skipping Firebase CLI — not strictly required for the TUI." >&2
    fi
  else
    echo "✓ firebase-tools $(firebase --version 2>/dev/null || echo installed)"
  fi

  # 0d. Firebase application-default credentials (what the TUI actually needs)
  if command -v gcloud >/dev/null 2>&1; then
    if ! gcloud auth application-default print-access-token >/dev/null 2>&1; then
      if ask "No Firebase credentials found (ADC). Run \`gcloud auth application-default login\` now?"; then
        gcloud auth application-default login
      else
        echo "  Skipping — run \`gcloud auth application-default login\` before launching." >&2
      fi
    else
      echo "✓ Firebase credentials (ADC) present"
    fi
  elif [[ $SKIP_REQUIREMENTS -eq 0 ]]; then
    echo "note: gcloud unavailable — \`gcloud auth application-default login\` will be needed later." >&2
  fi
fi

# --- 1. Build --------------------------------------------------------------
if [[ $FORCE_BUILD -eq 1 || ! -x "$PROJ/nwFires" ]]; then
  GO="$(find_go || true)"
  if [[ -z "$GO" ]]; then
    echo "error: cannot build — no working \`go\` (rerun setup.sh and accept the Go install, or check for a conflicting \`go\` on PATH)." >&2
    exit 1
  fi
  echo "Building nwFires..."
  ( cd "$PROJ" && "$GO" build -o nwFires . )
fi

# --- 2. Launcher scripts ---------------------------------------------------
mkdir -p "$BIN"

cat > "$BIN/hackcamp 2026" <<'EOF'
#!/bin/zsh
# HackCamp 2026 checker — find & edit HackCamp 2026 applicants (nwPlus UBC)
# Usage: "hackcamp 2026" (launches the TUI)
# Optional: "hackcamp 2026 --build" to rebuild from source
set -euo pipefail

PROJ="@PROJ@"

if [[ "${1:-}" == "--build" ]]; then
  cd "$PROJ"
  GO=go
  command -v go >/dev/null 2>&1 && go version >/dev/null 2>&1 || GO=/opt/homebrew/bin/go
  "$GO" build -o nwFires .
  echo "Built nwFires."
  exit 0
fi

exec "$PROJ/nwFires" "$@"
EOF

sed -e "s|@PROJ@|$PROJ|" -i.wf-temp "$BIN/hackcamp 2026" && rm -f "$BIN/hackcamp 2026.wf-temp"

cat > "$BIN/nwfires" <<'EOF'
#!/bin/zsh
exec "$HOME/bin/hackcamp 2026" "$@"
EOF

chmod +x "$BIN/hackcamp 2026" "$BIN/nwfires"

# --- 3. PATH check ---------------------------------------------------------
case ":$PATH:" in
  *":$BIN:"*) ;;
  *) echo "note: $BIN is not on your PATH" >&2 ;;
esac

# --- 4. Alias (idempotent) -------------------------------------------------
if [[ -f "$HOME/.zshrc" ]] && ! grep -q 'alias nwfires=' "$HOME/.zshrc"; then
  printf '\nalias nwfires="$HOME/bin/nwfires"\n' >> "$HOME/.zshrc"
  echo "Added alias to ~/.zshrc (run \`source ~/.zshrc\` in open shells)."
fi

echo "Setup complete — run \`nwfires\` to launch."
