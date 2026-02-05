package hardware

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3/gpioioctl"
)

const (
	debouncedLow  uint8 = 0x00
	debouncedHigh uint8 = 0xFF

	gpioReadSampleRate = 5 * time.Millisecond // Rate at which GPIO pin is sampled for debouncing

	autoRepeatInitialDelay = 500 * time.Millisecond // Initial delay before button auto-repeat starts
	autoRepeatMaxTime      = 2 * time.Second        // Time to reach max auto-repeat rate
	autoRepeatInitialRate  = 200 * time.Millisecond // Initial auto-repeat rate
	autoRepeatMaxRate      = 50 * time.Millisecond  // Fastest auto-repeat rate
)

// buttonAutoRepeat manages auto-repeat state for button presses.
type buttonAutoRepeat struct {
	sync.Mutex

	cancel context.CancelFunc
	active bool
}

// getAutoRepeatManager returns the singleton auto-repeat manager instance.
func getAutoRepeatManager() *buttonAutoRepeat {
	type container struct {
		instance *buttonAutoRepeat
		once     sync.Once
	}

	static := &container{}

	static.once.Do(func() {
		static.instance = &buttonAutoRepeat{}
	})

	return static.instance
}

// OnGPIOButtonPressed sets up a GPIO pin to call the provided handler function when the button is pressed.
func OnGPIOButtonPressed(n int, handler func()) {
	go func() {
		pin := setupGPIOPin(n)
		monitorGPIOButton(pin, handler)
	}()
}

// setupGPIOPin prepares a GPIO pin for input with pull-up resistor.
func setupGPIOPin(n int) gpio.PinIO { //nolint:ireturn // intended to return gpio.PinIO
	pinName := fmt.Sprintf("GPIO%d", n)

	// Try to get the pin from gpioioctl first (gpiod character device interface)
	// This is preferred on modern Linux kernels where sysfs is deprecated
	var pin gpio.PinIO

	if len(gpioioctl.Chips) > 0 {
		// Look for the pin in gpioioctl chips
		for _, chip := range gpioioctl.Chips {
			if p := chip.ByName(pinName); p != nil {
				pin = p

				break
			}
		}
	}

	// Fallback to gpioreg if gpioioctl didn't find the pin
	if pin == nil {
		pin = gpioreg.ByName(pinName)
	}

	if pin == nil {
		log.Fatalf("Failed to find pin %s", pinName)
	}

	err := pin.In(gpio.PullUp, gpio.BothEdges)
	if err != nil {
		log.Fatal(err)
	}

	return pin
}

// monitorGPIOButton continuously monitors a GPIO pin for button presses with debouncing.
func monitorGPIOButton(pin gpio.PinIO, handler func()) {
	lastStableLevel := gpio.High
	gpioStates := debouncedHigh

	ticker := time.NewTicker(gpioReadSampleRate)
	defer ticker.Stop()

	for range ticker.C {
		// Read current pin state
		currentRead := pin.Read()

		// Shift in the new reading
		gpioStates <<= 1
		if currentRead == gpio.High {
			gpioStates |= 1
		}

		// Check for stable low (button pressed)
		if gpioStates == debouncedLow && lastStableLevel == gpio.High {
			lastStableLevel = gpio.Low

			handler()
			startAutoRepeat(pin, handler)
		}

		// Check for stable high (button released)
		if gpioStates == debouncedHigh && lastStableLevel == gpio.Low {
			lastStableLevel = gpio.High

			stopActiveAutoRepeat()
		}
	}
}

// stopActiveAutoRepeat stops any active auto-repeat and marks the system as available.
func stopActiveAutoRepeat() {
	manager := getAutoRepeatManager()

	manager.Lock()
	defer manager.Unlock()

	if manager.active && manager.cancel != nil {
		manager.cancel()
	}

	manager.active = false
	manager.cancel = nil
}

// startAutoRepeat starts a new auto-repeat session, stopping any existing one.
func startAutoRepeat(pin gpio.PinIO, function func()) {
	stopActiveAutoRepeat()

	// Start a new auto-repeat handler
	ctx, cancel := context.WithCancel(context.Background())

	manager := getAutoRepeatManager()
	manager.Lock()
	manager.cancel = cancel
	manager.active = true
	manager.Unlock()

	go func() {
		defer stopActiveAutoRepeat()

		// Sample the button during initial delay to ensure it stays pressed
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		deadline := time.Now().Add(autoRepeatInitialDelay)

		for {
			select {
			case <-ticker.C:
				// If button released during initial delay, cancel auto-repeat
				if pin.Read() == gpio.High {
					return
				}
				// If we've reached the deadline and button still pressed, start repeating
				if time.Now().After(deadline) {
					goto startRepeating
				}
			case <-ctx.Done():
				return
			}
		}

	startRepeating:
		repeatStartTime := time.Now()

		for {
			// Finish auto repeat if the button has been released
			if pin.Read() == gpio.High {
				return
			}

			function()

			elapsed := time.Since(repeatStartTime)

			// Linear acceleration of repeat rate
			acceleration := min(
				1.0,
				float64(elapsed)/float64(autoRepeatMaxTime),
			)

			// Interpolate between initial and accelerated repeat rates
			repeatRate := float64(autoRepeatInitialRate) - (float64(autoRepeatInitialRate-autoRepeatMaxRate) * acceleration)

			select {
			case <-time.After(time.Duration(repeatRate)):
			case <-ctx.Done():
				return
			}
		}
	}()
}
