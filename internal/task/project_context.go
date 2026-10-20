package task

// SetProjectFinder configures project lookup before the service starts serving.
// Server wiring uses a live resolver so replacing a project service cannot leave
// task creation reading a stale, unrelated store.
func (s *Service) SetProjectFinder(finder ProjectFinder) {
	s.projectFinder = finder
}
