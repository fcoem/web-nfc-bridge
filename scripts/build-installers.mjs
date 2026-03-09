#!/usr/bin/env node
import {
  mkdtempSync,
  mkdirSync,
  chmodSync,
  readFileSync,
  readdirSync,
  writeFileSync,
  rmSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve, dirname, basename } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import process from "node:process";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const supportedPlatforms = [
  "macos",
  "windows-x64",
  "windows-arm64",
  "linux-x64",
];
const artifactFamilies = [
  {
    platform: "macos",
    prefix: "connector-macos-arm64",
    cleanupPrefixes: ["connector-macos-arm64", "connector-macos-universal"],
    extensions: [".pkg"],
    cleanupExtensions: [".pkg"],
  },
  {
    platform: "windows-x64",
    prefix: "connector-windows-x64",
    extensions: [".msi"],
    cleanupExtensions: [".msi", ".zip"],
  },
  {
    platform: "windows-arm64",
    prefix: "connector-windows-arm64",
    extensions: [".msi"],
    cleanupExtensions: [".msi", ".zip"],
  },
  {
    platform: "linux-x64",
    prefix: "connector-linux-x64",
    extensions: [".deb"],
    cleanupExtensions: [".deb", ".AppImage", ".tar.gz"],
  },
];
const downloadsDir = join(repoRoot, "public", "downloads");
const downloadsManifestPath = join(downloadsDir, "manifest.json");
const defaultAllowedOrigins = [
  "http://localhost:*",
  "https://localhost:*",
  "http://127.0.0.1:*",
  "https://127.0.0.1:*",
  "https://nfc-tool.abcd854884.workers.dev",
  "https://nfc-tool.abcd854884.workers.dev.",
  "https://nfc.yudefine.com.tw",
  "https://nfc.yudefine.com.tw.",
].join(",");
const installerCatalog = [
  {
    platform: "macOS",
    label: "Connector for macOS (Apple Silicon)",
    prefix: "connector-macos-arm64",
    extensions: [".pkg"],
  },
  {
    platform: "Windows x64",
    label: "Connector for Windows x64",
    prefix: "connector-windows-x64",
    extensions: [".msi"],
  },
  {
    platform: "Windows ARM64",
    label: "Connector for Windows ARM64",
    prefix: "connector-windows-arm64",
    extensions: [".msi"],
  },
  {
    platform: "Linux x64",
    label: "Connector for Linux x64",
    prefix: "connector-linux-x64",
    extensions: [".deb"],
  },
];

function parseArgs(argv) {
  const result = {};
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token.startsWith("--")) {
      continue;
    }
    const key = token.slice(2);
    const next = argv[index + 1];
    if (!next || next.startsWith("--")) {
      result[key] = "true";
      continue;
    }
    result[key] = next;
    index += 1;
  }
  return result;
}

function normalizeVersion(value) {
  const clean = (value || "0.1.0").replace(/^v/, "");
  const match = clean.match(/\d+(?:\.\d+){0,2}/);
  return match ? match[0] : "0.1.0";
}

function compareVersions(left, right) {
  const leftParts = left
    .split(".")
    .map((part) => Number.parseInt(part, 10) || 0);
  const rightParts = right
    .split(".")
    .map((part) => Number.parseInt(part, 10) || 0);
  const length = Math.max(leftParts.length, rightParts.length);

  for (let index = 0; index < length; index += 1) {
    const delta = (leftParts[index] ?? 0) - (rightParts[index] ?? 0);
    if (delta !== 0) {
      return delta;
    }
  }

  return 0;
}

