//lint:file-ignore SA4026

package config

import appHaptics "github.com/zetetos/simtezilo/app/haptics"

func defaultConfig() *viperConfig {
	return &viperConfig{
		Schema:        "https://simtezilo.com/schemas/config/v1.0.0/config.schema.json",
		SchemaVersion: "1.0.0",
		App: &app{
			Language:      "en",
			Accent:        "us",
			LogLevel:      "info",
			BaseDir:       ".",
			VehicleDBFile: "",
			EnabledWebUI:  true,
			WebUIPort:     80,
			Update: &update{
				BaseURL:              "https://simtezilo.com/releases",
				Channel:              "stable",
				AutoCheck:            false,
				AutoInstall:          false,
				CheckIntervalMinutes: 60,
			},
			EnableDevTools: false,
		},
		Hardware: &hardware{
			Model:              "none",
			DisplayOrientation: 0,
		},
		Haptics: &haptics{
			EnableReplay:                 false,
			DynamicTransmissionFeedback:  true,
			DynamicTransmissionCurve:     150,
			DynamicTransmissionGforceMax: 2.0,
			JerkCurve:                    190,
			JerkMax:                      37,
			SnapCurve:                    310,
			SnapMax:                      90,
			PulseMaxAmplitude:            1,
			PulseMaxFrequencyHz:          60,
			PulseMinFrequencyHz:          16,
			_pulseWidthMax:               0.5,
			_pulseWidthMin:               0.1,
			EngineProfiles: map[string]appHaptics.EngineProfile{
				"s1":                  {PrimaryBalance: 0.15, SecondaryBalance: 0.250, Gain: +0.00, PulseScale: 1.00}, // Racing Kart 125 Shifter
				"i2":                  {PrimaryBalance: 0.65, SecondaryBalance: 0.850, Gain: -0.75, PulseScale: 1.00}, // Fiat 500 F '68
				"i3":                  {PrimaryBalance: 0.95, SecondaryBalance: 0.850, Gain: -1.50, PulseScale: 1.00}, // Daihatsu COPEN RJ VGT
				"i4":                  {PrimaryBalance: 0.76, SecondaryBalance: 0.800, Gain: -4.25, PulseScale: 0.75}, // Toyota Supra GT500 '97
				"i4_rmed":             {PrimaryBalance: 0.76, SecondaryBalance: 0.800, Gain: -2.75, PulseScale: 0.68}, // Honda NSX CONCEPT-GT '16
				"i5":                  {PrimaryBalance: 0.70, SecondaryBalance: 0.700, Gain: -1.25, PulseScale: 0.60}, // Audi Sport quattro S1 Pikes Peak '87
				"i6":                  {PrimaryBalance: 0.90, SecondaryBalance: 0.950, Gain: -1.00, PulseScale: 0.68}, // Toyota GR Supra Racing Concept '18
				"i8":                  {PrimaryBalance: 0.92, SecondaryBalance: 0.980, Gain: -1.50, PulseScale: 0.32}, // Mercedes-Benz W 196 R '55
				"v4":                  {PrimaryBalance: 0.90, SecondaryBalance: 0.850, Gain: -0.50, PulseScale: 1.00}, // Porsche 919 Hybrid '16
				"v6":                  {PrimaryBalance: 0.80, SecondaryBalance: 0.950, Gain: -1.25, PulseScale: 0.50}, // REF v6_b60_c120_rstd
				"v6_b15_c120_rstd":    {PrimaryBalance: 0.85, SecondaryBalance: 0.960, Gain: -1.50, PulseScale: 0.50}, // Volkswagen GTI VGT (Gr.3)
				"v6_b60_c120_rstd":    {PrimaryBalance: 0.80, SecondaryBalance: 0.950, Gain: -1.25, PulseScale: 0.50}, // Renault R.S.01 GT3 '16"
				"v6_b75_c120_rstd":    {PrimaryBalance: 0.86, SecondaryBalance: 0.720, Gain: -2.00, PulseScale: 0.60}, // Honda NSX Gr.3
				"v6_b80_c120_rhigh":   {PrimaryBalance: 0.06, SecondaryBalance: 0.930, Gain: -0.50, PulseScale: 0.44}, // Mclaren MP4/4 '88
				"v6_b90_c120_rstd":    {PrimaryBalance: 0.72, SecondaryBalance: 0.720, Gain: -1.75, PulseScale: 0.60}, // Toyota GR010 HYBRID '21
				"v6_b90_c120_rhigh":   {PrimaryBalance: 0.85, SecondaryBalance: 0.960, Gain: -1.50, PulseScale: 0.31}, // Red Bull X2019 Competition
				"v6_b120_c120_rstd":   {PrimaryBalance: 0.85, SecondaryBalance: 0.960, Gain: -1.75, PulseScale: 0.70}, // Audi R18 TDI '11
				"v8":                  {PrimaryBalance: 0.95, SecondaryBalance: 0.980, Gain: -3.00, PulseScale: 0.40}, // REF v8_b90_c90_rstd
				"v8_b90_c180_rstd":    {PrimaryBalance: 0.72, SecondaryBalance: 0.980, Gain: -1.00, PulseScale: 0.44}, // Nissan R92CP '92
				"v8_b90_c180_rmed":    {PrimaryBalance: 0.80, SecondaryBalance: 0.980, Gain: -0.25, PulseScale: 0.40}, // Toyota TS030 Hybrid '12
				"v8_b90_c90_rstd":     {PrimaryBalance: 0.95, SecondaryBalance: 0.980, Gain: -3.00, PulseScale: 0.40}, // BMW M6 GT3 Endurance Model '16
				"v10":                 {PrimaryBalance: 0.86, SecondaryBalance: 0.940, Gain: -2.25, PulseScale: 0.40}, // REF v10_b90_c72_rstd
				"v10_b72_c72_rmed":    {PrimaryBalance: 0.86, SecondaryBalance: 0.950, Gain: -1.75, PulseScale: 0.25}, // Lexus LFA '10
				"v10_b90_c72_rstd":    {PrimaryBalance: 0.86, SecondaryBalance: 0.940, Gain: -2.25, PulseScale: 0.40}, // Lamborghini Huracán GT3 '15
				"v12":                 {PrimaryBalance: 0.90, SecondaryBalance: 0.990, Gain: -0.75, PulseScale: 0.30}, // REF v12_b60_c120_rstd
				"v12_b60_c120_rstd":   {PrimaryBalance: 0.90, SecondaryBalance: 0.990, Gain: -0.75, PulseScale: 0.30}, // McLaren F1 GTR - BMW '95
				"v12_b60_c120_rhigh":  {PrimaryBalance: 0.90, SecondaryBalance: 0.990, Gain: -0.75, PulseScale: 0.30}, // Gran Turismo 1500T-A
				"v12_b75_c120_rstd":   {PrimaryBalance: 0.85, SecondaryBalance: 0.990, Gain: -0.75, PulseScale: 0.30}, // Aston Martin V12 Vantage GT3 '12
				"v12_b100_c120_rstd":  {PrimaryBalance: 0.77, SecondaryBalance: 0.980, Gain: -0.75, PulseScale: 0.33}, // Peugeot 908 HDi FAP '10
				"v12_b144_c120_rhigh": {PrimaryBalance: 0.80, SecondaryBalance: 0.970, Gain: -0.75, PulseScale: 0.20}, // Dodge SRT Tomahawk X VGT
				"w16":                 {PrimaryBalance: 0.80, SecondaryBalance: 0.990, Gain: -0.75, PulseScale: 0.30}, // Bugatti Veyron Gr.4
				"h2":                  {PrimaryBalance: 0.70, SecondaryBalance: 0.950, Gain: -8.00, PulseScale: 1.00}, // Toyota Sports 800 '65
				"h4":                  {PrimaryBalance: 0.80, SecondaryBalance: 0.950, Gain: -1.75, PulseScale: 0.80}, // Subaru BRZ GT300 '21
				"h6":                  {PrimaryBalance: 0.92, SecondaryBalance: 0.970, Gain: -1.25, PulseScale: 0.60}, // Porsche 911 RSR (991) '17
				"h12":                 {PrimaryBalance: 0.98, SecondaryBalance: 0.990, Gain: -3.00, PulseScale: 0.25}, // Porsche 917K '70
				"k2":                  {PrimaryBalance: 0.85, SecondaryBalance: 0.960, Gain: -0.75, PulseScale: 0.20}, // RE Amemiya FD3S RX-7
				"k4":                  {PrimaryBalance: 0.75, SecondaryBalance: 0.800, Gain: +0.00, PulseScale: 0.10}, // Mazda 787B '91
			},
		},
		PitRadio: &pitRadio{
			Enabled:               false,
			Output:                "log",
			MessageSendIntervalMs: 2000,
			Notifications: &notifications{
				EnableRaceProgress:      true,
				RaceProgressMinLaps:     10,
				RaceProgressIntervalPc:  25,
				EnableRaceLaps:          true,
				RaceLapsIntervalLaps:    1,
				RaceLapsCountdownLaps:   3,
				EnableLapTimes:          true,
				LapTimesMaxDeltaSeconds: 2.0,
				EnableCircuitMatching:   false,
			},
			Discord: &discord{
				Token:          "",
				GuildID:        "",
				ChannelID:      "",
				VoiceChannelID: "",
			},
			FuelMonitoring: &fuelMonitoring{
				Enabled:                 true,
				PreWarnNotifyLaps:       2.0,
				StrategyNotifyLaps:      5.0,
				RangeSafetyMarginLaps:   0.2,
				RangeSafetyMarginMetres: 750,
			},
			TyreMonitoring: &tyreMonitoring{
				Enabled:                    true,
				TemperatureOptimalCelsius:  81,
				TemperatureOperatingWindow: 6,
				TemperatureMarginCelsius:   3,
			},
		},
		Synthesizer: &Synthesizer{
			InternalSampleRateHz:      8000,
			OutputSampleRateHz:        32000,
			OutputFile:                "",
			MasterMute:                false,
			MasterGain:                0.00,
			ChannelMute:               []bool{false, false},
			ChannelGain:               []float64{-30.00, -30.00},
			GainIncrement:             0.25,
			ChassisMute:               false,
			ChassisGain:               0.00,
			TransmissionMute:          false,
			TransmissionGain:          0.00,
			TransmissionGainMinRace:   -3.00,
			TransmissionGainMinStreet: -6.00,
			EngineMute:                false,
			EngineGain:                -4.25,
			EnableEq:                  []bool{false, false},
			EqBands: [][]EQBand{
				{ // Channel 0 (Left)
					{Frequency: 8, Gain: 0.0, Q: 2.0},
					{Frequency: 10, Gain: 0.0, Q: 2.0},
					{Frequency: 13, Gain: 0.0, Q: 2.0},
					{Frequency: 17, Gain: 0.0, Q: 2.0},
					{Frequency: 22, Gain: 0.0, Q: 2.0},
					{Frequency: 30, Gain: 0.0, Q: 2.0},
					{Frequency: 40, Gain: 0.0, Q: 2.0},
					{Frequency: 50, Gain: 0.0, Q: 2.0},
				},
				{ // Channel 1 (Right)
					{Frequency: 8, Gain: 0.0, Q: 2.0},
					{Frequency: 10, Gain: 0.0, Q: 2.0},
					{Frequency: 13, Gain: 0.0, Q: 2.0},
					{Frequency: 17, Gain: 0.0, Q: 2.0},
					{Frequency: 22, Gain: 0.0, Q: 2.0},
					{Frequency: 30, Gain: 0.0, Q: 2.0},
					{Frequency: 40, Gain: 0.0, Q: 2.0},
					{Frequency: 50, Gain: 0.0, Q: 2.0},
				},
			},
		},
		Telemetry: &Telemetry{
			Source: "udp://255.255.255.255:33739",
		},
	}
}
