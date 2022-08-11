package progressbar

import (
	"github.com/vbauerster/mpb/v7"
	"sync/atomic"
)

type generalProgressBar struct {
	bar   *mpb.Bar
	total int64
}

// IncGeneralProgressTotalBy increments the amount of total by n.
func (p *generalProgressBar) IncGeneralProgressTotalBy(n int64) {
	atomic.AddInt64(&p.total, n)
	if p.bar != nil {
		p.bar.SetTotal(p.total, false)
	}
}

// SetGeneralProgressTotal sets the amount of total to n.
func (p *generalProgressBar) SetGeneralProgressTotal(n int64) {
	atomic.StoreInt64(&p.total, n)
	if p.bar != nil {
		p.bar.SetTotal(p.total, false)
	}
}

func (p *generalProgressBar) GetBar() *mpb.Bar {
	return p.bar
}

func (p *generalProgressBar) GetTotal() int64 {
	return p.total
}
