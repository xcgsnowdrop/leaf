package timer

import (
	"fmt"
	"testing"
	"time"
	// "github.com/name5566/leaf/timer"
)

func TestDispatcher(t *testing.T) {
	d := NewDispatcher(10)

	// timer 1
	d.AfterFunc(1, func() {
		fmt.Println("My name is Leaf")
	})

	// timer 2
	t2 := d.AfterFunc(1, func() {
		fmt.Println("will not print")
	})
	t2.Stop()

	// dispatch
	(<-d.ChanTimer).Cb()

	// Output:
	// My name is Leaf
}

func TestNewCronExpr(t *testing.T) {
	cronExpr, err := NewCronExpr("0 * * * *")
	if err != nil {
		t.Errorf("NewCronExpr(\"0 * * * *\") failed: %v", err)
	}

	// 下一个触发cron任务的时间点
	// Output: 2000-01-01 21:00:00 +0000 UTC
	fmt.Println(cronExpr.Next(time.Date(
		2000, 1, 1,
		20, 10, 5,
		0, time.UTC,
	)))
}

func TestDispatcherCron(t *testing.T) {
	d := NewDispatcher(10)

	// cron expr
	cronExpr, err := NewCronExpr("* * * * * *")
	if err != nil {
		t.Errorf("%v", err)
	}

	// cron
	var c *Cron
	c = d.CronFunc(cronExpr, func() {
		fmt.Println("My name is Leaf")
		c.Stop()
	})

	// dispatch
	(<-d.ChanTimer).Cb()

	// Output:
	// My name is Leaf
}
