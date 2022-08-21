package progressbar

// TasksProgressBar counts tasks that have been completed, using a "%d/%d" format.
type TasksProgressBar struct {
	generalProgressBar
	tasksCount int64
}

func (tpb *TasksProgressBar) GetTasksCount() int64 {
	return tpb.tasksCount
}
