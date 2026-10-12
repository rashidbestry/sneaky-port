//go:build !dashboard_bundle

package control

// bundledHelper leaves ordinary source/tests independent of generated release artifacts.
// Packaged dashboards use the dashboard_bundle build tag and include a matching Linux helper.
func bundledHelper() []byte { return nil }
