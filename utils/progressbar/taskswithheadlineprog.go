package progressbar

import "github.com/vbauerster/mpb/v7"

// Progress that includes two bars:
// 1. Headline bar
// 2. Tasks counter progress bar.
type TasksWithHeadlineProg struct {
	headlineBar      *mpb.Bar
	tasksProgressBar *TasksProgressBar
	emptyLine        *mpb.Bar
}

func (hp *TasksWithHeadlineProg) GetTasksProgressBar() *TasksProgressBar {
	return hp.tasksProgressBar
}
