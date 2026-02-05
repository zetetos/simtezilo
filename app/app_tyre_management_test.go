package app //nolint:testpackage // white-box testing

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	gttelemetry "github.com/zetetos/gt-telemetry/v2"
	"github.com/zetetos/gt-telemetry/v2/pkg/models"
	"github.com/zetetos/simtezilo/app/config"
	"github.com/zetetos/simtezilo/app/pitradio"
	"github.com/zetetos/simtezilo/app/tyres"
)

type TyreIntegrationTestSuite struct {
	suite.Suite

	app      *App
	pitRadio pitRadioMock
}

func TestTyreIntegrationTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(TyreIntegrationTestSuite))
}

func (suite *TyreIntegrationTestSuite) SetupTest() {
	gtClient, err := gttelemetry.New(gttelemetry.Options{})
	if err != nil {
		suite.FailNow("Failed to create GTClient", err)
	}

	suite.pitRadio = pitRadioMock{
		messages: []pitradio.Message{},
	}

	configJSON := []byte(`{
		"app": {
			"language": "en",
			"accent": "us"
		},
		"pitRadio": {
			"tyreTemperatureMonitoring": true,
			"tyreTemperatureOptimalCelsius": 81,
			"tyreTemperatureOperatingWindow": 6,
			"tyreTemperatureMarginCelsius": 3
		}
	}`)

	suite.app = &App{
		config:   config.NewFromJSON(configJSON, zerolog.Nop()),
		gtClient: gtClient,
		state: gameState{
			current: raceState{lapNumber: 5},
			last:    raceState{},
		},
		pitRadio:      &suite.pitRadio,
		pitRadioState: &pitRadioState{},
		i18n:          createTestI18n(),
	}
}

func (suite *TyreIntegrationTestSuite) TestNoNotificationWhenRateLimited() {
	// Arrange
	suite.app.pitRadioState.tyreState.lastTempNotifyTime = time.Now().Add(-25 * time.Second)

	// Act
	suite.app.notifyTyreTemperature()

	// Assert
	suite.Empty(suite.pitRadio.messages)
}

func (suite *TyreIntegrationTestSuite) TestNoNotificationWhenTyreMonitoringDisabled() {
	// Arrange
	suite.setupDisabledTyreTemperatureConfig()

	// Act
	suite.app.notifyTyreTemperature()

	// Assert
	suite.Empty(suite.pitRadio.messages)
}

func (suite *TyreIntegrationTestSuite) TestTyreOptimalConditionNotificationMessages() {
	// Arrange
	testCases := []struct {
		name            string
		temps           models.CornerSet
		expectedMessage string
		description     string
	}{
		{
			name: "All tyres optimal",
			temps: models.CornerSet{
				FrontLeft:  81.0,
				FrontRight: 81.0,
				RearLeft:   81.0,
				RearRight:  81.0,
			},
			expectedMessage: "Tyres optimal",
			description:     "Should report optimal when all tyres are in optimal range",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			// Create tyre attributes from the test temperatures
			attributes := tyres.New(
				suite.app.config.GetPitRadioTyreTemperatureOptimalCelsius(),
				suite.app.config.GetPitRadioTyreTemperatureOperatingWindow(),
				suite.app.config.GetPitRadioTyreTemperatureMarginCelsius(),
				testCase.temps,
			)

			// Act
			message := suite.app.generateTyreConditionMessage(attributes)

			// Assert
			suite.Equal(testCase.expectedMessage, message, testCase.description)
		})
	}
}

