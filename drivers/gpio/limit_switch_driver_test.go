package gpio

import (
	"github.com/pkg/errors"
	"gobot.io/x/gobot/gobottest"
	"strings"
	"testing"
	"time"
)

func initLimitSwitchDriver() *LimitSwitchDriver {
	return NewLimitSwitchDriver(newGpioTestAdaptor(), "1")
}

func TestLimitSwitchError(t *testing.T) {
	sem := make(chan bool, 0)
	a := newGpioTestAdaptor()
	g := NewLimitSwitchDriver(a, "1")
	gobottest.Refute(t, g.Connection(), nil)

	g.On(Error, func(data interface{}) {
		sem <- true
	})

	a.TestAdaptorDigitalRead(func() (val int, err error) {
		return 0, errors.New("Error")
	})
	g.EndDetected()
	select {
	case <-sem:
	case <-time.After(250 * time.Millisecond):
		t.Errorf("Error event was not published")
	}

}

func TestLimitSwitchEndDetected(t *testing.T) {
	sem := make(chan bool, 0)
	a := newGpioTestAdaptor()
	g := NewLimitSwitchDriver(a, "1")
	g.On(EndDetected, func(data interface{}) {
		sem <- true
	})

	a.TestAdaptorDigitalRead(func() (val int, err error) {
		return 0, nil
	})
	end, _ := g.EndDetected()
	gobottest.Assert(t, end, false)
	select {
	case <-sem:
		t.Errorf("EndDetected shouldn't be published")
	case <-time.After(250 * time.Millisecond):
	}

	a.TestAdaptorDigitalRead(func() (val int, err error) {
		return 1, nil
	})
	end, _ = g.EndDetected()
	gobottest.Assert(t, end, true)
	select {
	case <-sem:
	case <-time.After(250 * time.Millisecond):
		t.Errorf("End detected event was not published")
	}

	g.DefaultOpen = false
	a.TestAdaptorDigitalRead(func() (val int, err error) {
		return 0, nil
	})
	end, _ = g.EndDetected()
	gobottest.Assert(t, end, true)
	select {
	case <-sem:
	case <-time.After(250 * time.Millisecond):
		t.Errorf("End detected event was not published")
	}

	a.TestAdaptorDigitalRead(func() (val int, err error) {
		return 1, nil
	})
	end, _ = g.EndDetected()
	gobottest.Assert(t, end, false)
	select {
	case <-sem:
		t.Errorf("EndDetected shouldn't be published")
	case <-time.After(250 * time.Millisecond):
	}
}

func TestLimitSwitchDefaultName(t *testing.T) {
	g := initLimitSwitchDriver()
	gobottest.Assert(t, strings.HasPrefix(g.Name(), "LimitSwitch"), true)
}

func TestLimitSwitchDefaultOpen(t *testing.T) {
	g := initLimitSwitchDriver()
	gobottest.Assert(t, g.DefaultOpen, true)
}

func TestLimitSwitchDriverSetName(t *testing.T) {
	g := initLimitSwitchDriver()
	g.SetName("myswitch")
	gobottest.Assert(t, g.Name(), "myswitch")
}
