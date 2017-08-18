package gpio

import (
	"github.com/pkg/errors"
	"gobot.io/x/gobot/gobottest"
	"math"
	"strings"
	"testing"
	"time"
)

func initStepperMotorDriver() *StepperMotorDriver {
	return NewStepperMotorDriver(newGpioTestAdaptor(), "1", "2", "3")
}

func TestStepperMotorDriver(t *testing.T) {
	a := newGpioTestAdaptor()
	g := NewStepperMotorDriver(a, "1", "2", "3")
	gobottest.Refute(t, g.Connection(), nil)

	err := g.Command("Min")(nil)
	gobottest.Assert(t, err.(error), ErrStepperMotorEndstopUnsupported)

	err = g.Command("Center")(nil)
	gobottest.Assert(t, err.(error), ErrNotCalibrated)

	err = g.Command("Max")(nil)
	gobottest.Assert(t, err.(error), ErrNotCalibrated)

	err = g.Command("Move")(map[string]interface{}{"angle": 100.0})
	gobottest.Assert(t, err, nil)
}

func TestStepperMotorDriverMove(t *testing.T) {
	g := initStepperMotorDriver()
	g.CurrentPosition = 50
	g.Move(20)
	gobottest.Assert(t, math.Abs(g.CurrentPosition-70) < 1.8, true)
	curr_pos := g.CurrentPosition
	g.Move(0)
	gobottest.Assert(t, curr_pos, g.CurrentPosition)
}

func TestStepperMotorDriverMoveError(t *testing.T) {
	a := newGpioTestAdaptor()
	g := NewStepperMotorDriver(a, "1", "2", "3")

	a.TestAdaptorDigitalWrite(func() (err error) {
		return errors.New("ERR")
	})
	err := g.Move(10)
	gobottest.Refute(t, err, nil)
}

func TestStepperMotorDriverMoveAsync(t *testing.T) {
	g := initStepperMotorDriver()
	g.CurrentPosition = 50
	sem := make(chan bool)
	g.MoveAsync(-10, sem)

	select {
	case <-sem:
	case <-time.After(2 * time.Second):
		t.Errorf("Move not ended")
	}
	gobottest.Assert(t, math.Abs(g.CurrentPosition-40) < 1.8, true)

	g.Microstepping = 0
	g.DirectionInversion = true
	g.MoveAsync(10, sem)
	select {
	case <-sem:
	case <-time.After(2 * time.Second):
		t.Errorf("Move not ended")
	}
	gobottest.Assert(t, g.GetLastError(), ErrIncorrectMicrostepping)
}

func TestStepperMotorDefaultName(t *testing.T) {
	g := initStepperMotorDriver()
	gobottest.Assert(t, strings.HasPrefix(g.Name(), "StepperMotor"), true)
}

func TestStepperMotorDriverSetName(t *testing.T) {
	g := initStepperMotorDriver()
	g.SetName("mystepper")
	gobottest.Assert(t, g.Name(), "mystepper")
}

func TestStepperMotorDriverStart(t *testing.T) {
	d := initStepperMotorDriver()
	gobottest.Assert(t, d.Start(), nil)
}

func TestStepperMotorDriverHalt(t *testing.T) {
	d := initStepperMotorDriver()
	gobottest.Assert(t, d.Halt(), nil)
}

func TestStepperMotorConfigureEndDetection(t *testing.T) {
	g := initStepperMotorDriver()
	gobottest.Assert(t, g.checkEndstopsDefined(), false)
	g.ConfigureEndDetection(nil, nil, 0)
	gobottest.Assert(t, g.checkEndstopsDefined(), false)

	limit := new(LimitSwitchDriver)
	g.ConfigureEndDetection(limit, nil, 0)
	gobottest.Assert(t, g.checkEndstopsDefined(), false)
	g.ConfigureEndDetection(limit, limit, 0)
	gobottest.Assert(t, g.checkEndstopsDefined(), true)
	g.ConfigureEndDetection(limit, nil, 100)
	gobottest.Assert(t, g.checkEndstopsDefined(), true)
	g.ConfigureEndDetection(nil, limit, 100)
	gobottest.Assert(t, g.checkEndstopsDefined(), false)
}

func TestStepperMotorGetError(t *testing.T) {
	g := initStepperMotorDriver()
	gobottest.Assert(t, g.GetLastError(), nil)
	err := errors.New("Test err")
	g.pushError(err)
	gobottest.Assert(t, g.GetLastError(), err)
	gobottest.Assert(t, g.GetLastError(), nil)
}

func TestStepperMotorOnOff(t *testing.T) {
	g := initStepperMotorDriver()
	gobottest.Assert(t, g.CurrentState, STATE_DISABLED)
	g.On()
	gobottest.Assert(t, g.CurrentState, STATE_ENABLED)
	g.Off()
	gobottest.Assert(t, g.CurrentState, STATE_DISABLED)
}

func TestStepperMotorCalculateSleepErrors(t *testing.T) {
	g := initStepperMotorDriver()
	var sleep *time.Duration = nil
	g.Microstepping = 0
	sleep, err := g.calculateSleep()
	gobottest.Assert(t, sleep, sleep)
	gobottest.Assert(t, err, ErrIncorrectMicrostepping)

	g.Microstepping = 4
	g.StepsPerTurn = -10
	sleep, err = g.calculateSleep()
	gobottest.Assert(t, sleep, sleep)
	gobottest.Assert(t, err, ErrIncorrectStepsPerTurn)

	g.StepsPerTurn = 200
	g.Speed = -10
	sleep, err = g.calculateSleep()
	gobottest.Assert(t, sleep, sleep)
	gobottest.Assert(t, err, ErrIncorrectSpeed)
}