function escapeRegex(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function latestArtifact(files, prefix, extensions) {
  const matched = files
    .flatMap((fileName) => {
      return extensions.map((extension, priority) => {
        const match = fileName.match(
          new RegExp(
            `^${escapeRegex(prefix)}-(\\d+(?:\\.\\d+){0,2})${escapeRegex(extension)}$`,
          ),
        );
        return match ? { fileName, version: match[1], priority } : null;
      });
    })
    .filter((item) => item !== null)
    .sort((left, right) => {
      const versionDelta = compareVersions(right.version, left.version);
      if (versionDelta !== 0) {
        return versionDelta;
      }

      return left.priority - right.priority;
    });

  return matched[0] ?? null;
}

function writeDownloadsManifest() {
  ensureDir(downloadsDir);

  const files = readdirSync(downloadsDir, { withFileTypes: true })
    .filter((entry) => entry.isFile())
    .map((entry) => entry.name);

  const manifest = {
    generatedAt: new Date().toISOString(),
    installers: installerCatalog.map((installer) => {
      const match = latestArtifact(
        files,
        installer.prefix,
        installer.extensions,
      );
      return {
        platform: installer.platform,
        label: installer.label,
        href: match ? `/downloads/${match.fileName}` : null,
        fileName: match?.fileName ?? null,
        status: match ? "available" : "planned",
      };
    }),
  };

  writeFileSync(
    downloadsManifestPath,
    `${JSON.stringify(manifest, null, 2)}\n`,
  );
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd || repoRoot,
    stdio: "inherit",
    env: { ...process.env, ...(options.env || {}) },
    shell: false,
  });
  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(" ")} failed with exit code ${result.status ?? 1}`,
    );
  }
}

function runCapture(command, args) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
  if (result.status !== 0) {
    throw new Error((result.stderr || `${command} failed`).trim());
  }
  return result.stdout.trim();
}

function tryRunCapture(command, args) {
  try {
    return runCapture(command, args);
  } catch {
    return null;
  }
}

function findTool(tool) {
  if (process.platform === "win32") {
    return tryRunCapture("where.exe", [tool]);
  }

  return tryRunCapture("sh", ["-lc", `command -v ${tool}`]);
}

function ensureTool(tool, hint) {
  const toolPath = findTool(tool);
  if (!toolPath) {
    throw new Error(hint);
  }

  return toolPath;
}

function ensureDir(path) {
  mkdirSync(path, { recursive: true });
}

function readPackageVersion() {
  try {
    const packageJson = JSON.parse(
      readFileSync(join(repoRoot, "package.json"), "utf8"),
    );
    return typeof packageJson.version === "string" ? packageJson.version : null;
  } catch {
    return null;
  }
}

function readExactGitTag() {
  return tryRunCapture("git", ["describe", "--tags", "--exact-match", "HEAD"]);
}

function resolveVersion(explicitVersion) {
  return normalizeVersion(
    explicitVersion ||
      process.env.RELEASE_VERSION ||
      readExactGitTag() ||
      readPackageVersion(),
  );
}

function removeFile(path) {
  rmSync(path, { force: true });
}

function cleanupOldArtifacts(outputDir) {
  const files = readdirSync(outputDir, { withFileTypes: true })
    .filter((entry) => entry.isFile())
    .map((entry) => entry.name);

  for (const family of artifactFamilies) {
    const prefixes = family.cleanupPrefixes || [family.prefix];
    const matched = files
      .flatMap((fileName) => {
        return prefixes.flatMap((prefix) => {
          return family.cleanupExtensions.map((extension) => {
            const match = fileName.match(
              new RegExp(
                `^${prefix}-(\\d+(?:\\.\\d+){0,2})${extension.replace(".", "\\.")}$`,
              ),
            );
            return match ? { fileName, version: match[1] } : null;
          });
        });
      })
      .filter((item) => item !== null);

    if (matched.length === 0) {
      continue;
    }

    const latestVersion = matched
      .map((item) => item.version)
      .sort((left, right) => compareVersions(right, left))[0];

    for (const artifact of matched) {
      const shouldKeep =
        artifact.version === latestVersion &&
        family.extensions.some((extension) =>
          artifact.fileName.endsWith(extension),
        );

      if (!shouldKeep) {
        removeFile(join(outputDir, artifact.fileName));
      }
    }
  }
}

function nativeOutputName(platform, version) {
  const suffix = `-${normalizeVersion(version)}`;
  switch (platform) {
    case "macos":
      return `connector-macos-arm64${suffix}.pkg`;
    case "windows-x64":
      return `connector-windows-x64${suffix}.msi`;
    case "windows-arm64":
      return `connector-windows-arm64${suffix}.msi`;
    case "linux-x64":
      return `connector-linux-x64${suffix}.deb`;
    default:
      throw new Error(`Unknown platform: ${platform}`);
  }
}

function buildGoBinary(outputPath, env) {
  ensureDir(dirname(outputPath));
  const buildVersion = env.BUILD_VERSION || "dev";
  const buildTime = env.BUILD_TIME || new Date().toISOString();
  run(
    "go",
    [
      "build",
      "-ldflags",
      `-X main.version=${buildVersion} -X main.buildTime=${buildTime}`,
      "-o",
      outputPath,
      "./connector/cmd/nfc-connector",
    ],
    {
      env,
    },
  );
  chmodSync(outputPath, 0o755);
}

function createTarArchive(sourceDir, outputPath) {
  ensureTool("tar", "tar is required to build Debian package archives");
  run("tar", ["-czf", outputPath, "."], {
    cwd: sourceDir,
  });
}

function createDebPackage(packageRootDir, controlDir, outputPath) {
  ensureTool("ar", "ar is required to build Debian packages");

  const workDir = dirname(packageRootDir);
  const controlArchive = join(workDir, "control.tar.gz");
  const dataArchive = join(workDir, "data.tar.gz");
  const debianBinary = join(workDir, "debian-binary");

  createTarArchive(controlDir, controlArchive);
  createTarArchive(packageRootDir, dataArchive);
  writeFileSync(debianBinary, "2.0\n");

  run(
    "ar",
    ["rc", outputPath, "debian-binary", "control.tar.gz", "data.tar.gz"],
    {
      cwd: workDir,
    },
  );
}

function buildMacOS(version, outputDir) {
  if (process.platform !== "darwin") {
    throw new Error("macOS pkg packaging must run on a macOS host");
  }

  ensureTool("pkgbuild", "pkgbuild is required to build the macOS installer");

  const workDir = mkdtempSync(join(tmpdir(), "nfc-tool-macos-"));
  const pkgRoot = join(workDir, "root");
  const scriptsDir = join(workDir, "scripts");
  const binaryDir = join(pkgRoot, "usr", "local", "libexec", "nfc-tool");
  const wrapperDir = join(pkgRoot, "usr", "local", "bin");
  const launchAgentsDir = join(pkgRoot, "Library", "LaunchAgents");
  ensureDir(binaryDir);
  ensureDir(wrapperDir);
  ensureDir(launchAgentsDir);
  ensureDir(scriptsDir);

  const binaryPath = join(binaryDir, "nfc-connector");
  buildGoBinary(binaryPath, {
    BUILD_VERSION: version,
    BUILD_TIME: new Date().toISOString(),
    GOOS: "darwin",
    GOARCH: "arm64",
    CGO_ENABLED: "1",
  });

  const wrapperPath = join(wrapperDir, "nfc-tool-connector");
  writeFileSync(
    wrapperPath,
    '#!/bin/sh\nexec /usr/local/libexec/nfc-tool/nfc-connector "$@"\n',
  );
  chmodSync(wrapperPath, 0o755);

  const plistPath = join(launchAgentsDir, "com.nfc-tool.connector.plist");
  writeFileSync(
    plistPath,
    `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>com.nfc-tool.connector</string>
    <key>ProgramArguments</key>
    <array>
      <string>/usr/local/libexec/nfc-tool/nfc-connector</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>EnvironmentVariables</key>
    <dict>
      <key>NFC_CONNECTOR_ADDR</key>
      <string>127.0.0.1:42619</string>
      <key>NFC_CONNECTOR_DRIVER</key>
      <string>pcsc</string>
      <key>NFC_CONNECTOR_ALLOWED_ORIGINS</key>
      <string>${defaultAllowedOrigins}</string>
      <key>NFC_CONNECTOR_SHARED_SECRET</key>
      <string>development-shared-secret</string>
    </dict>
    <key>StandardOutPath</key>
    <string>/tmp/nfc-tool-connector.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/nfc-tool-connector.error.log</string>
  </dict>
