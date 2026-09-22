# Third-party components

- `third_party/wirectl`: CLI source snapshot from `../wirectl` (k0ngk0ng),
  with Windows `.exe` discovery fixes. The snapshot makes builds independent
  of sibling checkouts. The host/plugin executable contract is unchanged.
- `internal/installpath`: MIT-licensed Windows junction resolution and tests
  adapted from k0ngk0ng/wire-connect; copyright 2026 k0ngk0ng.
- `github.com/gen2brain/malgo v0.11.24`: native audio bindings, Unlicense.
- miniaudio (embedded by malgo): public domain or MIT No Attribution,
  selected by the consumer. Its license is included in release archives.
- `golang.org/x/sys v0.31.0`: Windows security APIs, BSD 3-Clause.
- Go standard library/runtime: Go BSD-style license.

The application source is MIT licensed; see LICENSE. Module versions and
checksums are recorded in go.mod and go.sum. No speech recognition or cloud
speech service is used.
