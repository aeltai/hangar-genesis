# Example Hangar policy.json

Use this file when the Genesis UI asks for an optional **policy file** before scanning images. It controls how Hangar verifies image signatures when pulling images to scan.

- **`policy.example.json`** – Copy this to `policy.json` and upload it in the "Scan selected images" dialog, or use it as a reference.

## What it does

- **`default`** – Applied to all transports when no transport-specific rule matches.  
  `insecureAcceptAnything` means: accept any image (no signature verification). Safe for local/private scans.
- **`transports`** – Optional per-transport rules (e.g. `docker://`, `containers-storage:`). Empty `{}` means use the default for all.

## Stricter policies

For production you may want to require signed images. See [Hangar documentation](https://hangar.cnrancher.com/docs/) and the [containers/image policy configuration](https://github.com/containers/image/blob/main/docs/containers-policy.json.5.md) for `signedBy`, `reject`, etc.

## Quick start

1. Copy: `cp policy.example.json my-policy.json`
2. In Genesis UI: click "Scan selected images" → choose **Start scan** with no file to use the default, or upload `my-policy.json` to use this policy.
