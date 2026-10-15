package control

// DefaultServerPolicy supplies the binary's fixed service/resource ceilings for runtime-selected bindings.
// Clients still supply every test plan; requests beyond these ceilings reject without clamping or tuning.
func DefaultServerPolicy(controlListen, dataBindIP string) ServerPolicy {
	return ServerPolicy{
		Version: 1, ControlListen: controlListen, DataBindIP: dataBindIP,
		PortMin: 1, PortMax: 65535,
		SetupTimeout: 30_000_000_000, ShutdownTimeout: 30_000_000_000,
		AuthPoll: 1_000_000_000, MaxConnections: 4, MaxAuthTries: 3,
		MaxControlRequests: 1 << 20, MaxBindAttempts: 16,
		MaxQueueBytes: 1 << 30, MaxLedgerMessages: 1 << 20,
		MaxEventBytes: 1 << 30, MaxFileBytes: 1 << 30,
	}
}