func TestStepperMotorCalculateSleep(t *testing.T) {
	g := initStepperMotorDriver()
	g.Microstepping = 2

	expected := float64(1000000000) / (float64(400*30) / 60) / 2
	expected_sleep := time.Duration(expected)

	sleep, _ := g.calculateSleep()
	gobottest.Assert(t, expected_sleep, *sleep)
}

func TestStepperMotorSingleStepError(t *testing.T) {
	a := newGpioTestAdaptor()
	g := NewStepperMotorDriver(a, "1", "2", "3")

	a.TestAdaptorDigitalWrite(func() (err error) {
		return errors.New("ERR")
	})
	err := g.singleStep(time.Duration(10), true)
	gobottest.Refute(t, err, nil)
}

func TestStepperMotorMoveMicroStepsError(t *testing.T) {
	a := newGpioTestAdaptor()
	g := NewStepperMotorDriver(a, "1", "2", "3")

	a.TestAdaptorDigitalWrite(func() (err error) {
		return errors.New("ERR")
	})
	steps, err := g.moveMicroSteps(10)
	gobottest.Assert(t, steps, 0)
	gobottest.Refute(t, err, nil)
}

type mockedEndstop struct {
	LimitSwitchDriverInterface
	detected bool
	err      error
	counter  int
}

func (m *mockedEndstop) EndDetected() (bool, error) {
	m.counter++
	if m.counter == 10 {
		return true, nil
	}
	return m.detected, m.err
}

func TestStepperMotorMoveWhileEndstopOpenErr(t *testing.T) {
	a := newGpioTestAdaptor()
	g := NewStepperMotorDriver(a, "1", "2", "3")
	g.Microstepping = 0

	a.TestAdaptorDigitalWrite(func() (err error) {
		return errors.New("ERR")
	})
	steps, err := g.moveWhileEndstopOpen(&mockedEndstop{})
	gobottest.Assert(t, steps, 0)
	gobottest.Refute(t, err, nil)

	g.Microstepping = 1
	steps, err = g.moveWhileEndstopOpen(&mockedEndstop{})
	gobottest.Assert(t, steps, 0)
	gobottest.Refute(t, err, nil)
}

func TestStepperMotorMoveWhileEndstopOpen(t *testing.T) {
	a := newGpioTestAdaptor()
	g := NewStepperMotorDriver(a, "1", "2", "3")
	mock := &mockedEndstop{
		err:      nil,
		detected: false,
		counter:  0,
	}

	steps, err := g.moveWhileEndstopOpen(mock)
	gobottest.Assert(t, err, nil)
	gobottest.Assert(t, steps, 9)
}

func TestStepperMotorMoveCenterAndMax(t *testing.T) {
	g := initStepperMotorDriver()
	g.motorCalibrated = true
	g.maxPosition = 110

	g.CurrentPosition = 20
	err := g.Center()
	gobottest.Assert(t, err, nil)
	gobottest.Assert(t, math.Abs(g.CurrentPosition-55) < 1.8, true)

	g.CurrentPosition = 90
	err = g.Center()
	gobottest.Assert(t, math.Abs(g.CurrentPosition-55) < 1.8, true)

	err = g.Max()
	gobottest.Assert(t, math.Abs(g.CurrentPosition-110) < 1.8, true)
	gobottest.Assert(t, err, nil)
}

func TestStepperMotorMoveMinMaxEndStops(t *testing.T) {
	a := newGpioTestAdaptor()
	g := NewStepperMotorDriver(a, "1", "2", "3")
	mock := &mockedEndstop{
		detected: false, counter: 0,
	}
	g.minLimitSwitch = mock
	g.maxLimitSwitch = mock

	err := g.Min()
	gobottest.Assert(t, err, nil)
	gobottest.Assert(t, g.CurrentPosition, float64(0))
	gobottest.Assert(t, g.motorCalibrated, true)

	mock.counter = -191
	err = g.Max()
	gobottest.Assert(t, err, nil)
	gobottest.Assert(t, math.Abs(g.CurrentPosition-360) < 1.8, true)

	mock.err = errors.New("ERR")
	err = g.Min()
	gobottest.Refute(t, err, nil)
	err = g.Max()
	gobottest.Refute(t, err, nil)

	a.TestAdaptorDigitalWrite(func() (err error) {
		return errors.New("ERR")
	})
	err = g.Min()
	gobottest.Refute(t, err, nil)
	err = g.Max()
	gobottest.Refute(t, err, nil)
}

func TestStepperMotorSingleStepDetectedError(t *testing.T) {
	a := newGpioTestAdaptor()
	g := NewStepperMotorDriver(a, "1", "2", "3")
	mock := &mockedEndstop{
		detected: false, counter: 0,
	}
	g.minLimitSwitch = mock
	for x := 0; x < 9; x++ {
		err := g.singleStep(time.Duration(5), true)
		gobottest.Assert(t, err, nil)
	}
	err := g.singleStep(time.Duration(5), true)
	gobottest.Refute(t, err, nil)
}
