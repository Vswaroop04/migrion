const https = require("https");
const http = require("http");
const fs = require("fs");
const path = require("path");
const os = require("os");
const { execSync } = require("child_process");

const platformMap = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const archMap = {
  x64: "amd64",
  arm64: "arm64",
};

const platform = platformMap[os.platform()];
const arch = archMap[os.arch()];

if (!platform || !arch) {
  console.error(`Unsupported platform: ${os.platform()}-${os.arch()}`);
  process.exit(1);
}

const pkg = require("./package.json");
const version = pkg.version;
const ext = os.platform() === "win32" ? ".exe" : "";
const binaryName = `migratex-${platform}-${arch}${ext}`;

const repo = "vswaroop04/migratex";
const url = `https://github.com/${repo}/releases/download/v${version}/${binaryName}`;

const dest = path.join(__dirname, "bin", binaryName);

// Skip download if binary already exists (e.g. CI pre-packed it)
if (fs.existsSync(dest)) {
  console.log(`migratex binary already exists at ${dest}`);
  process.exit(0);
}

console.log(`Downloading migratex v${version} for ${platform}-${arch}...`);
console.log(`  ${url}`);

function download(url, dest, redirects = 0) {
  if (redirects > 5) {
    console.error("Too many redirects");
    process.exit(1);
  }

  const client = url.startsWith("https") ? https : http;

  client
    .get(url, (res) => {
      // Follow redirects (GitHub releases redirect to S3)
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return download(res.headers.location, dest, redirects + 1);
      }

      if (res.statusCode !== 200) {
        console.error(`Failed to download: HTTP ${res.statusCode}`);
        console.error(`URL: ${url}`);
        console.error(
          `\nMake sure a GitHub release exists for v${version} with the binary "${binaryName}".`
        );
        process.exit(1);
      }

      fs.mkdirSync(path.dirname(dest), { recursive: true });
      const file = fs.createWriteStream(dest);
      res.pipe(file);

      file.on("finish", () => {
        file.close();
        // Make binary executable on unix
        if (os.platform() !== "win32") {
          fs.chmodSync(dest, 0o755);
        }
        console.log(`migratex installed successfully.`);
      });
    })
    .on("error", (err) => {
      console.error(`Download failed: ${err.message}`);
      process.exit(1);
    });
}

download(url, dest);
