# Dependency notices

First foundation pins checked on 2026-10-11 with Go 1.27.1. `go.mod` and `go.sum` are the build record; these pins do not establish a Linux release minimum or future security status.

| Module | Pin | License / retained text |
| --- | --- | --- |
| [Go YAML](https://github.com/yaml/go-yaml) | `go.yaml.in/yaml/v4 v4.0.0-rc.6` | Apache-2.0 and inherited libyaml MIT notice: [LICENSE](../licenses/go-yaml-LICENSE), [NOTICE](../licenses/go-yaml-NOTICE), [libyaml license](../licenses/go-yaml-libyaml-LICENSE) |
| [Go crypto / SSH](https://pkg.go.dev/golang.org/x/crypto/ssh) | `golang.org/x/crypto v0.58.0` | BSD-3-Clause: [LICENSE](../licenses/go-crypto-LICENSE) |
| [Go system APIs](https://pkg.go.dev/golang.org/x/sys) | `golang.org/x/sys v0.49.0` | BSD-3-Clause: [LICENSE](../licenses/go-sys-LICENSE) |
| [Go terminal support](https://pkg.go.dev/golang.org/x/term) | `golang.org/x/term v0.47.0` (direct dependency for masked local key prompts) | BSD-3-Clause: [LICENSE](../licenses/go-term-LICENSE) |

YAML v4 is a release candidate, deliberately recorded rather than labeled stable. The selected SSH/system releases require Go 1.26 or later; this workspace uses 1.27.1. The public plan hash implements the [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785.html) integer/string domain. Windows marking follows [Microsoft's per-flow DSCP API](https://learn.microsoft.com/en-us/windows/win32/api/qos2/nf-qos2-qossetflow); API availability does not prove outer marking or ECN preservation.

Retain these notices with redistributed binaries. Reference measurement tools are not bundled. Licensing/release review must be refreshed when dependencies change; no repository-wide application license is inferred from dependency licenses.
