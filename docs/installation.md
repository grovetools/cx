# Installation

The recommended way to install Grove Context (cx) is via the Grove meta-CLI.

## Prerequisites
- Grove CLI installed and available on your PATH

If you don’t have the Grove CLI yet, install it first following the Grove documentation for your platform.

## Install

Run:
```bash
grove install context
```

This will install the `cx` binary managed by the Grove toolchain.

## Verify

- Check that `cx` is available:
  ```bash
  cx version
  ```
- Optionally list installed Grove tools:
  ```bash
  grove list
  ```

If `cx` is not found, ensure the Grove CLI is installed correctly and that your shell PATH includes the location where Grove exposes managed binaries.