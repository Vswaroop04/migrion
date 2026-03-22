#!/usr/bin/env node

const { execFileSync } = require("child_process");
const path = require("path");
const os = require("os");
const fs = require("fs");

function getBinaryPath() {
  const platform = os.platform();
  const arch = os.arch();

  const platformMap = {
    darwin: "darwin",
    linux: "linux",
    win32: "windows",
  };

  const archMap = {
    x64: "amd64",
    arm64: "arm64",
  };

  const goPlatform = platformMap[platform];
  const goArch = archMap[arch];

  if (!goPlatform || !goArch) {
    console.error(`Unsupported platform: ${platform}-${arch}`);
    process.exit(1);
  }

  const ext = platform === "win32" ? ".exe" : "";
  const binaryName = `migratex-${goPlatform}-${goArch}${ext}`;
  const binaryPath = path.join(__dirname, "..", "bin", binaryName);

  if (!fs.existsSync(binaryPath)) {
    console.error(
      `Binary not found: ${binaryPath}\nRun "npm run postinstall" or reinstall the package.`
    );
    process.exit(1);
  }

  return binaryPath;
}

const binary = getBinaryPath();

try {
  execFileSync(binary, process.argv.slice(2), { stdio: "inherit" });
} catch (err) {
  if (err.status !== undefined) {
    process.exit(err.status);
  }
  throw err;
}
