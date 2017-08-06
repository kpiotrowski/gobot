package gpio

import (
	"gobot.io/x/gobot"
)

type LimitSwitchDriver struct {
	pin        string
	name       string
	DefaultOpen bool
	connection DigitalReader
	gobot.Eventer
}

//Return new LimitSwitch Driver fir given DigitalReader and pin
//By default it's configured for end stops which are open if end is not detected (most of mechanical end stops)
//If you are using different switch (ex. optical) set DefaultOpen = false
func NewLimitSwitchDriver(a DigitalReader, pin string) *LimitSwitchDriver{
	l := &LimitSwitchDriver{
		name:       gobot.DefaultName("LimitSwitch"),
		connection: a,
		pin:        pin,
		DefaultOpen: true,
		Eventer:    gobot.NewEventer(),
	}

	l.AddEvent(Error)
	l.AddEvent(EndDetected)

	return l
}

func (l *LimitSwitchDriver) EndDetected() (bool, error) {
	newValue, err := l.connection.DigitalRead(l.Pin())
	if err != nil {
		l.Publish(Error, err)
		return false, err
	}
	if (l.DefaultOpen == true && newValue == 1) || (l.DefaultOpen == false && newValue == 0) {
		l.Publish(EndDetected, newValue)
		return true, nil
	} else {
		return false, nil
	}
}

// Name returns the LimitSwitchDriver name
func (l *LimitSwitchDriver) Name() string { return l.name }

// SetName sets the LimitSwitchDriver name
func (l *LimitSwitchDriver) SetName(n string) { l.name = n }

// Pin returns the LimitSwitchDriver pin
func (l *LimitSwitchDriver) Pin() string { return l.pin }

// Connection returns the LimitWitchDriver Connection
func (l *LimitSwitchDriver) Connection() gobot.Connection { return l.connection.(gobot.Connection) }

