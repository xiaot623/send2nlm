const fs = require('fs');
const https = require('https');
const os = require('os');
const path = require('path');

// If not being installed from node_modules, skip download (local development)
if (!__dirname.includes('node_modules')) {
  console.log('Running locally, skipping binary download.');
  process.exit(0);
}

const version = process.env.npm_package_version || '0.1.0';
const binName = os.platform() === 'win32' ? 'send2nlm.exe' : 'send2nlm';
const binDir = path.join(__dirname, '..', 'bin');
const binPath = path.join(binDir, binName);

const platformMap = {
  darwin: 'darwin',
  linux: 'linux',
  win32: 'windows'
};

const archMap = {
  x64: 'amd64',
  arm64: 'arm64'
};

const platform = platformMap[os.platform()];
const arch = archMap[os.arch()];

if (!platform || !arch) {
  console.error(`Unsupported platform or architecture: ${os.platform()}-${os.arch()}`);
  process.exit(1);
}

// GitHub release tags usually start with v
const tag = version.startsWith('v') ? version : `v${version}`;
const downloadName = `send2nlm-${platform}-${arch}${os.platform() === 'win32' ? '.exe' : ''}`;
// The URL should match where GitHub Actions uploads the binaries
const url = `https://github.com/xiaot623/send2nlm/releases/download/${tag}/${downloadName}`;

console.log(`Downloading send2nlm from ${url}...`);

if (!fs.existsSync(binDir)) {
  fs.mkdirSync(binDir, { recursive: true });
}

const file = fs.createWriteStream(binPath);

function download(url) {
  https.get(url, (response) => {
    if (response.statusCode === 302 || response.statusCode === 301) {
      // Follow redirect
      download(response.headers.location);
    } else if (response.statusCode === 200) {
      response.pipe(file);
      file.on('finish', () => {
        file.close();
        if (os.platform() !== 'win32') {
          fs.chmodSync(binPath, 0o755); // Make it executable
        }
        console.log('Download completed successfully.');
      });
    } else {
      console.error(`Failed to download binary, HTTP status code: ${response.statusCode}`);
      process.exit(1);
    }
  }).on('error', (err) => {
    fs.unlinkSync(binPath);
    console.error('Error downloading the file:', err.message);
    process.exit(1);
  });
}

download(url);
