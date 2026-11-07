package task

// Only Service can grant this after validating native evidence. Stores still
// recheck the exact claimed launch and predecessor atomically with insertion.
type inspectionAdmissionKey struct{}
type inspectionAdmission struct {
	LaunchID    string
	Revision    string
	Predecessor string
}