</plist>
`,
  );

  const preinstallPath = join(scriptsDir, "preinstall");
  writeFileSync(
    preinstallPath,
    '#!/bin/sh\nset -eu\nplist=/Library/LaunchAgents/com.nfc-tool.connector.plist\nbinary=/usr/local/libexec/nfc-tool/nfc-connector\nwrapper=/usr/local/bin/nfc-tool-connector\ncurrent_user=$(stat -f %Su /dev/console || true)\nif [ -n "$current_user" ] && [ "$current_user" != "root" ]; then\n  uid=$(id -u "$current_user")\n  launchctl bootout "gui/$uid" "$plist" >/dev/null 2>&1 || true\n  launchctl remove "gui/$uid/com.nfc-tool.connector" >/dev/null 2>&1 || true\nfi\nrm -f "$wrapper"\nrm -f "$binary"\nrm -f "$plist"\nrmdir /usr/local/libexec/nfc-tool >/dev/null 2>&1 || true\nexit 0\n',
  );
  chmodSync(preinstallPath, 0o755);

  const postinstallPath = join(scriptsDir, "postinstall");
  writeFileSync(
    postinstallPath,
    '#!/bin/sh\nset -eu\ncurrent_user=$(stat -f %Su /dev/console || true)\nif [ -n "$current_user" ] && [ "$current_user" != "root" ]; then\n  uid=$(id -u "$current_user")\n  plist=/Library/LaunchAgents/com.nfc-tool.connector.plist\n  launchctl bootout "gui/$uid" "$plist" >/dev/null 2>&1 || true\n  launchctl bootstrap "gui/$uid" "$plist" >/dev/null 2>&1 || true\n  launchctl kickstart -k "gui/$uid/com.nfc-tool.connector" >/dev/null 2>&1 || true\nfi\nexit 0\n',
  );
  chmodSync(postinstallPath, 0o755);

  const outputPath = join(outputDir, nativeOutputName("macos", version));
  run("pkgbuild", [
    "--root",
    pkgRoot,
    "--scripts",
    scriptsDir,
    "--identifier",
    "com.nfc-tool.connector",
    "--version",
    version,
    outputPath,
  ]);

  rmSync(workDir, { recursive: true, force: true });
  return outputPath;
}

function buildWindows(version, outputDir, arch) {
  const goArch = arch === "x64" ? "amd64" : arch;

  if (process.platform !== "win32" || !findTool("wix")) {
    return null;
  }

  ensureTool(
    "wix",
    "wix CLI is required to build the Windows MSI. Install it with: dotnet tool install --global wix",
  );

  const workDir = mkdtempSync(join(tmpdir(), `nfc-tool-windows-${arch}-`));
  const binaryPath = join(workDir, "nfc-connector.exe");
  buildGoBinary(binaryPath, {
    BUILD_VERSION: version,
    BUILD_TIME: new Date().toISOString(),
    GOOS: "windows",
    GOARCH: goArch,
  });

  const wixSource = join(workDir, "connector.wxs");
  const upgradeCode =
    arch === "arm64"
      ? "CA1D4C2B-5D84-4F43-8A47-4BE8F9B4F120"
      : "F6AFDE3D-4F3B-4E84-85A8-91BAA77A23A8";
  writeFileSync(
    wixSource,
    `<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="NFC Tool Connector" Manufacturer="NFC Tool" Version="${version}" UpgradeCode="${upgradeCode}">
    <MediaTemplate EmbedCab="yes" />
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="NFC Tool Connector" />
    </StandardDirectory>
    <Feature Id="MainFeature" Title="NFC Tool Connector" Level="1">
      <ComponentGroupRef Id="ConnectorFiles" />
    </Feature>
  </Package>
  <Fragment>
    <ComponentGroup Id="ConnectorFiles" Directory="INSTALLFOLDER">
      <Component Guid="*">
        <File Source="${binaryPath.replace(/\\/g, "/")}" KeyPath="yes" />
      </Component>
    </ComponentGroup>
  </Fragment>
