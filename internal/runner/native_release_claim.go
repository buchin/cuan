package runner

// Native release stages cannot fall through to an older model worker. This is
// intentionally not advertised by the poller until its native executor is wired.
type nativeReleaseClaimKey struct{}