func (suite *TyreIntegrationTestSuite) TestTyreColdConditionNotificationMessages() { //nolint:dupl // Similar code for hot and cold tests
	// Arrange
	testCases := []struct {
		name            string
		temps           models.CornerSet
		expectedMessage string
		description     string
	}{
		{
			name: "All tyres cold",
			temps: models.CornerSet{
				FrontLeft:  65.0,
				FrontRight: 65.0,
				RearLeft:   65.0,
				RearRight:  65.0,
			},
			expectedMessage: "Tyres under temp",
			description:     "Should report under temp when all tyres are cold",
		},
		{
			name: "Individual front left cold",
			temps: models.CornerSet{
				FrontLeft:  65.0, // Cold
				FrontRight: 81.0, // Optimal
				RearLeft:   81.0, // Optimal
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 1 tyre is cold",
		},
		{
			name: "Individual front right cold",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 65.0, // Cold
				RearLeft:   81.0, // Optimal
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 1 tyre is cold",
		},
		{
			name: "Individual rear left cold",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 81.0, // Optimal
				RearLeft:   65.0, // Cold
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 1 tyre is cold",
		},
		{
			name: "Individual rear right cold",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 81.0, // Optimal
				RearLeft:   81.0, // Optimal
				RearRight:  65.0, // Cold
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 1 tyre is cold",
		},
		{
			name: "Left side cold",
			temps: models.CornerSet{
				FrontLeft:  65.0, // Cold
				FrontRight: 81.0, // Optimal
				RearLeft:   65.0, // Cold
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report under temp when only 2 tyres are cold on different axles",
		},
		{
			name: "Right side cold",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 65.0, // Cold
				RearLeft:   81.0, // Optimal
				RearRight:  65.0, // Cold
			},
			expectedMessage: "",
			description:     "Should not report under temp when only 2 tyres are cold on different axles",
		},
		{
			name: "Left diagonal cold",
			temps: models.CornerSet{
				FrontLeft:  65.0, // Cold
				FrontRight: 81.0, // Optimal
				RearLeft:   81.0, // Optimal
				RearRight:  65.0, // Cold
			},
			expectedMessage: "",
			description:     "Should not report under temp when only 2 tyres are cold on different axles",
		},
		{
			name: "Right diagonal cold",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 65.0, // Cold
				RearLeft:   65.0, // Cold
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report under temp when only 2 tyres are cold on different axles",
		},
		{
			name: "Front axle cold",
			temps: models.CornerSet{
				FrontLeft:  65.0, // Cold
				FrontRight: 65.0, // Cold
				RearLeft:   81.0, // Optimal
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "Tyres under temp",
			description:     "Should report general under temp when front axle cold condition",
		},
		{
			name: "Rear axle cold",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Hot
				FrontRight: 81.0, // Hot
				RearLeft:   65.0, // Cold
				RearRight:  65.0, // Cold
			},
			expectedMessage: "Tyres under temp",
			description:     "Should report general under temp when front axle cold condition",
		},
		{
			name: "Front axle and rear left cold",
			temps: models.CornerSet{
				FrontLeft:  65.0, // Cold
				FrontRight: 65.0, // Cold
				RearLeft:   65.0, // Cold
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "Tyres under temp",
			description:     "Should report general under temp when 3 or more individual tyres are cold",
		},
		{
			name: "Front axle and rear right cold",
			temps: models.CornerSet{
				FrontLeft:  65.0, // Cold
				FrontRight: 65.0, // Cold
				RearLeft:   81.0, // Optimal
				RearRight:  65.0, // Cold
			},
			expectedMessage: "Tyres under temp",
			description:     "Should report general under temp when 3 or more individual tyres are cold",
		},
		{
			name: "Rear axle and front left cold",
			temps: models.CornerSet{
				FrontLeft:  65.0, // Cold
				FrontRight: 81.0, // Optimal
				RearLeft:   65.0, // Cold
				RearRight:  65.0, // Cold
			},
			expectedMessage: "Tyres under temp",
			description:     "Should report general under temp when 3 or more individual tyres are cold",
		},
		{
			name: "Front axle and rear right cold",
			temps: models.CornerSet{
				FrontLeft:  65.0, // Cold
				FrontRight: 65.0, // Cold
				RearLeft:   65.0, // Cold
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "Tyres under temp",
			description:     "Should report general under temp when 3 or more individual tyres are cold",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			// Create tyre attributes from the test temperatures
			attributes := tyres.New(
				suite.app.config.GetPitRadioTyreTemperatureOptimalCelsius(),
				suite.app.config.GetPitRadioTyreTemperatureOperatingWindow(),
				suite.app.config.GetPitRadioTyreTemperatureMarginCelsius(),
				testCase.temps,
			)

			// Act
			message := suite.app.generateTyreConditionMessage(attributes)

			// Assert
			suite.Equal(testCase.expectedMessage, message, testCase.description)
		})
	}
}

func (suite *TyreIntegrationTestSuite) TestTyreHotConditionNotificationMessages() { //nolint:dupl // Similar code for hot and cold tests
	// Arrange
	testCases := []struct {
		name            string
		temps           models.CornerSet
		expectedMessage string
		description     string
	}{
		{
			name: "All tyres hot",
			temps: models.CornerSet{
				FrontLeft:  95.0,
				FrontRight: 95.0,
				RearLeft:   95.0,
				RearRight:  95.0,
			},
			expectedMessage: "Tyres over temp",
			description:     "Should report over temp when all tyres are hot",
		},
		{
			name: "Individual front left hot",
			temps: models.CornerSet{
				FrontLeft:  95.0, // Hot
				FrontRight: 81.0, // Optimal
				RearLeft:   81.0, // Optimal
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 1 tyre is hot",
		},
		{
			name: "Individual front right hot",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 95.0, // Hot
				RearLeft:   81.0, // Optimal
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 1 tyre is hot",
		},
		{
			name: "Individual rear left hot",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 81.0, // Optimal
				RearLeft:   95.0, // Hot
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 1 tyre is hot",
		},
		{
			name: "Individual rear right hot",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 81.0, // Optimal
				RearLeft:   81.0, // Optimal
				RearRight:  95.0, // Hot
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 1 tyre is hot",
		},
		{
			name: "Left side hot",
			temps: models.CornerSet{
				FrontLeft:  95.0, // Hot
				FrontRight: 81.0, // Optimal
				RearLeft:   95.0, // Hot
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 2 tyres are hot on different axles",
		},
		{
			name: "Right side hot",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 95.0, // Hot
				RearLeft:   81.0, // Optimal
				RearRight:  95.0, // Hot
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 2 tyres are hot on different axles",
		},
		{
			name: "Left diagonal hot",
			temps: models.CornerSet{
				FrontLeft:  95.0, // Hot
				FrontRight: 81.0, // Optimal
				RearLeft:   81.0, // Optimal
				RearRight:  95.0, // Hot
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 2 tyres are hot on different axles",
		},
		{
			name: "Right diagonal hot",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 95.0, // Hot
				RearLeft:   95.0, // Hot
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "",
			description:     "Should not report specific tyre when only 2 tyres are hot on different axles",
		},
		{
			name: "Front axle hot",
			temps: models.CornerSet{
				FrontLeft:  95.0, // Hot
				FrontRight: 95.0, // Hot
				RearLeft:   81.0, // Optimal
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "Tyres over temp: Front",
			description:     "Should report front-specific message when front axle is hot",
		},
		{
			name: "Rear axle hot",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 81.0, // Optimal
				RearLeft:   95.0, // Hot
				RearRight:  95.0, // Hot
			},
			expectedMessage: "Tyres over temp: Rear",
			description:     "Should report rear-specific message when rear axle is hot",
		},
		{
			name: "Front axle and rear left hot",
			temps: models.CornerSet{
				FrontLeft:  95.0, // Hot
				FrontRight: 95.0, // Hot
				RearLeft:   95.0, // Hot
				RearRight:  81.0, // Optimal
			},
			expectedMessage: "Tyres over temp",
			description:     "Should report general over temp when 3 or more individual tyres are hot",
		},
		{
			name: "Front axle and rear right hot",
			temps: models.CornerSet{
				FrontLeft:  95.0, // Hot
				FrontRight: 95.0, // Hot
				RearLeft:   81.0, // Optimal
				RearRight:  95.0, // Hot
			},
			expectedMessage: "Tyres over temp",
			description:     "Should report general over temp when 3 or more individual tyres are hot",
		},
		{
			name: "Rear axle and front left hot",
			temps: models.CornerSet{
				FrontLeft:  95.0, // Hot
				FrontRight: 81.0, // Optimal
				RearLeft:   95.0, // Hot
				RearRight:  95.0, // Hot
			},
			expectedMessage: "Tyres over temp",
			description:     "Should report general over temp when 3 or more individual tyres are hot",
		},
		{
			name: "Rear axle and front right hot",
			temps: models.CornerSet{
				FrontLeft:  81.0, // Optimal
				FrontRight: 95.0, // Hot
				RearLeft:   95.0, // Hot
				RearRight:  95.0, // Hot
			},
			expectedMessage: "Tyres over temp",
			description:     "Should report general over temp when 3 or more individual tyres are hot",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			// Create tyre attributes from the test temperatures
			attributes := tyres.New(
				suite.app.config.GetPitRadioTyreTemperatureOptimalCelsius(),
				suite.app.config.GetPitRadioTyreTemperatureOperatingWindow(),
				suite.app.config.GetPitRadioTyreTemperatureMarginCelsius(),
				testCase.temps,
			)

			// Act
			message := suite.app.generateTyreConditionMessage(attributes)

			// Assert
			suite.Equal(testCase.expectedMessage, message, testCase.description)
		})
	}
}

// ********************************************************************************************************************
// Test helper functions
// ********************************************************************************************************************

// setupDisabledTyreTemperatureConfig provides configuration with tyre temperature monitoring disabled.
func (suite *TyreIntegrationTestSuite) setupDisabledTyreTemperatureConfig() {
	configJSON := []byte(`{
		"app": {
			"language": "en",
			"accent": "us"
		},
		"pitRadio": {
			"tyreTemperatureMonitoring": false,
			"tyreTemperatureOptimalCelsius": 80,
			"tyreTemperatureOperatingWindow": 10,
			"tyreTemperatureMarginCelsius": 5
		}
	}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())
}