</Wix>
`,
  );

  const outputPath = join(
    outputDir,
    nativeOutputName(
      arch === "arm64" ? "windows-arm64" : "windows-x64",
      version,
    ),
  );
  run("wix", ["build", "-arch", arch, "-o", outputPath, wixSource]);
  rmSync(workDir, { recursive: true, force: true });
  return outputPath;
}

function buildLinux(version, outputDir) {
  const workDir = mkdtempSync(join(tmpdir(), "nfc-tool-linux-deb-"));
  const dataRoot = join(workDir, "data");
  const controlRoot = join(workDir, "control");
  const binaryDir = join(dataRoot, "usr", "local", "libexec", "nfc-tool");
  const wrapperDir = join(dataRoot, "usr", "local", "bin");
  const configDir = join(dataRoot, "etc", "default");
  const systemdDir = join(dataRoot, "lib", "systemd", "system");
  ensureDir(binaryDir);
  ensureDir(wrapperDir);
  ensureDir(configDir);
  ensureDir(systemdDir);
  ensureDir(controlRoot);

  const binaryPath = join(binaryDir, "nfc-connector");
  buildGoBinary(binaryPath, {
    BUILD_VERSION: version,
    BUILD_TIME: new Date().toISOString(),
    GOOS: "linux",
    GOARCH: "amd64",
  });

  writeFileSync(
    join(wrapperDir, "nfc-tool-connector"),
    '#!/bin/sh\nexec /usr/local/libexec/nfc-tool/nfc-connector "$@"\n',
  );
  chmodSync(join(wrapperDir, "nfc-tool-connector"), 0o755);

  writeFileSync(
    join(configDir, "nfc-tool-connector"),
    [
      'NFC_CONNECTOR_ADDR="127.0.0.1:42619"',
      'NFC_CONNECTOR_DRIVER="pcsc"',
      `NFC_CONNECTOR_ALLOWED_ORIGINS="${defaultAllowedOrigins}"`,
      'NFC_CONNECTOR_SHARED_SECRET="development-shared-secret"',
    ].join("\n") + "\n",
  );

  writeFileSync(
    join(systemdDir, "nfc-tool-connector.service"),
    [
      "[Unit]",
      "Description=NFC Tool Connector",
      "After=network.target pcscd.service",
      "Wants=network.target",
      "",
      "[Service]",
      "Type=simple",
      "EnvironmentFile=-/etc/default/nfc-tool-connector",
      "ExecStart=/usr/local/libexec/nfc-tool/nfc-connector",
      "Restart=always",
      "RestartSec=3",
      "",
      "[Install]",
      "WantedBy=multi-user.target",
    ].join("\n") + "\n",
  );

  writeFileSync(
    join(controlRoot, "control"),
    [
      "Package: nfc-tool-connector",
      `Version: ${version}`,
      "Section: utils",
      "Priority: optional",
      "Architecture: amd64",
      "Maintainer: NFC Tool <support@nfc-tool.local>",
      "Depends: libc6",
      "Description: NFC Tool localhost connector",
      " Connector service for the NFC Tool web app.",
    ].join("\n") + "\n",
  );

  writeFileSync(
    join(controlRoot, "postinst"),
    [
      "#!/bin/sh",
      "set -eu",
      "if command -v systemctl >/dev/null 2>&1; then",
      "  systemctl daemon-reload >/dev/null 2>&1 || true",
      "  systemctl enable nfc-tool-connector.service >/dev/null 2>&1 || true",
      "  systemctl restart nfc-tool-connector.service >/dev/null 2>&1 || systemctl start nfc-tool-connector.service >/dev/null 2>&1 || true",
      "fi",
      "exit 0",
    ].join("\n") + "\n",
  );
  chmodSync(join(controlRoot, "postinst"), 0o755);

  writeFileSync(
    join(controlRoot, "prerm"),
    [
      "#!/bin/sh",
      "set -eu",
      "if command -v systemctl >/dev/null 2>&1; then",
      "  systemctl stop nfc-tool-connector.service >/dev/null 2>&1 || true",
      "  systemctl disable nfc-tool-connector.service >/dev/null 2>&1 || true",
      "fi",
      "exit 0",
    ].join("\n") + "\n",
  );
  chmodSync(join(controlRoot, "prerm"), 0o755);

  writeFileSync(
    join(controlRoot, "postrm"),
    [
      "#!/bin/sh",
      "set -eu",
      "if command -v systemctl >/dev/null 2>&1; then",
      "  systemctl daemon-reload >/dev/null 2>&1 || true",
      "  systemctl reset-failed nfc-tool-connector.service >/dev/null 2>&1 || true",
      "fi",
      "exit 0",
    ].join("\n") + "\n",
  );
  chmodSync(join(controlRoot, "postrm"), 0o755);

  const outputPath = join(outputDir, nativeOutputName("linux-x64", version));
  createDebPackage(dataRoot, controlRoot, outputPath);
  rmSync(workDir, { recursive: true, force: true });
  return outputPath;
}

function buildPlatform(platform, version, outputDir) {
  switch (platform) {
    case "macos":
      return buildMacOS(version, outputDir);
    case "windows-x64":
      return buildWindows(version, outputDir, "x64");
    case "windows-arm64":
      return buildWindows(version, outputDir, "arm64");
    case "linux-x64":
      return buildLinux(version, outputDir);
    default:
      throw new Error(`Unsupported platform target: ${platform}`);
  }
}

const args = parseArgs(process.argv.slice(2));
const version = resolveVersion(args.version);
const requestedPlatform = args.platform || "all";
const outputDir = resolve(repoRoot, args["output-dir"] || "public/downloads");
ensureDir(outputDir);

const platforms =
  requestedPlatform === "all" ? supportedPlatforms : [requestedPlatform];
for (const platform of platforms) {
  if (!supportedPlatforms.includes(platform)) {
    throw new Error(
      `Unknown platform '${platform}'. Supported: ${supportedPlatforms.join(", ")}, all`,
    );
  }
  const artifactPath = buildPlatform(platform, version, outputDir);
  if (artifactPath) {
    console.log(`Built ${platform} installer: ${artifactPath}`);
    continue;
  }

  console.log(
    `Skipped ${platform} installer: required packaging toolchain is unavailable on this host`,
  );
}

cleanupOldArtifacts(outputDir);
writeDownloadsManifest();
