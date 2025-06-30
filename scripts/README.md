# Lacquer Scripts

This directory contains utility scripts for the Lacquer project.

## install.sh

The installation script for Lacquer that can be used via curl:

```bash
curl -sSL https://get.lacquer.ai | sh
```

### Features

- Automatic OS and architecture detection
- Support for Linux, macOS, and Windows (via WSL)
- Downloads the latest release from GitHub
- Verifies the installation
- Provides PATH setup instructions if needed

### Environment Variables

- `INSTALL_DIR`: Custom installation directory (default: `/usr/local/bin`)
- `VERSION`: Specific version to install (default: latest)

### Examples

Install to custom directory:
```bash
INSTALL_DIR=$HOME/.local/bin curl -sSL https://get.lacquer.ai | sh
```

Install specific version:
```bash
VERSION=v0.1.0 curl -sSL https://get.lacquer.ai | sh
```