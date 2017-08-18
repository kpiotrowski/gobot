package gpio

import (
	"fmt"
	"github.com/pkg/errors"
	"gobot.io/x/gobot"
	"math"
	"time"
)

/*
This driver allows you to control stepper motors using hardware driver
(with 3 pins - enable, direction and step). tested on DRV8825 driver

This driver also support end detection using hardware endstops.
You can define 2 hardware endstops or 1 for start and MaxPosition (software end stop)

TODO: implement motor acceleration
*/

const (
	STATE_ENABLED  = iota
	STATE_DISABLED = iota
)

type moveDirection byte

var (
	ErrNotCalibrated          = errors.New("Stepper motor was not calibrated")
	ErrIncorrectMicrostepping = errors.New("Stepper motor: incorrect microstepping settings")
	ErrIncorrectStepsPerTurn  = errors.New("Stepper motor: incorrect steps per turn")
	ErrIncorrectSpeed         = errors.New("Stepper motor: incorrect speed")
)

type StepperMotorDriver struct {
	name          string
	connection    DigitalWriter
	DirectionPin  string
	StepPin       string
	EnablePin     string
	StepsPerTurn  int64 //default 200 - most of common motors
	Microstepping int64 //default 1 - no microstepping (value depends on your hardware driver configuration)

	EnableLevel        byte    //0 - low value enable, 1 - high value enable
	DirectionInversion bool    //def. false - change if your motor rotates in wrong direction :)
	Speed              float64 //RPM def. 30
	CurrentPosition    float64
	CurrentState       int
	CheckEndWhenMoving bool //def. true - use it to protect your hardware. Driver check if endstop is closed after every step and return error if it's true

	minLimitSwitch  LimitSwitchDriverInterface
	maxLimitSwitch  LimitSwitchDriverInterface
	maxPosition     float64 //angle ex 1080 if you allow 3 turns
	motorCalibrated bool    //If moved to min - motor is calibrated

	errorsList []error
	gobot.Commander
}

// NewStepperMotorDriver return new driver given digital writer and pins
// Remember that you need to have hardware stepper motor driver!
//
// Adds the following API Commands:
// 	"Move" - See ServoDriver.Move
// If endstops were defined:
//		"Min" - See ServoDriver.Min
//		"Center" - See ServoDriver.Center
//		"Max" - See ServoDriver.Max
func NewStepperMotorDriver(a DigitalWriter, stepPin, dirPin, enablePin string) *StepperMotorDriver {
	s := &StepperMotorDriver{
		name:               gobot.DefaultName("StepperMotor"),
		connection:         a,
		StepPin:            stepPin,
		DirectionPin:       dirPin,
		EnablePin:          enablePin,
		StepsPerTurn:       2E2,
		Microstepping:      1,
		EnableLevel:        0,
		DirectionInversion: false,
		Speed:              30,
		CurrentPosition:    0,
		CurrentState:       STATE_DISABLED,
		motorCalibrated:    false,
		CheckEndWhenMoving: true,
		Commander:          gobot.NewCommander(),
	}

	s.AddCommand("Move", func(params map[string]interface{}) interface{} {
		fmt.Println(params)
		angle := params["angle"].(float64)
		return s.Move(angle)
	})

	s.AddCommand("Min", func(params map[string]interface{}) interface{} {
		return s.Min()
	})
	s.AddCommand("Center", func(params map[string]interface{}) interface{} {
		return s.Center()
	})
	s.AddCommand("Max", func(params map[string]interface{}) interface{} {
		return s.Max()
	})
	return s
}

// Name returns the StepperMotorDriver name
func (s *StepperMotorDriver) Name() string { return s.name }

// SetName sets the StepperMotorDriver name
func (s *StepperMotorDriver) SetName(n string) { s.name = n }

// Connection returns the StepperMotorDriver Connection
func (s *StepperMotorDriver) Connection() gobot.Connection { return s.connection.(gobot.Connection) }

// Start implements the Driver interface
func (s *StepperMotorDriver) Start() (err error) { return }

// Halt implements the Driver interface
func (s *StepperMotorDriver) Halt() (err error) { return }

// Configure end detection
func (s *StepperMotorDriver) ConfigureEndDetection(min, max LimitSwitchDriverInterface, maxPosition float64) (err error) {
	s.minLimitSwitch = min
	s.maxLimitSwitch = max
	s.maxPosition = maxPosition
	if s.checkEndstopsDefined() {
		return nil
	}
	return ErrStepperMotorEndstopUnsupported
}

// Off Turns the stepper motor off (set enablePin value = 1-enableLevel)
func (s *StepperMotorDriver) Off() (err error) {
	err = s.connection.DigitalWrite(s.EnablePin, 1-s.EnableLevel)
	if err == nil {
		s.CurrentState = STATE_DISABLED
	}
	return err
}

// On turns the stepper motor on (set enablePin value = enableLevel)
func (s *StepperMotorDriver) On() (err error) {
	err = s.connection.DigitalWrite(s.EnablePin, s.EnableLevel)
	if err == nil {
		s.CurrentState = STATE_ENABLED
	}
	return err
}

// Return the last error from StepperMotorDriver errorList. Check error if executing async methods
func (s *StepperMotorDriver) GetLastError() (err error) {
	l := len(s.errorsList)
	if l > 0 {
		err = s.errorsList[l-1]
		s.errorsList = s.errorsList[:l-1]
	}
	return
}

