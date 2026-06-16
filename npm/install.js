#!/usr/bin/env node
/**
 * AI Novel Agent — npm launcher
 *
 * Two modes:
 *   1. postinstall (npm link / npm install -g triggers this):
 *      Copies the Go binary from dist/ to the npm global bin directory.
 *
 *   2. runtime (user types `novel-agent`):
 *      Finds the real Go binary and execs it with user arguments.
 *
 * Platform mapping:
 *   darwin/amd64  → novel-agent_darwin_amd64
 *   darwin/arm64  → novel-agent_darwin_arm64
 *   linux/amd64   → novel-agent_linux_amd64
 *   win32/x64     → novel-agent_windows_amd64.exe
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const { execFileSync } = require("child_process");

const BINARY_MAP = {
  "darwin-x64":   "novel-agent_darwin_amd64",
  "darwin-arm64": "novel-agent_darwin_arm64",
  "linux-x64":    "novel-agent_linux_amd64",
  "linux-arm64":  "novel-agent_linux_arm64",
  "win32-x64":    "novel-agent_windows_amd64.exe",
};

// ---- helpers ----

function platformKey() {
  return `${os.platform()}-${os.arch()}`;
}

function isGoBinary(fpath) {
  try {
    const st = fs.statSync(fpath);
    if (!st.isFile()) return false;
    // Go binaries > 1 MB; npm wrapper scripts < 10 KB
    return st.size >= 512 * 1024;
  } catch (_) {
    return false;
  }
}

function npmBinDir() {
  // On Windows: C:\Users\X\AppData\Roaming\npm
  // On Unix: ~/.npm-global/bin or /usr/local/bin
  if (process.platform === "win32") {
    // npm on Windows puts binaries directly in prefix, no /bin subdir
    return process.env.npm_config_prefix
      || path.join(os.homedir(), "AppData", "Roaming", "npm");
  }
  return path.join(
    process.env.npm_config_prefix || path.join(os.homedir(), ".npm-global"),
    "bin"
  );
}

// ---- find binary ----

function findBinary() {
  const binName = BINARY_MAP[platformKey()];
  if (!binName) {
    console.error("novelAgent: unsupported platform:", platformKey());
    process.exit(1);
  }
  const exeExt = process.platform === "win32" ? ".exe" : "";

  // 1. npm global bin — where postinstall copied it
  const globalPath = path.join(npmBinDir(), `novel-agent${exeExt}`);
  if (isGoBinary(globalPath)) return globalPath;

  // 2. local npm/dist/ — development mode
  const localPath = path.join(__dirname, "dist", binName);
  if (isGoBinary(localPath)) return localPath;

  // 3. project root (go build -o novel-agent.exe .\cmd\novel-agent\)
  const rootExe = path.join(__dirname, "..", "..", `novel-agent${exeExt}`);
  if (isGoBinary(rootExe)) return rootExe;

  // 4. PATH search (only real binaries, skip npm wrappers)
  const dirs = (process.env.PATH || "").split(path.delimiter);
  for (const dir of dirs) {
    const candidate = path.join(dir, `novel-agent${exeExt}`);
    if (isGoBinary(candidate)) return candidate;
  }

  return null;
}

// ---- install (npm postinstall) ----

function install() {
  const binName = BINARY_MAP[platformKey()];
  if (!binName) process.exit(1);

  const srcPath = path.join(__dirname, "dist", binName);

  if (!fs.existsSync(srcPath)) {
    console.log("  ╔══════════════════════════════════════════════════╗");
    console.log("  ║  novel-agent — dev mode (no pre-built binary)    ║");
    console.log("  ║  go build -o npm\\dist\\" + binName.padEnd(30) + "║");
    console.log("  ╚══════════════════════════════════════════════════╝");
    return;
  }

  const binDir = npmBinDir();
  if (!fs.existsSync(binDir)) fs.mkdirSync(binDir, { recursive: true });

  const ext = process.platform === "win32" ? ".exe" : "";
  const dest = path.join(binDir, `novel-agent${ext}`);

  // Only copy if newer or missing
  let shouldCopy = true;
  if (fs.existsSync(dest)) {
    const srcStat = fs.statSync(srcPath);
    const dstStat = fs.statSync(dest);
    if (dstStat.mtimeMs >= srcStat.mtimeMs) shouldCopy = false;
  }

  if (shouldCopy) {
    fs.copyFileSync(srcPath, dest);
    if (process.platform !== "win32") fs.chmodSync(dest, 0o755);
  }

  const sizeMB = (fs.statSync(dest).size / (1024 * 1024)).toFixed(1);
  console.log(`✓ novel-agent ${sizeMB}MB installed → ${dest}`);
  console.log("  Run: novel-agent init");
}

// ---- runtime ----

function run(args) {
  const binPath = findBinary();
  if (!binPath) {
    console.error("╔══════════════════════════════════════════════════╗");
    console.error("║  novel-agent binary not found                    ║");
    console.error("║                                                  ║");
    console.error("║  cd ai-novel-matrix-studio                       ║");
    console.error("║  go build -o novel-agent.exe .\\cmd\\novel-agent\\  ║");
    console.error("╚══════════════════════════════════════════════════╝");
    process.exit(1);
  }

  try {
    execFileSync(binPath, args, { stdio: "inherit" });
  } catch (err) {
    if (err.status != null) process.exit(err.status);
    console.error("novel-agent:", err.message);
    process.exit(1);
  }
}

// ---- entry ----

const isPostInstall =
  process.env.npm_lifecycle_event === "postinstall" ||
  process.env.npm_command === "install" ||
  process.env.npm_command === "link";

if (require.main === module) {
  if (isPostInstall) install();
  else run(process.argv.slice(2));
}
