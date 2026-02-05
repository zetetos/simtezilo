package app

import (
	"github.com/zetetos/simtezilo/app/kinematics"
	"github.com/zetetos/simtezilo/app/signal"
)

func (a *App) sendTelemetryChartData() {
	if a.webUI == nil {
		return
	}

	if !a.webUI.HasActiveClients() {
		return
	}

	if a.gtClient.Telemetry.Flags().GamePaused {
		return
	}

	if a.gtClient.Finished {
		return
	}

	if a.kinematics.Current.SequenceID == a.webSequenceID {
		return
	}

	a.webSequenceID = a.kinematics.Current.SequenceID

	go func() {
		a.telemetryChartFeed <- map[string]float32{
			"computeTime":                 float32(a.kinematics.Last.ComputeTime.Microseconds()),
			"seq":                         float32(a.state.current.sequenceNumber),
			"timeOfDay":                   float32(a.gtClient.Telemetry.TimeOfDay().Milliseconds()),
			"throttleInput":               a.gtClient.Telemetry.ThrottleInputPercent(),
			"throttleOutput":              a.gtClient.Telemetry.ThrottleOutputPercent(),
			"brakeInput":                  a.gtClient.Telemetry.BrakeInputPercent(),
			"brakeOutput":                 a.gtClient.Telemetry.BrakeOutputPercent(),
			"rpm":                         a.gtClient.Telemetry.EngineRPM(),
			"speed":                       a.gtClient.Telemetry.GroundSpeedKPH(),
			"gear":                        float32(a.kinematics.Current.TransmissionGear),
			"fuelUsagePerKm":              float32(a.fuelRange.UsageRatePerKm()),
			"fuelRangeKm":                 float32(a.fuelRange.DistanceMetres() / 1000),
			"fuelRangeLaps":               float32(a.fuelRange.DistanceLaps(a.circuit.LengthMetres())),
			"tyreTempFL":                  a.gtClient.Telemetry.TyreTemperatureCelsius().FrontLeft,
			"tyreTempFR":                  a.gtClient.Telemetry.TyreTemperatureCelsius().FrontRight,
			"tyreTempRL":                  a.gtClient.Telemetry.TyreTemperatureCelsius().RearLeft,
			"tyreTempRR":                  a.gtClient.Telemetry.TyreTemperatureCelsius().RearRight,
			"surgeGforce":                 float32(a.kinematics.Current.SixDOFTranslation.Acceleration.Surge) / kinematics.GravityConstant,
			"surgeGforceCalc":             float32(a.kinematics.Current.SurgeCalculated) / kinematics.GravityConstant,
			"SixDOFTranslationalJerk":     float32(a.kinematics.Current.SixDOFTranslation.Jerk),
			"SixDOFTranslationalSnap":     float32(a.kinematics.Current.SixDOFTranslation.Snap),
			"SixDOFTranslationalJerkCalc": float32(a.kinematics.Current.SixDOFTranslationCalc.Jerk),
			"SixDOFTranslationalSnapCalc": float32(a.kinematics.Current.SixDOFTranslationCalc.Snap),
			"SixDOFTranslationalAccelX":   float32(a.kinematics.Current.SixDOFTranslationCalc.Acceleration.X),
			"SixDOFTranslationalAccelY":   float32(a.kinematics.Current.SixDOFTranslationCalc.Acceleration.Y),
			"SixDOFTranslationalAccelZ":   float32(a.kinematics.Current.SixDOFTranslationCalc.Acceleration.Z),
			"SixDOFRotationalJerk":        float32(a.kinematics.Current.SixDOFRotationCalc.Jerk),
			"SixDOFRotationalSnap":        float32(a.kinematics.Current.SixDOFRotationCalc.Snap),
			"SixDOFRotationalAccelX":      float32(a.kinematics.Current.SixDOFRotationCalc.Acceleration.X),
			"SixDOFRotationalAccelY":      float32(a.kinematics.Current.SixDOFRotationCalc.Acceleration.Y),
			"SixDOFRotationalAccelZ":      float32(a.kinematics.Current.SixDOFRotationCalc.Acceleration.Z),
			"synthOutputAmplitude":        float32(signal.Abs(float64(a.kinematics.Current.SynthOutputAmplitude))),
			"synthOutputFrequency":        float32(a.kinematics.Current.SynthOutputFrequency),
			"engineVibrationEnabled": func() float32 {
				if a.gtClient.Telemetry.EngineRPM() > 0 {
					return 1
				}

				return 0
			}(),
		}
	}()
}
