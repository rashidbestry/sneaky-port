# Dependency notices

Build pins are recorded in [go.mod](../go.mod) and `go.sum`; the module requires Go 1.27.0. Retain the linked license/notice texts with redistributed binaries.

| Module | Pin | Retained licenses/notices |
| --- | --- | --- |
| [Go YAML](https://github.com/yaml/go-yaml) | `go.yaml.in/yaml/v4 v4.0.0-rc.6` | [Apache-2.0 license](../licenses/go-yaml-LICENSE), [NOTICE](../licenses/go-yaml-NOTICE), [inherited libyaml MIT license](../licenses/go-yaml-libyaml-LICENSE) |
| [Go crypto/SSH](https://pkg.go.dev/golang.org/x/crypto/ssh) | `golang.org/x/crypto v0.58.0` | [BSD-3-Clause](../licenses/go-crypto-LICENSE) |
| [Go system APIs](https://pkg.go.dev/golang.org/x/sys) | `golang.org/x/sys v0.49.0` | [BSD-3-Clause](../licenses/go-sys-LICENSE) |
| [Go terminal support](https://pkg.go.dev/golang.org/x/term) | `golang.org/x/term v0.47.0` | [BSD-3-Clause](../licenses/go-term-LICENSE) |

YAML v4 is a release candidate. Packaging also includes the Go runtime license from the actual toolchain. Reference tools are external and are not bundled. Refresh licensing review when pins change; dependency licenses do not establish a repository-wide application license, security status or product Linux minimum.