// Move stepper motor for an specified angle <-x, y>.
// Example: move motor for -45 deg
func (s *StepperMotorDriver) Move(angle float64) (err error) {
	if angle == 0 {
		return
	} else if angle < 0 {
		err = s.setDirection(0)
	} else {
		err = s.setDirection(1)
	}
	if err != nil {
		return
	}

	steps := int(math.Abs(angle) * float64(s.StepsPerTurn) * float64(s.Microstepping) / 360)
	steps, err = s.moveMicroSteps(steps)
	moved_angle := float64(steps) * 360 / float64(s.StepsPerTurn) / float64(s.Microstepping)
	if angle < 0 {
		moved_angle *= -1
	}
	s.CurrentPosition += moved_angle
	return
}

// Start move job without waiting for finishing. You can wait for job finish using created channel
// Example use: you want to move 2 steppers together and then wait
func (s *StepperMotorDriver) MoveAsync(angle float64, sem chan bool) {
	go func() {
		err := s.Move(angle)
		if err != nil {
			s.pushError(err)
		}
		sem <- true
	}()
}

//For this functions you need to define 2 hardware end detections (start + stop)
//or 1 hardware detection (start) + MaxPosition (software end)
//Example use: min-hardware, max-software - move motor to min and then to center (home position)

// Move stepper motor to min position (min endstop)
func (s *StepperMotorDriver) Min() (err error) {
	if s.checkEndstopsDefined() {
		err = s.setDirection(0)
		if err != nil {
			return
		}
		_, err = s.moveWhileEndstopOpen(s.minLimitSwitch)
		if err != nil {
			return
		}
		s.motorCalibrated = true
		s.CurrentPosition = 0
	} else {
		err = ErrStepperMotorEndstopUnsupported
	}
	return
}

// Move stepper motor to max position (max endstop or maxPosition)
// You need to move to MIN position before to calibrate motor
func (s *StepperMotorDriver) Max() (err error) {
	if s.motorCalibrated == false {
		return ErrNotCalibrated
	}
	steps := 0
	if s.maxLimitSwitch != nil {
		err = s.setDirection(1)
		if err != nil {
			return
		}
		steps, err = s.moveWhileEndstopOpen(s.maxLimitSwitch)
		moved_angle := float64(steps) / float64(s.StepsPerTurn) / float64(s.Microstepping) * 360
		s.CurrentPosition += moved_angle
		s.maxPosition = s.CurrentPosition
	} else if s.maxPosition > 0 && s.CurrentPosition < s.maxPosition {
		err = s.Move(s.maxPosition - s.CurrentPosition)
	}
	return
}

// Move stepper motor to center. Remember to move MIN before running this method
// MaxPosition must be defined, but you can run Max() to calculate maxPosition :)
func (s *StepperMotorDriver) Center() (err error) {
	if s.motorCalibrated == false || s.maxPosition <= 0 {
		return ErrNotCalibrated
	} else {
		angle := (s.maxPosition / 2) - s.CurrentPosition
		err = s.Move(angle)
	}
	return
}

//Rotate motor for specified number of microsteps. Return number of steps it made and error
func (s *StepperMotorDriver) moveMicroSteps(steps int) (int, error) {
	sleep, err := s.calculateSleep()
	if err != nil {
		return 0, err
	}

	for i := 0; i < steps; i++ {
		err = s.singleStep(*sleep, true)
		if err != nil {
			return i, err
		}
	}
	return steps, nil
}

//Rotate while endstop is open. Return number of steps it made and error
func (s *StepperMotorDriver) moveWhileEndstopOpen(endstop LimitSwitchDriverInterface) (int, error) {
	sleep, err := s.calculateSleep()
	if err != nil {
		return 0, err
	}
	steps := 0

	end, err := endstop.EndDetected()
	for err == nil && !end {
		err = s.singleStep(*sleep, false)
		if err != nil {
			return steps, err
		}
		end, err = endstop.EndDetected()
		steps++
	}
	return steps, err
}

func (s *StepperMotorDriver) calculateSleep() (*time.Duration, error) {
	if s.Microstepping < 1 {
		return nil, ErrIncorrectMicrostepping
	}
	if s.StepsPerTurn < 1 {
		return nil, ErrIncorrectStepsPerTurn
	}
	if s.Speed <= 0 {
		return nil, ErrIncorrectSpeed
	}
	micro_steps_per_turn := s.Microstepping * s.StepsPerTurn
	micro_steps_per_sec := float64(micro_steps_per_turn) * s.Speed / 60
	sleep := time.Duration(1e9 / micro_steps_per_sec / 2)
	return &sleep, nil
}

func (s *StepperMotorDriver) singleStep(d time.Duration, check_end bool) (err error) {
	err = s.connection.DigitalWrite(s.StepPin, 0)
	time.Sleep(d)
	if err != nil {
		return
	}
	err = s.connection.DigitalWrite(s.StepPin, 1)
	time.Sleep(d)
	if s.CheckEndWhenMoving && check_end {
		endstops := []LimitSwitchDriverInterface{s.minLimitSwitch, s.maxLimitSwitch}
		for _, endstop := range endstops {
			if endstop != nil {
				if end, _ := endstop.EndDetected(); end {
					return ErrStepperMotorOutOfRange
				}
			}
		}
	}
	return
}

//Check if endstops were correctly defined
func (s *StepperMotorDriver) checkEndstopsDefined() bool {
	if s.minLimitSwitch == nil {
		return false
	}
	if s.maxLimitSwitch != nil || s.maxPosition > 0 {
		return true
	}
	return false
}

func (s *StepperMotorDriver) setDirection(dir byte) (err error) {
	if s.DirectionInversion {
		dir = 1 - dir
	}
	return s.connection.DigitalWrite(s.DirectionPin, dir)
}

func (s *StepperMotorDriver) pushError(err error) {
	s.errorsList = append(s.errorsList, err)
}
