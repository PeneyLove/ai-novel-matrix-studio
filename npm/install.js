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
 *      Finds the real Go binary (npm global install, local dist/, or PATH)
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
  const binaryName = PLATFORM_MAP[key];
  if (!binaryName) {
    console.error(
      `ai-novel-agent: unsupported platform: ${key}\n` +
      `Supported: darwin-x64, darwin-arm64, linux-x64, win32-x64`
    );
    process.exit(1);
  }
  return { platform, arch, binaryName };
}

// ---------------------------------------------------------------------------
// Find the real Go binary (NOT npm wrapper scripts)
// ---------------------------------------------------------------------------

function isRealBinary(fpath) {
  try {
    const stat = fs.statSync(fpath);
    // Real Go binaries are > 1 MB. npm wrappers (shell scripts, .cmd) are < 10 KB.
    if (stat.size < 100 * 1024) return false;
    return true;
  } catch (_) {
    return false;
  }
}

function findBinary() {
  const { platform, binaryName } = detectPlatform();
  const exeExt = platform === "win32" ? ".exe" : "";

  // 1. npm global install target — the Go binary copied by postinstall
  const prefix = process.env.npm_config_prefix
    || path.resolve(os.homedir(), ".npm-global");
  const globalBin = path.join(prefix, "bin", `novel-agent${exeExt}`);
  if (isRealBinary(globalBin)) return globalBin;

  // 2. Local dist/ directory (development — npm link context)
  const localDist = path.join(__dirname, "dist", binaryName);
  if (isRealBinary(localDist)) return localDist;

  // 3. Project root dist/ (from go build or make build)
  const rootDist = path.join(__dirname, "..", "dist", binaryName);
  if (isRealBinary(rootDist)) return rootDist;

  // 4. PATH search — only match real binaries, not npm wrapper scripts
  const pathDirs = (process.env.PATH || "").split(path.delimiter);
  for (const dir of pathDirs) {
    const candidate = path.join(dir, `novel-agent${exeExt}`);
    if (isRealBinary(candidate)) return candidate;
  }

  return null;
}

// ---------------------------------------------------------------------------
// postinstall — copy binary to global bin
// ---------------------------------------------------------------------------

function install() {
  const { binaryName } = detectPlatform();
  const srcDir = path.join(__dirname, "dist");
  const srcPath = path.join(srcDir, binaryName);

  if (!fs.existsSync(srcPath)) {
    console.log("");
    console.log("  ╔═══════════════════════════════════════════════════╗");
    console.log("  ║  AI Novel Agent — development mode               ║");
    console.log("  ║                                                   ║");
    console.log("  ║  Pre-compiled Go binary not found.               ║");
    console.log("  ║  This is normal if you are developing locally.    ║");
    console.log("  ║                                                   ║");
    console.log("  ║  To compile from source:                          ║");
    console.log("  ║    cd ai-novel-matrix-studio                     ║");
    console.log("  ║    go build -o npm\\dist\\" + binaryName.padEnd(35) + " ║");
    console.log("  ║        .\\cmd\\novel-agent\\                        ║");
    console.log("  ║                                                   ║");
    console.log("  ║  Or:  make build   (cross-compiles all platforms) ║");
    console.log("  ╚═══════════════════════════════════════════════════╝");
    console.log("");
    return;
  }

  // Install to global bin
  const prefix = process.env.npm_config_prefix
    || path.resolve(os.homedir(), ".npm-global");
  const binDir = path.join(prefix, "bin");
  const exeExt = process.platform === "win32" ? ".exe" : "";
  const targetPath = path.join(binDir, `novel-agent${exeExt}`);

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
    console.error("╔═══════════════════════════════════════════════════╗");
    console.error("║  AI Novel Agent — Go binary not found            ║");
    console.error("║                                                   ║");
    console.error("║  The Go binary has not been compiled yet.          ║");
    console.error("║  Install Go 1.22+ from https://go.dev/dl/         ║");
    console.error("║  Then run these commands:                          ║");
    console.error("║                                                     ║");
    console.error("║    cd ai-novel-matrix-studio                       ║");
    console.error("║    go build -o novel-agent.exe .\\cmd\\novel-agent\\  ║");
    console.error("║                                                     ║");
    console.error("║  Then either:                                      ║");
    console.error("║    a) Add this folder to your PATH, or              ║");
    console.error("║    b) Move novel-agent.exe to a folder in PATH      ║");
    console.error("╚═══════════════════════════════════════════════════╝");
    process.exit(1);
  }

  try {
    execFileSync(binPath, args, { stdio: "inherit" });
  } catch (err) {
    if (err.status != null) {
      process.exit(err.status);
    }
    console.error("ai-novel-agent: failed to run binary:", err.message);
    process.exit(1);
  }
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

const isPostInstall =
  process.env.npm_lifecycle_event === "postinstall" ||
  process.env.npm_command === "install" ||
  process.env.npm_command === "link";

if (require.main === module) {
  if (isPostInstall) {
    install();
  } else {
    const args = process.argv.slice(2);
    run(args);
  }
}

module.exports = { install, run, findBinary, detectPlatform };
