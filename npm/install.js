#!/usr/bin/env node
/**
 * AI Novel Agent — npm installer
 *
 * Downloads the correct platform binary from the dist/ directory
 * and places it in the global node_modules/.bin/ path.
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

const PLATFORM_MAP = {
  "darwin-x64":   "novel-agent_darwin_amd64",
  "darwin-arm64": "novel-agent_darwin_arm64",
  "linux-x64":    "novel-agent_linux_amd64",
  "win32-x64":    "novel-agent_windows_amd64.exe",
};

function detectPlatform() {
  const platform = os.platform(); // 'darwin' | 'linux' | 'win32'
  const arch = os.arch();         // 'x64' | 'arm64'
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

function install() {
  const { binary } = detectPlatform();

  // Determine source path
  const srcDir = path.join(__dirname, "dist");
  const srcPath = path.join(srcDir, binary);
  if (!fs.existsSync(srcPath)) {
    console.error(`ai-novel-agent: binary not found: ${srcPath}`);
    console.error("Please ensure the npm package was published with all platform binaries.");
    process.exit(1);
  }

  // Determine target path
  // npm global install places binaries in prefix/bin/
  const prefix = process.env.npm_config_prefix || path.resolve(os.homedir(), ".npm-global");
  const binDir = path.join(prefix, "bin");
  const ext = process.platform === "win32" ? ".exe" : "";
  const targetPath = path.join(binDir, `novel-agent${ext}`);

  // Ensure bin directory exists
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  // Copy binary
  fs.copyFileSync(srcPath, targetPath);

  // Make executable on Unix
  if (process.platform !== "win32") {
    fs.chmodSync(targetPath, 0o755);
  }

  console.log(`✓ ai-novel-agent v${require("./package.json").version} installed successfully`);
  console.log(`  Binary: ${targetPath}`);
  console.log(`  Run 'novel-agent init' to get started.`);
}

// Only run when executed directly (npm postinstall)
if (require.main === module) {
  install();
}

module.exports = { install, detectPlatform };
