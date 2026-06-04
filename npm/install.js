#!/usr/bin/env node
/**
 * AI Novel Agent — npm launcher
 *
 * Two modes:
 *   1. postinstall — npm triggers this after install/link.
 *      Copies the Go binary to the global bin directory.
 *      If no binary exists (local dev without Go compilation), prints a
 *      friendly message and exits 0 so npm link still succeeds.
 *
 *   2. runtime — user runs `novel-agent <args>`.
 *      Finds the real binary (npm global install, local dist/, or PATH)
 *      and spawns it with the user's arguments.
 *
 * Platform detection:
 *   - darwin/amd64  → macOS Intel
 *   - darwin/arm64  → macOS Apple Silicon
 *   - linux/amd64   → Linux x86_64
 *   - windows/amd64 → Windows x64
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const { execFileSync } = require("child_process");

const PLATFORM_MAP = {
  "darwin-x64":   "novel-agent_darwin_amd64",
  "darwin-arm64": "novel-agent_darwin_arm64",
  "linux-x64":    "novel-agent_linux_amd64",
  "win32-x64":    "novel-agent_windows_amd64.exe",
};

function detectPlatform() {
  const platform = os.platform();
  const arch = os.arch();
  const key = `${platform}-${arch}`;
  const binary = PLATFORM_MAP[key];
  if (!binary) {
    console.error(
      `ai-novel-agent: unsupported platform: ${key}\n` +
      `Supported: darwin-x64, darwin-arm64, linux-x64, win32-x64`
    );
    process.exit(1);
  }
  return { platform, arch, binary };
}

// ---------------------------------------------------------------------------
// Find the real Go binary
// ---------------------------------------------------------------------------

function findBinary() {
  const { binary } = detectPlatform();
  const ext = process.platform === "win32" ? ".exe" : "";

  // 1. npm global install target (prefix/bin/)
  const prefix = process.env.npm_config_prefix
    || path.resolve(os.homedir(), ".npm-global");
  const globalPath = path.join(prefix, "bin", `novel-agent${ext}`);
  if (fs.existsSync(globalPath)) return globalPath;

  // 2. Local dist/ directory (development — npm link context)
  const localDist = path.join(__dirname, "dist", binary);
  if (fs.existsSync(localDist)) return localDist;

  // 3. Project root dist/ (from go build or make build)
  const rootDist = path.join(__dirname, "..", "dist", binary);
  if (fs.existsSync(rootDist)) return rootDist;

  // 4. PATH search (e.g. go install)
  const pathExts = (process.env.PATHEXT || "")
    .split(";")
    .map((e) => e.toLowerCase());
  const pathDirs = (process.env.PATH || "").split(path.delimiter);
  for (const dir of pathDirs) {
    const candidate = path.join(dir, "novel-agent");
    if (fs.existsSync(candidate)) return candidate;
    for (const ext of pathExts) {
      if (fs.existsSync(candidate + ext)) return candidate + ext;
    }
  }

  return null;
}

// ---------------------------------------------------------------------------
// postinstall — copy binary to global bin
// ---------------------------------------------------------------------------

function install() {
  const { binary } = detectPlatform();
  const srcDir = path.join(__dirname, "dist");
  const srcPath = path.join(srcDir, binary);

  if (!fs.existsSync(srcPath)) {
    console.log("");
    console.log("  ┌─────────────────────────────────────────────────────┐");
    console.log("  │  AI Novel Agent — development mode                  │");
    console.log("  │                                                     │");
    console.log("  │  Pre-compiled binary not found in npm/dist/.        │");
    console.log("  │  This is normal if you are linked locally.          │");
    console.log("  │                                                     │");
    console.log("  │  To build the binary:                               │");
    console.log("  │    go build -o npm/dist/novel-agent_windows_amd64   │");
    console.log("  │        ./cmd/novel-agent/                           │");
    console.log("  │                                                     │");
    console.log("  │  Or use Makefile:                                   │");
    console.log("  │    make build                                       │");
    console.log("  └─────────────────────────────────────────────────────┘");
    console.log("");
    return; // exit 0 — allow npm link to succeed
  }

  // Install to global bin
  const prefix = process.env.npm_config_prefix
    || path.resolve(os.homedir(), ".npm-global");
  const binDir = path.join(prefix, "bin");
  const ext = process.platform === "win32" ? ".exe" : "";
  const targetPath = path.join(binDir, `novel-agent${ext}`);

  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  fs.copyFileSync(srcPath, targetPath);
  if (process.platform !== "win32") {
    fs.chmodSync(targetPath, 0o755);
  }

  console.log(`✓ ai-novel-agent v${require("./package.json").version} installed`);
  console.log(`  Binary: ${targetPath}`);
  console.log(`  Run 'novel-agent init' to get started.`);
}

// ---------------------------------------------------------------------------
// runtime — user ran `novel-agent <args>`
// ---------------------------------------------------------------------------

function run(args) {
  const binPath = findBinary();
  if (!binPath) {
    console.error("ai-novel-agent: Go binary not found.");
    console.error("");
    console.error("  The binary needs to be compiled from source.");
    console.error("  Install Go 1.22+ from https://go.dev/dl/ then run:");
    console.error("");
    console.error("    cd ai-novel-matrix-studio");
    console.error("    go build -o novel-agent.exe ./cmd/novel-agent/");
    console.error("");
    console.error("  Then place novel-agent.exe in your PATH.");
    process.exit(1);
  }

  try {
    execFileSync(binPath, args, { stdio: "inherit" });
  } catch (err) {
    if (err.status) {
      process.exit(err.status);
    }
    console.error("ai-novel-agent: failed to run binary:", err.message);
    process.exit(1);
  }
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

// npm postinstall sets npm_config_global or runs from lifecycle scripts
const isPostInstall =
  process.env.npm_lifecycle_event === "postinstall" ||
  process.env.npm_command === "install" ||
  process.env.npm_command === "link";

if (require.main === module) {
  if (isPostInstall) {
    install();
  } else {
    // User invoked `novel-agent` directly — strip node and script path
    const args = process.argv.slice(2);
    run(args);
  }
}

module.exports = { install, run, findBinary, detectPlatform };
